package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Automatic preparation is opt-in and goal-driven. Adapters are keyed by the
// declared preparation ID, not chosen from routes or fabricated by the runner.
type RunnerPreparation struct {
	Goal      BusinessPredicate     `json:"goal"`
	MaxSteps  int                   `json:"max_steps"`
	MaxStates int                   `json:"max_states"`
	Observer  RunnerStep            `json:"observer"`
	Adapters  map[string]RunnerStep `json:"adapters"`
}

func validateRunnerPreparation(g Graph, manifest RunnerManifest, actors map[string]bool, readOnly bool) error {
	p := manifest.Preparation
	if p == nil {
		return nil
	}
	if g.BusinessContract == nil || readOnly || p.MaxSteps < 1 || p.MaxSteps > 128 || p.MaxStates < 1 || p.MaxStates > 10000 {
		return fmt.Errorf("automatic preparation requires a business contract, mutating scope and bounded step/state budgets")
	}
	count := 0
	if err := validateBusinessPredicate(p.Goal, businessAttributeIndex(g.BusinessContract), 0, &count); err != nil {
		return fmt.Errorf("invalid preparation goal: %w", err)
	}
	if !actors[p.Observer.Actor] || len(p.Observer.Oracles) < 1 || len(p.Observer.Oracles) > 8 {
		return fmt.Errorf("preparation observer requires a declared actor and independent oracles")
	}
	if len(p.Observer.InputMeasurements) > 0 {
		return fmt.Errorf("preparation observations cannot claim target request input measurements")
	}
	if err := validateRunnerBusinessBindings(g, p.Observer, actors); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, oracle := range p.Observer.Oracles {
		if seen[oracle.ID] {
			return fmt.Errorf("duplicate preparation observer oracle %s", oracle.ID)
		}
		seen[oracle.ID] = true
		if err := validateOracle(oracle); err != nil {
			return err
		}
	}
	declared := map[string]PreparationAction{}
	for _, action := range g.BusinessContract.Preparation {
		declared[action.ID] = action
	}
	for id, adapter := range p.Adapters {
		if len(adapter.InputMeasurements) > 0 {
			return fmt.Errorf("preparation adapter %s does not support target input_measurements; use declared independent preparation conditions", id)
		}
		action, exists := declared[id]
		if !exists || adapter.Node != action.Action || !actors[adapter.Actor] || adapter.ParallelGroup != "" || adapter.AsyncWait != nil {
			return fmt.Errorf("preparation adapter %s must bind its declared action and actor; preparations run serially with independent readback", id)
		}
		if err := validateAdapter(adapter.Call, false); err != nil {
			return err
		}
		if err := validateResponseMeasurements(adapter); err != nil {
			return err
		}
		if err := validateRunnerBusinessBindings(g, adapter, actors); err != nil {
			return err
		}
		for _, oracle := range p.Observer.Oracles {
			if oracle.Call.Kind == adapter.Call.Kind && oracle.Call.Path == adapter.Call.Path && equalArgs(oracle.Call.Args, adapter.Call.Args) {
				return fmt.Errorf("preparation action response cannot serve as the independent observer")
			}
		}
		oracleIDs := map[string]bool{}
		for _, oracle := range adapter.Oracles {
			if oracleIDs[oracle.ID] {
				return fmt.Errorf("duplicate preparation adapter oracle %s", oracle.ID)
			}
			oracleIDs[oracle.ID] = true
			if err := validateOracle(oracle); err != nil {
				return err
			}
			if oracle.Call.Kind == adapter.Call.Kind && oracle.Call.Path == adapter.Call.Path && equalArgs(oracle.Call.Args, adapter.Call.Args) {
				return fmt.Errorf("preparation action response cannot serve as its independent oracle")
			}
		}
		for _, extract := range adapter.Call.Extract {
			if extract.Source == "oracle" && !oracleIDs[extract.OracleID] {
				return fmt.Errorf("preparation extraction %s lacks its independent oracle", extract.Name)
			}
		}
	}
	return nil
}

func runRuntimePreparation(ctx context.Context, g Graph, flow string, manifest RunnerManifest, binding RunBinding, accounts map[string]Fixture, actors map[string]ParticipantBinding, sess *execSession, setup *ModelObservation) PreparationExecution {
	p := manifest.Preparation
	observe := func(ctx context.Context) (BusinessObservation, error) {
		observed := ModelObservation{EvidenceVersion: runtimeEvidenceVersion, Participants: setup.Participants, BeforeEntities: sess.entitySlice(), BeforeSessions: sess.sessionSlice()}
		for _, oracle := range p.Observer.Oracles {
			single, cancel := context.WithTimeout(ctx, time.Duration(manifest.StepTimeoutMS)*time.Millisecond)
			facts, hash, _, err := readOracleWithSession(single, oracle, binding, accounts[p.Observer.Actor], p.Observer.Actor, manifest.IsolationScope, sess)
			cancel()
			if err != nil {
				return BusinessObservation{}, err
			}
			observed.Oracles = append(observed.Oracles, OracleObservation{ActionIndex: 1, Actor: p.Observer.Actor, ID: oracle.ID, Boundary: oracle.Boundary, Before: facts, BeforeSHA256: hash})
		}
		return runtimeBusinessObservation(flow, p.Observer.Oracles[0].ID, p.Observer, 1, true, &observed)
	}
	act := func(ctx context.Context, planned PlannedPreparationStep) (PreparationReceipt, error) {
		adapter, found := p.Adapters[planned.ID]
		if !found || adapter.Node != planned.Action {
			return PreparationReceipt{}, fmt.Errorf("no registered preparation adapter for %s", planned.ID)
		}
		single, cancel := context.WithTimeout(ctx, time.Duration(manifest.StepTimeoutMS)*time.Millisecond)
		output, err := performCallWithSession(single, adapter.Call, binding, accounts[adapter.Actor], adapter.Actor, sess)
		cancel()
		action := actionRecord(adapter, actors[adapter.Actor], output)
		action.ActionIndex = len(setup.Actions) + 1
		if err == nil {
			action.ResponseValues, err = captureResponseMeasurements(adapter, output)
		}
		setup.Actions = append(setup.Actions, action)
		receipt := PreparationReceipt{Action: action}
		if err != nil || output.Denied {
			if err == nil {
				err = fmt.Errorf("preparation action was denied")
			}
			return receipt, err
		}
		oracleDocs := map[oracleIdentity]json.RawMessage{}
		for _, oracle := range adapter.Oracles {
			single, cancel := context.WithTimeout(ctx, time.Duration(manifest.StepTimeoutMS)*time.Millisecond)
			_, _, doc, readErr := readOracleWithSession(single, oracle, binding, accounts[adapter.Actor], adapter.Actor, manifest.IsolationScope, sess)
			cancel()
			if readErr != nil {
				return receipt, readErr
			}
			oracleDocs[oracleIdentity{ActionIndex: action.ActionIndex, Actor: adapter.Actor, ID: oracle.ID}] = doc
		}
		extracted, err := applyExtractions(adapter.Call.Extract, -1, action.ActionIndex, adapter.Actor, &output, oracleDocs, sess)
		setup.Extractions = append(setup.Extractions, extracted...)
		return receipt, err
	}
	return ExecutePreparation(ctx, g, PreparationRequest{Goal: p.Goal, MaxSteps: p.MaxSteps, MaxStates: p.MaxStates}, observe, act)
}
