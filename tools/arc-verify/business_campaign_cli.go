package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
)

type businessCampaignStop struct {
	status string
	reason string
	code   int
}

func (s businessCampaignStop) Error() string {
	return "business campaign stopped: " + s.status + ": " + s.reason
}

func businessCampaignExecutionError(campaign BusinessCampaign) error {
	if campaign.Status == "COMPLETED_IN_SCOPE" && campaign.EvidenceStatus == "PASS" && campaign.Coverage.SelectedVariantsCovered {
		return nil
	}
	code := chainExitBlocked
	switch campaign.Status {
	case "DIVERGED":
		code = chainExitDivergence
	case "SPECIFICATION_GAP", "MODEL_ERROR":
		code = chainExitNewBranch
	case "BUDGET":
		code = chainExitBudget
	}
	return businessCampaignStop{status: campaign.Status, reason: campaign.Reason, code: code}
}

func readBusinessCampaignRequest(path string) (BusinessCampaignRequest, error) {
	var request BusinessCampaignRequest
	info, err := os.Stat(path)
	if err != nil {
		return request, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return request, errors.New("campaign request must be a regular JSON file of at most 1 MiB")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return request, err
	}
	err = decodeStrictJSON(body, &request)
	return request, err
}

func businessCampaignCommand(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: arc verify campaign <run|resume|status|list|abort> [options]")
	}
	name := args[0]
	if name != "run" && name != "resume" && name != "status" && name != "list" && name != "abort" {
		return fmt.Errorf("unknown campaign command %q", name)
	}
	f := flag.NewFlagSet("campaign "+name, flag.ContinueOnError)
	dbfile := f.String("db", ".arc/verify/data.db", "existing project ledger")
	var file, id, graph, environment, round, reason string
	if name == "run" || name == "resume" {
		f.StringVar(&file, "file", "", "bounded campaign request JSON; required for run")
	}
	if name == "resume" || name == "status" || name == "abort" {
		f.StringVar(&id, "id", "", "existing business campaign ID")
	}
	if name == "list" {
		f.StringVar(&graph, "graph", "", "required model ID")
		f.StringVar(&environment, "env", "", "environment filter")
		f.StringVar(&round, "round", "", "verification round filter")
	}
	if name == "abort" {
		f.StringVar(&reason, "reason", "", "required reason; preserves dispatches and run history")
	}
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if len(f.Args()) != 0 {
		return fmt.Errorf("unexpected campaign arguments: %v", f.Args())
	}
	if name == "run" && file == "" {
		return errors.New("campaign run requires --file with explicit registered variants and budgets")
	}
	if (name == "resume" || name == "status" || name == "abort") && id == "" {
		return errors.New("campaign " + name + " requires --id")
	}
	if name == "list" && graph == "" {
		return errors.New("campaign list requires --graph")
	}
	if name == "abort" && reason == "" {
		return errors.New("campaign abort requires --reason")
	}
	var request BusinessCampaignRequest
	if file != "" {
		var err error
		request, err = readBusinessCampaignRequest(file)
		if err != nil {
			return err
		}
	}
	if name == "resume" {
		if request.Campaign != "" && request.Campaign != id {
			return errors.New("campaign request identity differs from --id")
		}
		request.Campaign = id
	}
	d, err := openDB(*dbfile, false)
	if err != nil {
		return err
	}
	defer d.close()
	available, err := d.tableExists("business_campaigns")
	if err != nil {
		return err
	}
	if !available {
		return errors.New("business campaign schema is missing; run arc verify init on this ledger to add it without rewriting history")
	}
	switch name {
	case "status":
		campaign, err := d.loadBusinessCampaign(id)
		if err == nil {
			emit(campaign)
		}
		return err
	case "list":
		campaigns, available, err := d.snapshotBusinessCampaigns(graph, environment, round)
		if err == nil {
			emit(map[string]any{"graph": graph, "environment": environment, "round": round,
				"campaigns": campaigns, "available": available, "truncated": len(campaigns) < available})
		}
		return err
	case "abort":
		campaign, err := d.abortBusinessCampaign(id, reason)
		if campaign.ID != "" {
			emit(campaign)
		}
		if err != nil {
			return err
		}
		if campaign.Status != "ABORTED" {
			return businessCampaignStop{status: campaign.Status, reason: campaign.Reason, code: chainExitBlocked}
		}
		return nil
	default:
		var campaign BusinessCampaign
		if name == "run" {
			campaign, err = d.runBusinessCampaign(ctx, request)
		} else {
			campaign, err = d.resumeBusinessCampaign(ctx, request)
		}
		if campaign.ID != "" {
			emit(campaign)
		}
		if err != nil {
			return err
		}
		return businessCampaignExecutionError(campaign)
	}
}
