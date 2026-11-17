package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

// These commands expose the same kernel used by real execution. A successful
// match, preparation plan, or finite-domain proof is model analysis; none is a
// product execution receipt or authorization to mutate a target environment.
func businessContractCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: arc verify contract <check|match|compare|prepare> --file MODEL.json|--graph ID --db DB [--input FILE] [--action ID]")
	}
	name := args[0]
	switch name {
	case "check", "match", "compare", "prepare":
	default:
		return fmt.Errorf("unknown contract command %q", name)
	}
	f := flag.NewFlagSet("contract "+name, flag.ContinueOnError)
	file := f.String("file", "", "versioned business model JSON")
	graph := f.String("graph", "", "model ID in the existing ledger")
	dbfile := f.String("db", ".arc/verify/data.db", "existing project ledger")
	input := f.String("input", "", "observation, comparison or preparation request JSON")
	action := f.String("action", "", "declared business action ID")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if len(f.Args()) != 0 {
		return fmt.Errorf("unexpected contract arguments: %v", f.Args())
	}
	if (*file == "") == (*graph == "") {
		return errors.New("contract command requires exactly one of --file or --graph")
	}
	var g Graph
	var err error
	if *file != "" {
		g, err = loadGraph(*file)
	} else {
		var d db
		d, err = openDB(*dbfile, false)
		if err == nil {
			defer d.close()
			g, err = d.model(*graph)
		}
	}
	if err != nil {
		return err
	}
	if err := ValidateBusinessContract(g); err != nil {
		return err
	}
	read := func(target any) error {
		if *input == "" {
			return errors.New("--input required; declarations are not live observations")
		}
		info, err := os.Stat(*input)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > 8<<20 {
			return errors.New("contract input must be a regular JSON file of at most 8 MiB")
		}
		body, err := os.ReadFile(*input)
		if err != nil {
			return err
		}
		return decodeStrictJSON(body, target)
	}
	switch name {
	case "check":
		proof := ProveBusinessCoverage(g)
		emit(proof)
		if proof.Status != "PROVEN_IN_SCOPE" || !proof.CompleteInScope {
			return fmt.Errorf("business contract %s: %s", proof.Status, proof.Reason)
		}
	case "match":
		if *action == "" {
			return errors.New("contract match requires --action")
		}
		var observation BusinessObservation
		if err := read(&observation); err != nil {
			return err
		}
		decision := FreezeBusinessExpectation(g, *action, observation)
		emit(decision)
		if decision.Frozen == nil {
			return fmt.Errorf("business branch %s: %s", decision.Status, decision.Reason)
		}
	case "compare":
		var request struct {
			Frozen FrozenBusinessExpectation `json:"frozen"`
			Actual BusinessObservation       `json:"actual"`
			Denied *bool                     `json:"denied"`
		}
		if err := read(&request); err != nil {
			return err
		}
		if request.Denied == nil {
			return errors.New("contract compare requires explicitly observed denied boolean")
		}
		comparison := CompareBusinessExpectation(g, request.Frozen, request.Actual, *request.Denied)
		emit(comparison)
		if comparison.Status != "PASS" {
			return fmt.Errorf("business expectation %s: %s", comparison.Status, comparison.Reason)
		}
	case "prepare":
		var request PreparationRequest
		if err := read(&request); err != nil {
			return err
		}
		plan := PlanPreparation(g, request)
		emit(plan)
		if plan.Status != "READY" && plan.Status != "ALREADY_SATISFIED" {
			return fmt.Errorf("business preparation %s: %s", plan.Status, plan.Reason)
		}
	}
	return nil
}
