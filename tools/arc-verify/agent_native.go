package main

import (
    "errors"
)

// AgentNativeProposal comes from the currently active coding agent, not an LLM HTTP API.
type AgentNativeProposal struct {
    Campaign string `json:"campaign"`
    Checkpoint string `json:"checkpoint"`
    HypothesisID string `json:"hypothesis_id,omitempty"`
    ExperimentID string `json:"experiment_id,omitempty"`
    Suggestion PlannerSuggestion `json:"suggestion"`
}

type AgentNativeContext struct {
    Protocol string `json:"protocol"`
    Campaign string `json:"campaign"`
    Graph string `json:"graph"`
    Flow string `json:"flow"`
    SourceCommit string `json:"source_commit"`
    Checkpoint string `json:"checkpoint"`
    Status string `json:"status"`
    StopReason string `json:"stop_reason"`
    UnsafeResume bool `json:"unsafe_resume"`
    Budget ExploreRuntimeBudget `json:"budget"`
    Spent ExploreSpent `json:"spent"`
    World ExplorePersistedWorld `json:"world"`
    Frontier []CandidateAction `json:"frontier"`
    Tried []string `json:"tried"`
    Boundary *DiscoveryContext `json:"boundary_discovery,omitempty"`
    Recent []ExploreStep `json:"recent_steps"`
    Next string `json:"next"`
}

func (d db) agentNativeContext(campaign string) (AgentNativeContext, error) {
    state, err := d.loadExploreCheckpoint(campaign)
    if err != nil { return AgentNativeContext{}, err }
    if state.Mode != "agent-native" { return AgentNativeContext{}, errors.New("campaign is not agent-native") }
    steps := state.Report.Steps
    if len(steps) > 4 { steps = steps[len(steps)-4:] }
    next := "submit proposal using this checkpoint, then execute one action"
    if len(state.Frontier) != 0 { next = "execute queued proposal" }
    if state.UnsafeResume || state.Report.UnsafeResume { next = "BLOCKED: unresolved side-effect; no replay" }
    if state.Status == exploreAborted || state.Status == exploreCompleted { next = "terminal campaign" }
    return AgentNativeContext{
        Protocol:"arc-verify/agent-native/v1", Campaign:state.Campaign, Graph:state.Graph,
        Flow:state.Flow, SourceCommit:state.SourceCommit, Checkpoint:state.Checkpoint,
        Status:state.Status, StopReason:state.Report.StopReason,
        UnsafeResume:state.UnsafeResume || state.Report.UnsafeResume,
        Budget:state.Budget, Spent:state.Spent, World:state.World, Frontier:state.Frontier,
        Tried:state.Tried, Boundary:state.Report.Boundary, Recent:steps, Next:next,
    },nil
}

func (d db) agentNativePropose(campaign string, adapter ExploreAdapter, proposal AgentNativeProposal) (AgentNativeContext,error) {
    state, err := d.loadExploreCheckpoint(campaign)
    if err != nil { return AgentNativeContext{},err }
    if state.Mode != "agent-native" || state.Status != exploreInterrupted ||
        state.UnsafeResume || state.Report.UnsafeResume || state.RecoveryClass == RecoveryFailedLocked {
        return AgentNativeContext{},errors.New("campaign is not in safe waiting state")
    }
    if proposal.Campaign != campaign || proposal.Checkpoint == "" || state.Checkpoint != proposal.Checkpoint {
        return AgentNativeContext{},errors.New("stale agent checkpoint: reload context")
    }
    if len(state.Frontier)!=0 { return AgentNativeContext{},errors.New("execute existing proposal first") }
    if err:=validateExploreAdapter(adapter,state.Graph);err!=nil{return AgentNativeContext{},err}
    if state.Binding.SourceRoot == "" || state.Binding.ConfigPath == "" { return AgentNativeContext{},errors.New("missing source/config binding") }
    current,err:=d.exploreBinding(state.Graph,state.Environment,state.SourceCommit,state.Account,state.Fixture,
        state.Binding.SourceRoot,state.Binding.ConfigPath,adapter.Oracle.ID,"explore/"+campaign)
    if err!=nil{return AgentNativeContext{},err}
    if current.Identity != state.Binding.Identity || current.Oracle != state.Binding.Oracle {return AgentNativeContext{},errors.New("environment/source/oracle changed: STALE")}
    if state.Budget.MaxActions>0 && state.Report.Actions>=state.Budget.MaxActions {return AgentNativeContext{},errors.New("action budget exhausted")}
    item,reason:=validatePlannerSuggestion(proposal.Suggestion,adapter,adapter.Accounts,1)
    if reason!="" {return AgentNativeContext{},errors.New("proposal rejected: "+reason)}
    item.ID=shortDigest(campaign+"|"+proposal.Checkpoint+"|"+item.Action+"|"+item.Role+"|"+item.RequestID)
    item.Source=sourceInfer
    for _,r:=range state.Actions {
        if r.Candidate.ID==item.ID || (r.RequestID!="" && r.RequestID==item.RequestID && r.SideEffecting && r.Status!=actionRecorded) {
            return AgentNativeContext{},errors.New("unresolved or replayed action")}
    }
    lease,err:=d.claimExploreCampaign(campaign,exploreWorkerID())
    if err!=nil{return AgentNativeContext{},err}
    state,err=d.loadExploreCheckpoint(campaign)
    if err!=nil{return AgentNativeContext{},err}
    if state.Checkpoint!=proposal.Checkpoint || len(state.Frontier)!=0 || state.UnsafeResume {return AgentNativeContext{},errors.New("campaign changed while acquiring ownership")}
    proposal.Suggestion.HypothesisID=proposal.HypothesisID
    proposal.Suggestion.ExperimentID=proposal.ExperimentID
    audit:=PlannerProposal{Model:"agent-native",Suggestions:[]PlannerSuggestion{proposal.Suggestion}}
    if err:=d.persistPlannerAudit(campaign,PlannerContext{GraphID:state.Graph},audit,[]CandidateAction{item},nil,state.Report.Actions+1);err!=nil{return AgentNativeContext{},err}
    state.Frontier=[]CandidateAction{item}
    state.Report.Remaining=state.Frontier
    state.Report.StopReason="proposal_ready"
    state.Report.Truncated=true
    state.Checkpoint=newID("CK")
    state.Report.Checkpoint=state.Checkpoint
    state.Status=exploreInterrupted
    if err:=d.saveExploreCheckpointOwned(state,lease);err!=nil{return AgentNativeContext{},err}
    if err:=d.persistExploration(state.Report);err!=nil{return AgentNativeContext{},err}
    return d.agentNativeContext(campaign)
}
