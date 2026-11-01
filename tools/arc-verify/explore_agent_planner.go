package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

var errPlannerWait = errors.New("agent planner is waiting for a matching proposal")

const (
	agentContextFile  = "context.json"
	agentProposalFile = "proposal.json"
	agentPlannerInstr = "Write proposal.json with for_call and context_sha256 copied from this envelope. Suggest only authorized adapter entries. Do not decide PASS or FAIL. Do not call the target, database or oracle yourself."
)

type AgentPlannerPending struct {
	Call       int    `json:"call"`
	ContextSHA string `json:"context_sha256"`
	Dir        string `json:"dir,omitempty"`
	Context    string `json:"context_path,omitempty"`
}

type AgentPlannerEnvelope struct {
	Campaign      string         `json:"campaign"`
	Call          int            `json:"call"`
	ContextSHA256 string         `json:"context_sha256"`
	Instructions  string         `json:"instructions"`
	Context       PlannerContext `json:"context"`
}

type AgentExplorePlanner struct {
	Dir     string
	asked   int
	pending AgentPlannerPending
}

func NewAgentExplorePlanner(dir string) (*AgentExplorePlanner, error) {
	if dir == "" {
		return nil, errors.New("agent planner needs a handshake directory")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("agent planner --planner must be a directory")
	}
	return &AgentExplorePlanner{Dir: dir}, nil
}

func (p *AgentExplorePlanner) Pending() AgentPlannerPending {
	if p == nil {
		return AgentPlannerPending{}
	}
	return p.pending
}

func (p *AgentExplorePlanner) Suggest(ctx context.Context, in PlannerContext) (PlannerProposal, error) {
	if p == nil {
		return PlannerProposal{}, errors.New("agent planner is missing")
	}
	if err := ctx.Err(); err != nil {
		return PlannerProposal{}, err
	}
	if p.Dir == "" {
		return PlannerProposal{}, errors.New("agent planner needs a handshake directory")
	}
	p.asked++
	if p.asked == 1 {
		env, err := loadAgentPlannerEnvelope(p.Dir)
		if err == nil {
			proposal, err := readAgentPlannerProposal(p.Dir, env)
			if err == nil {
				if err := consumeAgentPlannerProposal(p.Dir, env.Call); err != nil {
					return PlannerProposal{}, err
				}
				p.pending = agentPending(p.Dir, env)
				proposal.Model = firstNonEmpty(proposal.Model, "agent")
				if proposal.Source == "" {
					proposal.Source = "agent"
				}
				return proposal, nil
			}
			if err != nil && !errors.Is(err, errPlannerWait) && !errors.Is(err, os.ErrNotExist) {
				return PlannerProposal{}, err
			}
			p.pending = agentPending(p.Dir, env)
			return PlannerProposal{}, errPlannerWait
		}
		if !errors.Is(err, os.ErrNotExist) {
			return PlannerProposal{}, err
		}
	}
	env, err := writeAgentPlannerContext(p.Dir, in)
	if err != nil {
		return PlannerProposal{}, err
	}
	p.pending = agentPending(p.Dir, env)
	return PlannerProposal{}, errPlannerWait
}

func agentPending(dir string, env AgentPlannerEnvelope) AgentPlannerPending {
	return AgentPlannerPending{Call: env.Call, ContextSHA: env.ContextSHA256, Dir: dir, Context: filepath.Join(dir, agentContextFile)}
}

func writeAgentPlannerContext(dir string, in PlannerContext) (AgentPlannerEnvelope, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return AgentPlannerEnvelope{}, err
	}
	sha := digestBytes(body)
	call := in.Call
	if call < 1 {
		call = 1
	}
	env := AgentPlannerEnvelope{
		Campaign: in.Campaign, Call: call, ContextSHA256: sha,
		Instructions: agentPlannerInstr, Context: in,
	}
	raw, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return env, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return env, err
	}
	current := filepath.Join(dir, agentContextFile)
	history := filepath.Join(dir, fmt.Sprintf("context-%d.json", call))
	if err := os.WriteFile(history, raw, 0600); err != nil {
		return env, err
	}
	if err := os.WriteFile(current, raw, 0600); err != nil {
		return env, err
	}
	return env, nil
}

func readAgentPlannerProposal(dir string, env AgentPlannerEnvelope) (PlannerProposal, error) {
	path := filepath.Join(dir, agentProposalFile)
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return PlannerProposal{}, errPlannerWait
		}
		return PlannerProposal{}, err
	}
	var proposal PlannerProposal
	if err := decodeStrictJSON(body, &proposal); err != nil {
		return PlannerProposal{}, err
	}
	if proposal.ForCall != env.Call || proposal.ContextSHA256 != env.ContextSHA256 {
		return PlannerProposal{}, errPlannerWait
	}
	if len(proposal.Suggestions) > 8 {
		proposal.Suggestions = proposal.Suggestions[:8]
	}
	return proposal, nil
}

func consumeAgentPlannerProposal(dir string, call int) error {
	src := filepath.Join(dir, agentProposalFile)
	dst := filepath.Join(dir, "proposal-used-"+strconv.Itoa(call)+".json")
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("cannot consume agent proposal: %w", err)
	}
	return nil
}

func writeAgentPlannerProposal(dir string, proposal PlannerProposal) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if proposal.Model == "" {
		proposal.Model = "agent"
	}
	if proposal.Source == "" {
		proposal.Source = "agent"
	}
	body, err := json.MarshalIndent(proposal, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, agentProposalFile), body, 0600)
}

func loadAgentPlannerEnvelope(dir string) (AgentPlannerEnvelope, error) {
	var env AgentPlannerEnvelope
	body, err := os.ReadFile(filepath.Join(dir, agentContextFile))
	if err != nil {
		return env, err
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return env, err
	}
	return env, nil
}

func bindAgentProposalToEnvelope(proposal PlannerProposal, env AgentPlannerEnvelope) PlannerProposal {
	proposal.ForCall = env.Call
	proposal.ContextSHA256 = env.ContextSHA256
	proposal.Model = firstNonEmpty(proposal.Model, "agent")
	proposal.Source = firstNonEmpty(proposal.Source, "agent")
	if len(proposal.Suggestions) > 1 {
		proposal.Suggestions = proposal.Suggestions[:1]
	}
	return proposal
}
