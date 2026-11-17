package main

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"
)

// A campaign flow is an execution scope, not an edit to the registered task.
// Only the event written atomically by campaign start may attach that scope to
// a child. Ordinary and historical runs retain their original task flow.
func (d db) businessCampaignRunFlow(run string, binding RunBinding, record ModelRunRecord) (string, error) {
	events, err := d.query("SELECT task,reason FROM task_events WHERE run="+quote(run)+" AND event='CAMPAIGN_ATTEMPT'", 2)
	if err != nil {
		return "", err
	}
	if len(events) == 0 {
		return "", nil
	}
	if len(events) != 1 {
		return "", errors.New("duplicate campaign child start provenance")
	}
	rows, err := d.query("SELECT campaign_id,seq,variant_id,task,intent_sha256,body_json FROM business_campaign_dispatches WHERE run_id="+quote(run), 6)
	if err != nil {
		return "", err
	}
	if len(rows) != 1 {
		return "", errors.New("campaign child start has no unique durable dispatch")
	}
	row := rows[0]
	var intent businessCampaignDispatch
	if err := decodeStrictJSON([]byte(row[5]), &intent); err != nil {
		return "", err
	}
	if digestBytes([]byte(row[5])) != row[4] || intent.Run != run || intent.Campaign != row[0] || strconv.Itoa(intent.Sequence) != row[1] || intent.VariantID != row[2] || intent.Task != row[3] || intent.Binding != binding || record.Round != intent.Round || record.Runner != intent.Runner || record.RunnerSHA256 != intent.RunnerSHA256 || record.CodeSHA256 != intent.CodeSHA256 || !reflect.DeepEqual(record.Participants, intent.Participants) || events[0][0] != intent.Task || events[0][1] != "new immutable attempt for "+intent.Campaign+"/"+intent.VariantID {
		return "", errors.New("campaign child start provenance differs from its intent")
	}
	c, err := d.readBusinessCampaign(intent.Campaign)
	if err != nil {
		return "", err
	}
	if c.Flow != intent.Flow || c.Round != record.Round || c.Graph != binding.Identity.Graph {
		return "", errors.New("campaign child flow differs from its frozen campaign")
	}
	return intent.Flow, nil
}

type businessCampaignDispatchRow struct {
	Intent businessCampaignDispatch
	Hash   string
	Status string
}

func (d db) verifyBusinessCampaignDispatchBirth(intent businessCampaignDispatch, hash string) error {
	rows, err := d.query(fmt.Sprintf("SELECT kind,body_json FROM business_campaign_events WHERE campaign_id=%s AND seq=%d", quote(intent.Campaign), intent.LedgerRevision), 2)
	if err != nil {
		return err
	}
	if len(rows) != 1 || rows[0][0] != "DISPATCH_INTENT" {
		return errors.New("dispatch has no original pre-action campaign event")
	}
	var captured BusinessCampaign
	if err := decodeStrictJSON([]byte(rows[0][1]), &captured); err != nil {
		return err
	}
	if captured.ID != intent.Campaign || captured.Revision != intent.LedgerRevision || len(captured.Dispatches) != intent.Sequence || captured.Spent.Dispatches != intent.Sequence || captured.Dispatches[intent.Sequence-1] != (BusinessCampaignDispatchReference{Sequence: intent.Sequence, Run: intent.Run, SHA256: hash}) || captured.HistorySHA256 != intent.HistorySHA256 {
		return errors.New("dispatch intent differs from the original durable pre-action event")
	}
	for _, candidate := range captured.Frontier {
		if candidate.Selected && equalBusinessCampaignJSON(candidate, intent.Candidate) {
			return nil
		}
	}
	return errors.New("dispatch selection was not captured in its pre-action event")
}

func (d db) businessCampaignDispatches(c BusinessCampaign) ([]businessCampaignDispatchRow, error) {
	rows, err := d.query("SELECT seq,run_id,variant_id,runner_id,task,status,intent_sha256,body_json FROM business_campaign_dispatches WHERE campaign_id="+quote(c.ID)+" ORDER BY seq LIMIT 1025", 8)
	if err != nil {
		return nil, err
	}
	if len(rows) > 1024 || len(rows) != c.Spent.Dispatches || len(rows) != len(c.Dispatches) {
		return nil, errors.New("campaign dispatch count differs from its bounded ledger")
	}
	variants := map[string]BusinessCampaignVariant{}
	for _, variant := range c.Variants {
		variants[variant.ID] = variant
	}
	out := make([]businessCampaignDispatchRow, 0, len(rows))
	for index, row := range rows {
		var intent businessCampaignDispatch
		if len(row[7]) > businessCampaignBodyLimit || decodeStrictJSON([]byte(row[7]), &intent) != nil || digestBytes([]byte(row[7])) != row[6] {
			return nil, errors.New("campaign dispatch intent is damaged")
		}
		if c.Dispatches[index] != (BusinessCampaignDispatchReference{Sequence: index + 1, Run: intent.Run, SHA256: row[6]}) {
			return nil, errors.New("dispatch reference differs from its immutable intent")
		}
		if err := d.verifyBusinessCampaignDispatchBirth(intent, row[6]); err != nil {
			return nil, err
		}
		variant, exists := variants[intent.VariantID]
		if row[0] != strconv.Itoa(index+1) || intent.Sequence != index+1 || intent.Campaign != c.ID || intent.Run != row[1] || intent.VariantID != row[2] || intent.Runner != row[3] || intent.Task != row[4] || intent.Round != c.Round || intent.Flow != c.Flow || !exists || variant.Branch != intent.Branch || variant.Runner != intent.Runner || variant.Task != intent.Task || variant.RunnerSHA256 != intent.RunnerSHA256 || variant.CodeSHA256 != intent.CodeSHA256 || intent.Candidate.Run != intent.Run || intent.Candidate.VariantID != intent.VariantID || !intent.Candidate.Selected {
			return nil, errors.New("campaign dispatch identity differs from its frozen scope or variant")
		}
		identity := intent.Binding.Identity
		if identity.Graph != c.Graph || identity.Environment != c.Environment || identity.SourceCommit != c.Identity.SourceCommit || identity.SourceSHA256 != c.Identity.SourceSHA256 || identity.ConfigSHA256 != c.Identity.ConfigSHA256 || identity.GraphSHA256 != c.Identity.GraphSHA256 || identity.Account != c.Identity.Account || identity.AccountSHA256 != c.Identity.AccountSHA256 || identity.Principal != c.Identity.Principal || identity.Fixture != c.Identity.Fixture || identity.FixtureSHA256 != c.Identity.FixtureSHA256 || identity.Target != c.Identity.Target || identity.Task != intent.Task {
			return nil, errors.New("campaign child intent crosses source, account, environment or fixture scope")
		}
		if row[5] != "INTENT" && row[5] != "STARTED" && row[5] != "RECORDED" {
			return nil, errors.New("unknown campaign dispatch status")
		}
		runner, err := d.registeredRunner(intent.Runner)
		if err != nil {
			return nil, err
		}
		upper := 1
		if runner.Manifest.Preparation != nil {
			upper += runner.Manifest.Preparation.MaxSteps
		}
		if runner.Manifest.Recovery != nil {
			upper += len(runner.Manifest.Recovery.Steps)
		}
		if intent.ReservedActions != upper {
			return nil, errors.New("dispatch reservation does not cover all registered target/preparation/recovery attempts")
		}
		out = append(out, businessCampaignDispatchRow{Intent: intent, Hash: row[6], Status: row[5]})
	}
	return out, nil
}

func (d db) campaignChildVisit(c BusinessCampaign, intent businessCampaignDispatch, hash string) (BusinessCampaignVisit, BusinessCampaignSpent, error) {
	visit := BusinessCampaignVisit{Sequence: intent.Sequence, VariantID: intent.VariantID, Branch: intent.Branch, Runner: intent.Runner, Run: intent.Run, DispatchSHA256: hash, BeforeStateKey: intent.Candidate.StateKey}
	spent := BusinessCampaignSpent{}
	contract, err := d.runTaskContract(intent.Run)
	if err != nil {
		return visit, spent, err
	}
	if contract.Model == nil || contract.Receipt == nil || !contract.Receipt.Completed || contract.Status == "RUNNING" {
		return visit, spent, errors.New("child has no terminal, completed registered execution receipt")
	}
	receipt := contract.Receipt
	flow, flowErr := d.businessCampaignRunFlow(intent.Run, contract.Binding, *receipt)
	if flowErr != nil || flow != c.Flow {
		return visit, spent, errors.New("child has no matching atomic campaign start provenance")
	}
	if contract.Binding != intent.Binding || receipt.Round != intent.Round || receipt.Runner != intent.Runner || receipt.RunnerSHA256 != intent.RunnerSHA256 || receipt.CodeSHA256 != intent.CodeSHA256 || !reflect.DeepEqual(receipt.Participants, intent.Participants) {
		return visit, spent, errors.New("child execution identity, source, round, participants or runner differs from dispatch")
	}
	verified, err := d.verifiedRunFacts(intent.Run)
	if err != nil || verified.EvidenceStatus != "PASS" || !verified.OfflineComplete {
		if err == nil {
			err = errors.New("child has incomplete or legacy facts")
		}
		return visit, spent, err
	}
	runner, err := d.registeredRunner(intent.Runner)
	if err != nil {
		return visit, spent, err
	}
	if len(runner.Manifest.Steps) != 1 {
		return visit, spent, errors.New("campaign child no longer has exactly one target")
	}
	step := runner.Manifest.Steps[0]
	visit.Status = contract.Status
	visit.Reason = "child terminal status and independent facts verified"
	visit.Actor = step.Actor
	visit.Recovery = &receipt.Recovery
	setupFound, recoveryFound, targetFound := false, false, false
	var setupEntities []EntityBinding
	for _, fact := range verified.Facts {
		if fact.SourceKind == "setup" {
			setupEntities = fact.Entities
		}
	}
	for _, fact := range verified.Facts {
		if fact.Run != intent.Run || fact.Graph != c.Graph || fact.Task != intent.Task {
			return visit, spent, errors.New("campaign child fact scope differs")
		}
		switch fact.SourceKind {
		case "setup":
			if setupFound {
				return visit, spent, errors.New("duplicate child setup facts")
			}
			setupFound = true
			spent.PreparationActions += len(fact.Actions)
			visit.Preparation = fact.Preparation
		case "recovery":
			if recoveryFound {
				return visit, spent, errors.New("duplicate child recovery facts")
			}
			recoveryFound = true
			spent.RecoveryActions += len(fact.Actions)
		case "execute", "sequence":
			if targetFound || fact.Seq != 1 {
				return visit, spent, errors.New("campaign child contains multiple target facts")
			}
			targetFound = true
			spent.TargetActions += len(fact.Actions)
			visit.Principal, visit.Entities = fact.Principal, fact.Entities
			visit.Reason = fact.Reason
			if len(fact.BusinessExpectations) > 1 {
				return visit, spent, errors.New("campaign child has ambiguous target expectations")
			}
			if len(fact.BusinessExpectations) == 1 {
				entry := fact.BusinessExpectations[0]
				if entry.Actor != step.Actor || entry.ActionIndex != 1 {
					return visit, spent, errors.New("campaign expectation actor/action mismatch")
				}
				if entry.Decision.Frozen != nil && entry.Decision.Status == "READY" {
					if entry.Decision.Frozen.Branch != intent.Branch {
						return visit, spent, errors.New("child executed a branch different from its intent")
					}
					copy := entry.Decision.Frozen.Before
					visit.Before = &copy
				}
				visit.After, visit.Comparison = entry.After, entry.Comparison
			}
			if len(fact.Actions) > 0 {
				if len(fact.Requests) != 1 || intent.Candidate.Input == nil || !equalBusinessCampaignJSON(fact.Requests[0], *intent.Candidate.Input) || !requestMatchesAction(fact.Requests[0], fact.Actions[0]) {
					return visit, spent, errors.New("executed target request differs from the durable selected input")
				}
				if visit.Before == nil || intent.Candidate.Before == nil {
					return visit, spent, errors.New("executed target omits the frozen subject identities")
				}
				for subject, identity := range intent.Candidate.Before.Bindings {
					if identity != "" && visit.Before.Bindings[subject] != identity {
						return visit, spent, errors.New("executed target changed a selected subject identity")
					}
				}
				// Freshly rebound entities must be demonstrable from this exact
				// before read, or from this child's verified preparation capture.
				observed := ModelObservation{BeforeEntities: []EntityBinding{}, BeforeSessions: fact.BeforeSessions, Extractions: fact.Extractions, Oracles: fact.Oracles}
				for _, entity := range fact.BeforeEntities {
					fromSetup := false
					for _, prepared := range setupEntities {
						if reflect.DeepEqual(prepared, entity) {
							fromSetup = true
						}
					}
					if !fromSetup {
						observed.BeforeEntities = append(observed.BeforeEntities, entity)
					}
				}
				if err := verifyBusinessCampaignRebindings(step, observed); err != nil {
					return visit, spent, err
				}
			}
		default:
			return visit, spent, errors.New("unrecognized campaign child fact source")
		}
	}
	if !setupFound || !recoveryFound {
		return visit, spent, errors.New("campaign child omits setup or final independent checkpoint/recovery facts")
	}
	if contract.Status == "PASS" {
		if !targetFound || spent.TargetActions != 1 || visit.Before == nil || visit.After == nil || visit.Comparison == nil || visit.Comparison.Status != "PASS" || receipt.Recovery.Verdict != "PASS" {
			return visit, spent, errors.New("campaign PASS has no complete frozen/actual comparison and final fixture evidence")
		}
		if err := d.qualifyModelRun(intent.Run); err != nil {
			return visit, spent, err
		}
	} else if contract.Status != "FAIL" && contract.Status != "BLOCKED" {
		return visit, spent, errors.New("unsupported child terminal status")
	}
	spent.Actions = spent.TargetActions + spent.PreparationActions + spent.RecoveryActions
	if spent.Actions > intent.ReservedActions {
		return visit, spent, errors.New("child action count exceeds its pre-dispatch reservation")
	}
	visit.AfterStateKey, _ = businessJSONHash(struct {
		History string
		Run     string
		Binding RunBinding
		Facts   []UnifiedStepFact
	}{intent.HistorySHA256, intent.Run, intent.Binding, verified.Facts})
	visit.Reason = campaignReason(visit.Reason)
	return visit, spent, nil
}

func appendBusinessCampaignVisit(c *BusinessCampaign, visit BusinessCampaignVisit, spent BusinessCampaignSpent, intent businessCampaignDispatch) error {
	if visit.Sequence != len(c.Visits)+1 || c.HistorySHA256 != intent.HistorySHA256 {
		return errors.New("campaign child does not append to the frozen history")
	}
	for i := range c.Variants {
		if c.Variants[i].ID == visit.VariantID {
			c.Variants[i].Visits++
			if visit.Status == "PASS" {
				c.Variants[i].Passes++
			}
			c.Variants[i].LastRun = visit.Run
		}
	}
	c.Visits = append(c.Visits, visit)
	c.Spent.Actions += spent.Actions
	c.Spent.TargetActions += spent.TargetActions
	c.Spent.PreparationActions += spent.PreparationActions
	c.Spent.RecoveryActions += spent.RecoveryActions
	c.Spent.ReservedActions -= intent.ReservedActions
	c.Spent.States++ // a distinct captured child context; never merged by state label
	c.LastRun, c.LastActual = visit.Run, visit.After
	c.NextAllowed = nil
	if visit.Comparison != nil && visit.Comparison.Status == "PASS" {
		c.NextAllowed = append([]string(nil), visit.Comparison.NextAllowed...)
	}
	if visit.Status != "PASS" {
		if c.FirstDivergence == "" {
			c.FirstDivergence = visit.Run
		}
		c.Status, c.Reason = "DIVERGED", visit.Reason
		if visit.Status == "BLOCKED" {
			c.Status = "BLOCKED"
		}
	}
	c.HistorySHA256, _ = businessJSONHash(struct {
		Previous string
		Visit    BusinessCampaignVisit
	}{c.HistorySHA256, visit})
	return nil
}

func (d db) recordBusinessCampaignChild(c *BusinessCampaign, g Graph, intent businessCampaignDispatch, hash string, lease businessCampaignLease) error {
	visit, spent, err := d.campaignChildVisit(*c, intent, hash)
	if err != nil {
		return err
	}
	// A failed write must leave the caller's pending projection unchanged. A
	// shallow copy still aliases Variants and nested facts, so detach the whole
	// campaign before applying its verified child or releasing its reservation.
	next, err := cloneBusinessJSON(*c)
	if err != nil {
		return err
	}
	if err := appendBusinessCampaignVisit(&next, visit, spent, intent); err != nil {
		return err
	}
	if len(c.Visits) < c.Spent.Dispatches && len(next.Visits) == next.Spent.Dispatches {
		next.EvidenceStatus = "PASS"
		if c.Status == "UNKNOWN" && visit.Status == "PASS" && next.FirstDivergence == "" {
			next.Status, next.Reason = "RUNNING", "original pending child independently verified and reconciled without replay"
		}
	}
	next.Coverage = businessCampaignCoverage(g, next.Variants, next.Flow)
	next.Spent.ElapsedMS = businessCampaignElapsed(next)
	update := fmt.Sprintf("UPDATE business_campaign_dispatches SET status='RECORDED',updated=%s WHERE campaign_id=%s AND seq=%d AND run_id=%s AND intent_sha256=%s AND status='STARTED'", quote(time.Now().UTC().Format(time.RFC3339Nano)), quote(c.ID), intent.Sequence, quote(intent.Run), quote(hash))
	if err := d.saveBusinessCampaign(&next, lease, "CHILD_RECORDED", update); err != nil {
		return err
	}
	*c = next
	return nil
}

func (d db) reconcileBusinessCampaign(c *BusinessCampaign, g Graph, lease businessCampaignLease) error {
	rows, err := d.businessCampaignDispatches(*c)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.Intent.Sequence <= len(c.Visits) {
			if row.Status != "RECORDED" {
				return errors.New("campaign visit refers to an unsettled dispatch")
			}
			continue
		}
		if row.Status != "STARTED" {
			_, err := d.stopBusinessCampaign(c, lease, "UNKNOWN", "a durable dispatch intent has no verified completed child; automatic replay is forbidden")
			return err
		}
		if err := d.recordBusinessCampaignChild(c, g, row.Intent, row.Hash, lease); err != nil {
			_, saveErr := d.stopBusinessCampaign(c, lease, "UNKNOWN", "dispatched child remains incomplete or damaged; automatic replay is forbidden: "+err.Error())
			return saveErr
		}
	}
	return nil
}

func (d db) verifyBusinessCampaignEvidence(c BusinessCampaign) (BusinessCampaign, error) {
	bad := func(err error) (BusinessCampaign, error) {
		return blockedBusinessCampaign(c, "campaign evidence: "+err.Error()), nil
	}
	if err := validateBusinessCampaignRequest(c.Request); err != nil {
		return bad(err)
	}
	g, err := d.model(c.Graph)
	if err != nil {
		return bad(err)
	}
	if g.BusinessContract == nil {
		return bad(errors.New("campaign model has no declared business contract"))
	}
	_, runtimes, err := d.businessCampaignRuntimes(c.Request, false)
	if err != nil {
		return bad(err)
	}
	if len(runtimes) != len(c.Variants) {
		return bad(errors.New("campaign variants differ from its immutable request"))
	}
	for i := range runtimes {
		if runtimes[i].Variant != businessCampaignStaticVariant(c.Variants[i]) {
			return bad(errors.New("campaign variant identity differs from its immutable request and registered runner"))
		}
	}
	rows, err := d.businessCampaignDispatches(c)
	if err != nil {
		return bad(err)
	}
	derived := c
	derived.Variants = append([]BusinessCampaignVariant(nil), c.Variants...)
	for i := range derived.Variants {
		derived.Variants[i] = businessCampaignStaticVariant(derived.Variants[i])
	}
	derived.Visits = []BusinessCampaignVisit{}
	derived.LastRun, derived.FirstDivergence = "", ""
	derived.LastActual, derived.NextAllowed = nil, nil
	derived.Spent = BusinessCampaignSpent{}
	derived.HistorySHA256, _ = businessJSONHash(struct {
		Identity ExecutionIdentity
		Request  BusinessCampaignRequest
	}{c.Identity, c.Request})
	pending := false
	for _, row := range rows {
		intent := row.Intent
		if intent.HistorySHA256 != derived.HistorySHA256 {
			return bad(errors.New("dispatch branches from another campaign history"))
		}
		if intent.Candidate.Decision == nil {
			return bad(errors.New("dispatch has no typed decision"))
		}
		if len(derived.NextAllowed) > 0 && !containsBusinessID(derived.NextAllowed, intent.Candidate.Decision.Action) {
			return bad(errors.New("dispatch violates the previous verified next_allowed action set"))
		}
		if intent.Candidate.Eligibility != "READY" && intent.Candidate.Eligibility != "PREPARABLE" {
			return bad(errors.New("dispatch selected an ineligible candidate"))
		}
		if err := d.verifyBusinessCampaignCandidate(c, g, intent.Candidate, intent.HistorySHA256); err != nil {
			return bad(err)
		}
		derived.Spent.Dispatches++
		derived.Spent.ReservedActions += intent.ReservedActions
		if row.Status != "RECORDED" {
			if pending || intent.Sequence != len(rows) {
				return bad(errors.New("campaign has multiple or nonterminal unresolved dispatch intents"))
			}
			pending = true
			if intent.Sequence <= len(c.Visits) {
				return bad(errors.New("pending dispatch masquerades as a recorded visit"))
			}
			continue
		}
		if pending {
			return bad(errors.New("later dispatch follows an unresolved earlier intent"))
		}
		visit, spent, err := d.campaignChildVisit(c, intent, row.Hash)
		if err != nil {
			return bad(err)
		}
		if intent.Sequence > len(c.Visits) || !equalBusinessCampaignJSON(visit, c.Visits[intent.Sequence-1]) {
			return bad(errors.New("campaign visit projection differs from verified child-run facts"))
		}
		if err := appendBusinessCampaignVisit(&derived, visit, spent, intent); err != nil {
			return bad(err)
		}
	}
	if len(derived.Visits) != len(c.Visits) || !equalBusinessCampaignJSON(derived.Variants, c.Variants) || derived.HistorySHA256 != c.HistorySHA256 || !equalBusinessCampaignJSON(derived.LastActual, c.LastActual) || derived.LastRun != c.LastRun || derived.FirstDivergence != c.FirstDivergence || !equalBusinessCampaignJSON(derived.NextAllowed, c.NextAllowed) {
		return bad(errors.New("campaign coverage, history or actual-state projection differs from child evidence"))
	}
	coverage := businessCampaignCoverage(g, derived.Variants, c.Flow)
	if !equalBusinessCampaignJSON(coverage, c.Coverage) || derived.Spent.Actions != c.Spent.Actions || derived.Spent.ReservedActions != c.Spent.ReservedActions || derived.Spent.TargetActions != c.Spent.TargetActions || derived.Spent.PreparationActions != c.Spent.PreparationActions || derived.Spent.RecoveryActions != c.Spent.RecoveryActions || c.Spent.Actions+c.Spent.ReservedActions > c.Budget.MaxActions || c.Spent.States > c.Budget.MaxStates || c.Spent.States < len(c.Visits) {
		return bad(errors.New("campaign coverage or action/state budget projection is inconsistent"))
	}
	contexts, err := d.businessCampaignObservedContexts(c)
	if err != nil {
		return bad(err)
	}
	if c.Spent.Observations != contexts || c.Spent.States != contexts+len(c.Visits) {
		return bad(errors.New("campaign observed-context budget differs from its durable observation history"))
	}
	if c.Status == "COMPLETED_IN_SCOPE" && (!coverage.SelectedVariantsCovered || pending) {
		return bad(errors.New("campaign claims completion without all selected verified variants"))
	}
	// The latest frontier may precede the latest recorded child. Recover its
	// history coordinate from the selected intent instead of substituting the
	// child's expected or actual state into its prior observation.
	frontierHistory := c.HistorySHA256
	for _, candidate := range c.Frontier {
		if candidate.Selected {
			for _, row := range rows {
				if row.Intent.Run == candidate.Run {
					frontierHistory = row.Intent.HistorySHA256
				}
			}
		}
	}
	for _, candidate := range c.Frontier {
		if err := d.verifyBusinessCampaignCandidate(c, g, candidate, frontierHistory); err != nil {
			return bad(err)
		}
	}
	if pending {
		c.EvidenceStatus = "BLOCKED"
		if c.Status != "ABORTED" {
			c.Status = "UNKNOWN"
			c.Reason = "an unresolved dispatch has no verified terminal evidence; automatic replay is forbidden"
		}
	} else {
		c.EvidenceStatus = "PASS"
	}
	return c, nil
}

func (d db) businessCampaignObservedContexts(c BusinessCampaign) (int, error) {
	seen := map[string]bool{}
	err := d.each("SELECT body_json FROM business_campaign_events WHERE campaign_id="+quote(c.ID)+" ORDER BY seq", 1, func(row []string) error {
		var snapshot BusinessCampaign
		if err := decodeStrictJSON([]byte(row[0]), &snapshot); err != nil {
			return err
		}
		for _, candidate := range snapshot.Frontier {
			if candidate.Observation != nil {
				if candidate.Run == "" {
					return errors.New("observed context has no unique reserved run identity")
				}
				seen[candidate.Run] = true
			}
		}
		if len(seen) > c.Budget.MaxStates {
			return errors.New("durable observed contexts exceed the state budget")
		}
		return nil
	})
	return len(seen), err
}

func (d db) verifyBusinessCampaignCandidate(c BusinessCampaign, g Graph, candidate BusinessCampaignCandidate, history string) error {
	var variant *BusinessCampaignVariant
	for i := range c.Variants {
		if c.Variants[i].ID == candidate.VariantID {
			variant = &c.Variants[i]
		}
	}
	if variant == nil || variant.Branch != candidate.Branch || variant.Runner != candidate.Runner || candidate.Run == "" {
		return errors.New("frontier candidate is outside the frozen variant scope")
	}
	if candidate.Observation == nil {
		if candidate.Before != nil || candidate.Input != nil || candidate.Decision != nil || candidate.StateKey != "" || candidate.Eligibility == "READY" || candidate.Eligibility == "PREPARABLE" {
			return errors.New("candidate claims eligibility without independent observations")
		}
		return nil
	}
	runner, err := d.registeredRunner(candidate.Runner)
	if err != nil {
		return err
	}
	if runner.SHA256 != variant.RunnerSHA256 || runner.CodeSHA256 != variant.CodeSHA256 || len(runner.Manifest.Steps) != 1 {
		return errors.New("candidate registered runner identity changed")
	}
	step := runner.Manifest.Steps[0]
	obligation, err := validateManifest(g, runner.Manifest)
	if err != nil {
		return err
	}
	binding, err := d.startBinding(variant.Task, c.Environment, c.Request.Commit, c.Request.Account, c.Request.Fixture, c.Request.Root, c.Request.Config)
	if err != nil {
		return err
	}
	participants, err := d.participantBindings(obligation.Definition, binding, c.Request.Accounts)
	if err != nil {
		return err
	}
	obs := candidate.Observation
	if err := validateBusinessCampaignProbeIdentity(c, runner, participants, step, obs, candidate.Before != nil); err != nil {
		return err
	}
	if candidate.Input != nil {
		if err := validateRuntimeRequest(step, 1, *candidate.Input); err != nil {
			return err
		}
		if len(obs.Requests) != 1 || !equalBusinessCampaignJSON(obs.Requests[0], *candidate.Input) {
			return errors.New("candidate request differs from its observation capture")
		}
	}
	if candidate.Before == nil {
		if candidate.Decision != nil || candidate.StateKey != "" || candidate.Eligibility == "READY" || candidate.Eligibility == "PREPARABLE" {
			return errors.New("candidate decision has no complete before observation")
		}
		return nil
	}
	judged := stepRule(indexModel(g), obligation, step.Transition)
	before, err := runtimeBusinessObservation(c.Flow, judged.Oracle, step, 1, true, obs)
	if err != nil || !equalBusinessCampaignJSON(before, *candidate.Before) {
		return errors.New("candidate before differs from actual normalized readings")
	}
	if err := verifyBusinessCampaignRebindings(step, *obs); err != nil {
		return err
	}
	decision := FreezeBusinessExpectation(g, step.Node, before)
	if candidate.Decision == nil || !equalBusinessCampaignJSON(decision, *candidate.Decision) {
		return errors.New("candidate decision differs from typed pre-action evaluation")
	}
	runtime := businessCampaignVariantRuntime{Variant: *variant, Runner: runner, Binding: binding, Participants: participants}
	contextCopy := c
	contextCopy.HistorySHA256 = history
	if candidate.StateKey != campaignObservationKey(contextCopy, runtime, candidate.Run, *obs, before) {
		return errors.New("candidate context key omits or changes identity, input, actual facts or history")
	}
	if candidate.Eligibility == "READY" && (decision.Status != "READY" || decision.Frozen == nil || decision.Frozen.Branch != candidate.Branch) {
		return errors.New("candidate READY is not justified by its actual branch")
	}
	if candidate.Eligibility == "READY" {
		if err := businessCampaignPreparedPreconditions(g, obligation, step, *obs, before, before); err != nil {
			return fmt.Errorf("candidate READY preconditions: %w", err)
		}
	}
	if candidate.Preparation != nil {
		p := runner.Manifest.Preparation
		if p == nil || candidate.PreparationBefore == nil || candidate.PreparationObservation == nil {
			return errors.New("candidate preparation has no registered independent source")
		}
		if err := validateBusinessCampaignProbeIdentity(c, runner, participants, p.Observer, candidate.PreparationObservation, true); err != nil {
			return fmt.Errorf("preparation probe identity: %w", err)
		}
		actual, err := runtimeBusinessObservation(c.Flow, p.Observer.Oracles[0].ID, p.Observer, 1, true, candidate.PreparationObservation)
		if err != nil || !equalBusinessCampaignJSON(actual, *candidate.PreparationBefore) {
			return errors.New("candidate preparation before is not its actual readback")
		}
		plan := PlanPreparation(g, PreparationRequest{Observed: actual, Goal: p.Goal, MaxSteps: p.MaxSteps, MaxStates: p.MaxStates})
		if !equalBusinessCampaignJSON(plan, *candidate.Preparation) {
			return errors.New("candidate preparation plan differs from deterministic planning")
		}
		if candidate.Eligibility == "PREPARABLE" {
			if plan.Status != "READY" {
				return errors.New("candidate preparation is not READY")
			}
			predicted := before
			for _, layer := range plan.Layers {
				for _, planned := range layer.Steps {
					if _, ok := p.Adapters[planned.ID]; !ok {
						return errors.New("preparation adapter absent")
					}
					predicted = preparationWithEffects(predicted, planned.Effects)
				}
			}
			prepared := FreezeBusinessExpectation(g, step.Node, predicted)
			if prepared.Status != "READY" || prepared.Frozen == nil || prepared.Frozen.Branch != candidate.Branch {
				return errors.New("preparation predictions do not enable selected branch")
			}
			if err := businessCampaignPreparedPreconditions(g, obligation, step, *obs, before, predicted); err != nil {
				return fmt.Errorf("candidate preparation preconditions: %w", err)
			}
		}
	} else if candidate.Eligibility == "PREPARABLE" {
		return errors.New("candidate is preparable without a plan")
	}
	return nil
}

// Every projected actual observation keeps the registered reader and complete
// actor/fixture/round identity, including preparation planning reads. Typed
// value equality alone cannot turn another account's observations into ours.
func validateBusinessCampaignProbeIdentity(c BusinessCampaign, runner RegisteredRunner, participants []ParticipantBinding, step RunnerStep, obs *ModelObservation, complete bool) error {
	if obs == nil || obs.EvidenceVersion != runtimeEvidenceVersion || obs.Round != c.Round || obs.RunnerSHA256 != runner.SHA256 || obs.CodeSHA256 != runner.CodeSHA256 || !reflect.DeepEqual(obs.Participants, participants) || len(obs.Actions) != 0 {
		return errors.New("candidate observations cross round, actor or runner identity")
	}
	readers := map[string]OracleAdapter{}
	for _, oracle := range step.Oracles {
		readers[oracle.ID] = oracle
	}
	seen := map[string]bool{}
	for _, item := range obs.Oracles {
		reader, known := readers[item.ID]
		actorFound := false
		for _, participant := range participants {
			if participant.Alias == step.Actor && item.Actor == step.Actor && participant.Principal == item.Before.Principal && participant.Role == item.Before.Role {
				actorFound = true
			}
		}
		if !known || seen[item.ID] || item.Boundary != reader.Boundary || !actorFound || item.Before.Scope != runner.Manifest.IsolationScope || len(item.BeforeSHA256) != 64 || item.ActionIndex != 1 || item.Before.State == "" {
			return errors.New("candidate oracle belongs to a different registered reader, participant, boundary or fixture")
		}
		seen[item.ID] = true
	}
	if complete && len(seen) != len(readers) {
		return errors.New("complete candidate observation omits a registered independent reader")
	}
	return nil
}
