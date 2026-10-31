package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Exit codes name why run-chain stopped; 0 means the known path completed.
const (
	chainExitDivergence   = 10
	chainExitUnknownState = 11
	chainExitPrecondition = 12
	chainExitNewBranch    = 13
	chainExitBudget       = 14
	chainExitBlocked      = 15
)

// ChainRequest bounds one deterministic run over the known path.
type ChainRequest struct {
	Graph, Environment, Round, Commit, Root, Config, Account, Fixture string
	Accounts                                                          map[string]string
	MaxSteps                                                          int
	Deadline                                                          time.Duration
}

// ChainStep is one executed card.
type ChainStep struct {
	Task    string `json:"task"`
	Runner  string `json:"runner"`
	Run     string `json:"run,omitempty"`
	Verdict string `json:"verdict"`
	Reason  string `json:"reason,omitempty"`
}

// ChainPacket is the minimal context handed back when the chain stops.
type ChainPacket struct {
	Graph       string      `json:"graph"`
	Round       string      `json:"round"`
	Stop        string      `json:"stop"`
	Reason      string      `json:"reason"`
	Steps       []ChainStep `json:"steps"`
	Task        string      `json:"task,omitempty"`
	Run         string      `json:"run,omitempty"`
	State       string      `json:"state,omitempty"`
	Divergence  *StepTrace  `json:"divergence,omitempty"`
	Candidates  []string    `json:"candidates"`
	Remaining   []string    `json:"remaining"`
	Unautomated []string    `json:"unautomated"`
}

type chainStop struct {
	packet ChainPacket
	code   int
}

func (s chainStop) Error() string {
	return "run-chain stopped: " + s.packet.Stop + ": " + s.packet.Reason
}

// runChain executes the known path card by card through the same execute
// entry point, so every card keeps its RAW, independent verdict, locks and
// recovery. It stops at the first card that needs judgment and returns a
// chainStop carrying the minimal context packet.
func (d db) runChain(ctx context.Context, request ChainRequest) (ChainPacket, error) {
	packet := ChainPacket{Graph: request.Graph, Round: request.Round, Steps: []ChainStep{}, Candidates: []string{}, Remaining: []string{}, Unautomated: []string{}}
	if request.Graph == "" || request.Round == "" {
		return packet, errors.New("run-chain needs --graph and --round")
	}
	if request.MaxSteps < 1 || request.MaxSteps > 1024 || request.Deadline < 100*time.Millisecond || request.Deadline > 24*time.Hour {
		return packet, errors.New("run-chain needs --max-steps 1..1024 and --deadline 100ms..24h")
	}
	g, err := d.model(request.Graph)
	if err != nil {
		return packet, err
	}
	if g.SchemaVersion != 2 {
		return packet, errors.New("run-chain requires a v2 business model")
	}
	round, err := d.verificationRound(request.Round)
	if err != nil {
		return packet, err
	}
	if round.Context.Graph != g.ID || request.Environment != "" && round.Context.Environment != request.Environment {
		return packet, errors.New("round belongs to another graph or environment")
	}
	path, unautomated, err := d.knownPath(g)
	if err != nil {
		return packet, err
	}
	packet.Unautomated = unautomated
	if len(path) == 0 && len(unautomated) > 0 {
		packet.Stop, packet.Reason = "blocked", fmt.Sprintf("none of %d READY cards has exactly one current registered runner (missing, stale or ambiguous)", len(unautomated))
		return packet, chainStop{packet: packet, code: chainExitBlocked}
	}
	chainCtx, cancel := context.WithTimeout(ctx, request.Deadline)
	defer cancel()
	position := 0
	stop := func(kind string, code int, reason string) (ChainPacket, error) {
		packet.Stop, packet.Reason = kind, reason
		for _, card := range path[position:] {
			if card.task != packet.Task || len(packet.Steps) == 0 || packet.Steps[len(packet.Steps)-1].Task != card.task {
				packet.Remaining = append(packet.Remaining, card.task)
			}
		}
		return packet, chainStop{packet: packet, code: code}
	}
	for index, card := range path {
		position = index
		if len(packet.Steps) >= request.MaxSteps {
			return stop("budget", chainExitBudget, fmt.Sprintf("step budget %d reached before %s", request.MaxSteps, card.task))
		}
		if chainCtx.Err() != nil {
			return stop("budget", chainExitBudget, "deadline reached before "+card.task)
		}
		packet.Task = card.task
		result, err := d.execute(chainCtx, ExecuteRequest{Task: card.task, Runner: card.runner, Round: request.Round, Environment: round.Context.Environment, Commit: request.Commit, Root: request.Root, Config: request.Config, Account: request.Account, Fixture: request.Fixture, Accounts: request.Accounts})
		step := ChainStep{Task: card.task, Runner: card.runner, Run: result.Run, Verdict: result.Verdict, Reason: result.Reason}
		if err != nil {
			step.Verdict, step.Reason = "BLOCKED", err.Error()
		}
		packet.Steps = append(packet.Steps, step)
		packet.Run = result.Run
		if err != nil {
			if strings.Contains(err.Error(), "dependency") {
				return stop("precondition", chainExitPrecondition, err.Error())
			}
			return stop("blocked", chainExitBlocked, err.Error())
		}
		if result.Verdict == "PASS" {
			continue
		}
		trace, err := d.trace(result.Run)
		if err != nil {
			return packet, err
		}
		packet.State = observedState(trace)
		packet.Candidates = nextTransitions(g, packet.State)
		if divergence, found := firstDivergence(trace.Steps); found {
			divergence.Observation = nil
			packet.Divergence = &divergence
		}
		switch {
		case chainCtx.Err() != nil:
			return stop("budget", chainExitBudget, "deadline reached during "+card.task+": "+result.Reason)
		case result.Verdict == "BLOCKED" && strings.Contains(result.Reason, "preconditions"):
			return stop("precondition", chainExitPrecondition, result.Reason)
		case result.Verdict == "FAIL" && packet.State != "" && packet.State != "UNCHANGED" && packet.State != "UNKNOWN" && !modelState(g, packet.State):
			return stop("unknown_state", chainExitUnknownState, "observed state "+packet.State+" is not in the model")
		case result.Verdict == "FAIL" && trace.ContinueDecision == "REPLAN":
			return stop("new_branch", chainExitNewBranch, "a known but unexpected state was observed safely: "+packet.State)
		case result.Verdict == "FAIL":
			return stop("divergence", chainExitDivergence, result.Reason)
		default:
			return stop("blocked", chainExitBlocked, result.Reason)
		}
	}
	packet.Task, packet.Stop, packet.Reason = "", "completed", fmt.Sprintf("all %d cards on the known path passed", len(packet.Steps))
	return packet, nil
}

type chainCard struct{ task, runner string }

// knownPath orders READY model tasks that have exactly one registered runner
// by their declared test dependencies; the rest need a runner first.
func (d db) knownPath(g Graph) ([]chainCard, []string, error) {
	rows, err := d.query("SELECT id FROM tasks WHERE graph="+quote(g.ID)+" AND kind='MODEL' AND status='READY' ORDER BY id LIMIT 100001", 1)
	if err != nil {
		return nil, nil, err
	}
	runnerRows, err := d.query("SELECT id,test FROM registered_runners WHERE graph="+quote(g.ID)+" ORDER BY id", 2)
	if err != nil {
		return nil, nil, err
	}
	runners := map[string][]string{}
	for _, row := range runnerRows {
		if _, err := d.registeredRunner(row[0]); err == nil {
			runners[g.ID+"/"+row[1]] = append(runners[g.ID+"/"+row[1]], row[0])
		}
	}
	obligations := map[string]ModelObligation{}
	for _, obligation := range modelObligations(g) {
		obligations[obligation.Task.ID] = obligation
	}
	ready, unautomated := map[string]string{}, []string{}
	for _, row := range rows {
		if candidates := runners[row[0]]; len(candidates) == 1 {
			ready[row[0]] = candidates[0]
		} else {
			unautomated = append(unautomated, row[0])
		}
	}
	ordered, placed := []chainCard{}, map[string]bool{}
	ids := make([]string, 0, len(ready))
	for id := range ready {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for len(ordered) < len(ids) {
		progressed := false
		for _, id := range ids {
			if placed[id] {
				continue
			}
			waiting := false
			for _, dependency := range obligations[id].Definition.DependsOn {
				if _, inPath := ready[g.ID+"/"+dependency]; inPath && !placed[g.ID+"/"+dependency] {
					waiting = true
				}
			}
			if !waiting {
				ordered, placed[id], progressed = append(ordered, chainCard{task: id, runner: ready[id]}), true, true
			}
		}
		if !progressed {
			return nil, nil, errors.New("test dependencies among runnable cards are cyclic")
		}
	}
	return ordered, unautomated, nil
}

// observedState is the last independently observed business state; when the
// action never ran, it is the oracle's reading taken before the action.
func observedState(trace TraceReport) string {
	if trace.LatestObserved != "" && trace.LatestObserved != "UNKNOWN" {
		return trace.LatestObserved
	}
	for index := len(trace.Steps) - 1; index >= 0; index-- {
		if observation := trace.Steps[index].Observation; observation != nil {
			for _, oracle := range observation.Oracles {
				if oracle.After.State != "" {
					return oracle.After.State
				}
				if oracle.Before.State != "" {
					return oracle.Before.State
				}
			}
		}
	}
	return trace.LatestObserved
}

func modelState(g Graph, id string) bool {
	for _, node := range g.Nodes {
		if node.ID == id && node.Kind == "state" {
			return true
		}
	}
	return false
}

// nextTransitions lists the declared transitions leaving an observed state.
func nextTransitions(g Graph, state string) []string {
	result := []string{}
	for _, edge := range g.Edges {
		if edge.Kind == "transitions_to" && edge.From == state {
			result = append(result, edge.ID)
		}
	}
	sort.Strings(result)
	return result
}
