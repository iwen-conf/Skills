package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const businessCampaignBodyLimit = 16 << 20

type businessCampaignLease struct {
	Owner string
	Fence int64
	Until string
}

type businessCampaignStored struct {
	Campaign BusinessCampaign
	Body     string
	SHA256   string
	Lease    businessCampaignLease
}

// Corrupt ledger content is different from a database I/O failure. Public
// readers retain only the indexed scope for this error, never untrusted visits.
type businessCampaignCorruption struct {
	Scope  BusinessCampaign
	Reason string
}

func (e *businessCampaignCorruption) Error() string { return e.Reason }

func blockedBusinessCampaign(scope BusinessCampaign, reason string) BusinessCampaign {
	return BusinessCampaign{
		Version: 1, ID: scope.ID, Graph: scope.Graph, Flow: scope.Flow,
		Environment: scope.Environment, Round: scope.Round,
		Status: "BLOCKED", EvidenceStatus: "BLOCKED", Reason: reason,
		Frontier: []BusinessCampaignCandidate{}, Visits: []BusinessCampaignVisit{},
		Variants: []BusinessCampaignVariant{},
	}
}

func campaignCorrupt(scope BusinessCampaign, format string, args ...any) error {
	return &businessCampaignCorruption{Scope: scope, Reason: fmt.Sprintf(format, args...)}
}

func businessCampaignEventHash(previous, body string) string {
	return digestBytes([]byte(previous + "\n" + body))
}

func businessCampaignStaticVariant(v BusinessCampaignVariant) BusinessCampaignVariant {
	v.Visits, v.Passes, v.LastRun = 0, 0, ""
	return v
}

func sameBusinessCampaignScope(first, next BusinessCampaign) bool {
	if first.Version != next.Version || first.ID != next.ID || first.Graph != next.Graph || first.Flow != next.Flow || first.Environment != next.Environment || first.Round != next.Round || first.Created != next.Created || first.Identity != next.Identity || first.Budget != next.Budget || len(first.Variants) != len(next.Variants) {
		return false
	}
	a, errA := json.Marshal(first.Request)
	b, errB := json.Marshal(next.Request)
	if errA != nil || errB != nil || string(a) != string(b) {
		return false
	}
	for i := range first.Variants {
		if businessCampaignStaticVariant(first.Variants[i]) != businessCampaignStaticVariant(next.Variants[i]) {
			return false
		}
	}
	return true
}

func validateBusinessCampaignProgress(previous, next BusinessCampaignSpent) error {
	before := []int64{int64(previous.Actions), int64(previous.TargetActions), int64(previous.PreparationActions), int64(previous.RecoveryActions), int64(previous.Dispatches), int64(previous.States), int64(previous.Observations), previous.ElapsedMS}
	after := []int64{int64(next.Actions), int64(next.TargetActions), int64(next.PreparationActions), int64(next.RecoveryActions), int64(next.Dispatches), int64(next.States), int64(next.Observations), next.ElapsedMS}
	for i := range after {
		if after[i] < 0 || after[i] < before[i] {
			return errors.New("campaign spent counters cannot become negative or decrease across history")
		}
	}
	// Conservative reservations are released after the actual child count is
	// verified; Actions+ReservedActions is deliberately not monotonic. Intent
	// and child replay separately derive the exact outstanding reservation.
	if next.ReservedActions < 0 {
		return errors.New("campaign outstanding action reservation cannot be negative")
	}
	return nil
}

func validateBusinessCampaignEnvelope(c BusinessCampaign) error {
	if c.Version != 1 || c.ID == "" || c.Graph == "" || c.Flow == "" || c.Environment == "" || c.Round == "" || c.Status == "" || c.Revision < 1 {
		return errors.New("campaign version, identity, scope, status or revision is missing")
	}
	if c.Request.Campaign != "" && c.Request.Campaign != c.ID || c.Request.Graph != c.Graph || c.Request.Flow != c.Flow || c.Request.Environment != c.Environment || c.Request.Round != c.Round || c.Request.Budget != c.Budget {
		return errors.New("campaign request differs from its frozen scope or budget")
	}
	if c.Identity.Graph != c.Graph || c.Identity.Environment != c.Environment || c.Identity.Account != c.Request.Account || c.Identity.Fixture != c.Request.Fixture || c.Identity.SourceCommit != c.Request.Commit {
		return errors.New("campaign execution identity differs from its frozen request")
	}
	if err := validateBusinessCampaignProgress(BusinessCampaignSpent{}, c.Spent); err != nil {
		return err
	}
	created, err := time.Parse(time.RFC3339Nano, c.Created)
	if err != nil {
		return errors.New("campaign creation timestamp is invalid")
	}
	updated, err := time.Parse(time.RFC3339Nano, c.Updated)
	if err != nil || updated.Before(created) {
		return errors.New("campaign update timestamp is invalid")
	}
	return nil
}

// readBusinessCampaignStored validates the entire event chain in one database
// snapshot. SHA equality alone never qualifies child-run business evidence.
func readBusinessCampaignStored(ctx context.Context, tx *sql.Tx, id string) (businessCampaignStored, error) {
	var result businessCampaignStored
	indexed := BusinessCampaign{ID: id}
	var bodySize int64
	var revision, fence string
	err := tx.QueryRowContext(ctx, `SELECT graph,flow,environment,round,status,revision,owner,fence,lease_until,
CASE WHEN length(CAST(body_json AS BLOB))<=? THEN body_json ELSE '' END,sha256,created,updated,length(CAST(body_json AS BLOB))
FROM business_campaigns WHERE id=?`, businessCampaignBodyLimit, id).Scan(
		&indexed.Graph, &indexed.Flow, &indexed.Environment, &indexed.Round, &indexed.Status, &revision,
		&result.Lease.Owner, &fence, &result.Lease.Until, &result.Body, &result.SHA256, &indexed.Created, &indexed.Updated, &bodySize,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return result, fmt.Errorf("business campaign %q not found", id)
	}
	if err != nil {
		return result, err
	}
	bad := func(format string, args ...any) (businessCampaignStored, error) {
		return businessCampaignStored{}, campaignCorrupt(indexed, format, args...)
	}
	indexed.Revision, err = strconv.Atoi(revision)
	if err != nil {
		return bad("campaign indexed revision is not an integer")
	}
	result.Lease.Fence, err = strconv.ParseInt(fence, 10, 64)
	if err != nil {
		return bad("campaign fencing token is not an integer")
	}
	if bodySize < 2 || bodySize > businessCampaignBodyLimit {
		return bad("campaign projection exceeds the 16 MiB bound or is empty")
	}
	if err := decodeStrictJSON([]byte(result.Body), &result.Campaign); err != nil {
		return bad("campaign projection is invalid: %v", err)
	}
	c := result.Campaign
	if err := validateBusinessCampaignEnvelope(c); err != nil {
		return bad("campaign projection is inconsistent: %v", err)
	}
	if c.ID != id || c.Graph != indexed.Graph || c.Flow != indexed.Flow || c.Environment != indexed.Environment || c.Round != indexed.Round || c.Status != indexed.Status || c.Revision != indexed.Revision || c.Created != indexed.Created || c.Updated != indexed.Updated {
		return bad("campaign indexed identity, scope, status or revision contradicts its projection")
	}
	if result.Lease.Fence < 1 || result.Lease.Owner == "" && result.Lease.Until != "" || result.Lease.Owner != "" && result.Lease.Until == "" {
		return bad("campaign lease metadata is inconsistent")
	}
	if result.Lease.Until != "" {
		if _, err := time.Parse(time.RFC3339Nano, result.Lease.Until); err != nil {
			return bad("campaign lease timestamp is invalid")
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT seq,kind,previous_sha256,sha256,
CASE WHEN length(CAST(body_json AS BLOB))<=? THEN body_json ELSE '' END,created,length(CAST(body_json AS BLOB))
FROM business_campaign_events WHERE campaign_id=? ORDER BY seq`, businessCampaignBodyLimit, id)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	var first BusinessCampaign
	var previousSpent BusinessCampaignSpent
	var previousHash, previousBody string
	var previousUpdated time.Time
	count := 0
	for rows.Next() {
		var seqText string
		var kind, previous, hash, body, created string
		var size int64
		if err := rows.Scan(&seqText, &kind, &previous, &hash, &body, &created, &size); err != nil {
			return result, err
		}
		count++
		seq, seqErr := strconv.Atoi(seqText)
		if seqErr != nil || seq != count || count > c.Revision || size < 2 || size > businessCampaignBodyLimit || strings.TrimSpace(kind) == "" || count == 1 && kind != "CREATE" || count > 1 && kind == "CREATE" {
			return bad("campaign event sequence, kind or body size is invalid at revision %d", count)
		}
		if previous != previousHash || hash != businessCampaignEventHash(previousHash, body) {
			return bad("campaign event hash chain is invalid at revision %d", count)
		}
		var event BusinessCampaign
		if err := decodeStrictJSON([]byte(body), &event); err != nil {
			return bad("campaign event is invalid at revision %d: %v", count, err)
		}
		if err := validateBusinessCampaignEnvelope(event); err != nil {
			return bad("campaign event is inconsistent at revision %d: %v", count, err)
		}
		if event.Revision != seq || event.ID != id || event.Updated != created {
			return bad("campaign event identity or revision is invalid at revision %d", count)
		}
		updated, _ := time.Parse(time.RFC3339Nano, event.Updated)
		if count == 1 {
			first = event
		} else if !sameBusinessCampaignScope(first, event) || updated.Before(previousUpdated) {
			return bad("campaign frozen request, identity or scope changed at revision %d", count)
		}
		if err := validateBusinessCampaignProgress(previousSpent, event.Spent); err != nil {
			return bad("campaign budget history regressed at revision %d: %v", count, err)
		}
		previousHash, previousBody, previousUpdated = hash, body, updated
		previousSpent = event.Spent
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	if count != c.Revision || previousHash != result.SHA256 || previousBody != result.Body || !sameBusinessCampaignScope(first, c) {
		return bad("campaign projection is not the last complete event")
	}
	return result, nil
}

func businessCampaignJSON(c BusinessCampaign) (string, error) {
	if err := validateBusinessCampaignEnvelope(c); err != nil {
		return "", err
	}
	body, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	if len(body) > businessCampaignBodyLimit {
		return "", errors.New("campaign snapshot exceeds the 16 MiB bound")
	}
	return string(body), nil
}

func businessCampaignOne(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("campaign changed concurrently; guarded write was not committed")
	}
	return nil
}

func validateCampaignLease(record businessCampaignStored, lease businessCampaignLease, now time.Time) error {
	if lease.Owner == "" || lease.Fence < 1 || lease != record.Lease {
		return errors.New("campaign worker lost its ownership or fencing token")
	}
	until, err := time.Parse(time.RFC3339Nano, lease.Until)
	if err != nil || !now.Before(until) {
		return errors.New("campaign worker lease has expired")
	}
	return nil
}

func (d db) createBusinessCampaign(c *BusinessCampaign, until time.Time) (businessCampaignLease, error) {
	if c == nil {
		return businessCampaignLease{}, errors.New("campaign is missing")
	}
	now := time.Now().UTC()
	if !until.After(now) {
		return businessCampaignLease{}, errors.New("campaign lease must expire in the future")
	}
	next := *c
	next.Revision = 1
	next.Created, next.Updated = now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)
	body, err := businessCampaignJSON(next)
	if err != nil {
		return businessCampaignLease{}, err
	}
	lease := businessCampaignLease{Owner: newID("BCW"), Fence: 1, Until: until.UTC().Format(time.RFC3339Nano)}
	hash := businessCampaignEventHash("", body)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tx, err := d.handle.BeginTx(ctx, nil)
	if err != nil {
		return businessCampaignLease{}, err
	}
	defer tx.Rollback()
	if err := businessCampaignOne(tx.ExecContext(ctx, `INSERT INTO business_campaigns(id,graph,flow,environment,round,status,revision,owner,fence,lease_until,body_json,sha256,created,updated) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		next.ID, next.Graph, next.Flow, next.Environment, next.Round, next.Status, next.Revision, lease.Owner, lease.Fence, lease.Until, body, hash, next.Created, next.Updated)); err != nil {
		return businessCampaignLease{}, err
	}
	if err := businessCampaignOne(tx.ExecContext(ctx, `INSERT INTO business_campaign_events(campaign_id,seq,kind,previous_sha256,sha256,body_json,created) VALUES(?,1,'CREATE','',?,?,?)`, next.ID, hash, body, next.Updated)); err != nil {
		return businessCampaignLease{}, err
	}
	if err := tx.Commit(); err != nil {
		return businessCampaignLease{}, err
	}
	*c = next
	return lease, nil
}

func (d db) claimBusinessCampaign(c BusinessCampaign, until time.Time) (businessCampaignLease, error) {
	now := time.Now().UTC()
	if !until.After(now) {
		return businessCampaignLease{}, errors.New("campaign lease must expire in the future")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tx, err := d.handle.BeginTx(ctx, nil)
	if err != nil {
		return businessCampaignLease{}, err
	}
	defer tx.Rollback()
	current, err := readBusinessCampaignStored(ctx, tx, c.ID)
	if err != nil {
		return businessCampaignLease{}, err
	}
	if current.Campaign.Revision != c.Revision || !sameBusinessCampaignScope(current.Campaign, c) || current.Campaign.Status == "ABORTED" || current.Campaign.Status == "COMPLETED_IN_SCOPE" {
		return businessCampaignLease{}, errors.New("campaign revision changed or campaign is terminal")
	}
	if current.Lease.Owner != "" {
		expires, _ := time.Parse(time.RFC3339Nano, current.Lease.Until)
		if now.Before(expires) {
			return businessCampaignLease{}, errors.New("campaign is owned by another live worker")
		}
	}
	lease := businessCampaignLease{Owner: newID("BCW"), Fence: current.Lease.Fence + 1, Until: until.UTC().Format(time.RFC3339Nano)}
	if lease.Fence < 1 {
		return businessCampaignLease{}, errors.New("campaign fencing token exhausted")
	}
	if err := businessCampaignOne(tx.ExecContext(ctx, `UPDATE business_campaigns SET owner=?,fence=?,lease_until=? WHERE id=? AND revision=? AND owner=? AND fence=? AND lease_until=?`,
		lease.Owner, lease.Fence, lease.Until, c.ID, c.Revision, current.Lease.Owner, current.Lease.Fence, current.Lease.Until)); err != nil {
		return businessCampaignLease{}, err
	}
	if err := tx.Commit(); err != nil {
		return businessCampaignLease{}, err
	}
	return lease, nil
}

func writeBusinessCampaignRevision(ctx context.Context, tx *sql.Tx, current businessCampaignStored, next BusinessCampaign, lease businessCampaignLease, kind string) error {
	body, err := businessCampaignJSON(next)
	if err != nil {
		return err
	}
	if strings.TrimSpace(kind) == "" || kind == "CREATE" || next.Revision != current.Campaign.Revision+1 || !sameBusinessCampaignScope(current.Campaign, next) {
		return errors.New("campaign save changed its frozen scope or revision")
	}
	if err := validateBusinessCampaignProgress(current.Campaign.Spent, next.Spent); err != nil {
		return err
	}
	hash := businessCampaignEventHash(current.SHA256, body)
	if err := businessCampaignOne(tx.ExecContext(ctx, `UPDATE business_campaigns SET status=?,revision=?,owner=?,fence=?,lease_until=?,body_json=?,sha256=?,updated=? WHERE id=? AND revision=? AND owner=? AND fence=? AND lease_until=? AND sha256=?`,
		next.Status, next.Revision, lease.Owner, lease.Fence, lease.Until, body, hash, next.Updated,
		next.ID, current.Campaign.Revision, current.Lease.Owner, current.Lease.Fence, current.Lease.Until, current.SHA256)); err != nil {
		return err
	}
	return businessCampaignOne(tx.ExecContext(ctx, `INSERT INTO business_campaign_events(campaign_id,seq,kind,previous_sha256,sha256,body_json,created) VALUES(?,?,?,?,?,?,?)`,
		next.ID, next.Revision, kind, current.SHA256, hash, body, next.Updated))
}

func (d db) saveBusinessCampaign(c *BusinessCampaign, lease businessCampaignLease, kind string, statements ...string) error {
	if c == nil {
		return errors.New("campaign is missing")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tx, err := d.handle.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	current, err := readBusinessCampaignStored(ctx, tx, c.ID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := validateCampaignLease(current, lease, now); err != nil {
		return err
	}
	if c.Revision != current.Campaign.Revision || current.Campaign.Status == "ABORTED" || current.Campaign.Status == "COMPLETED_IN_SCOPE" {
		return errors.New("campaign save has a stale revision or terminal predecessor")
	}
	next := *c
	next.Revision++
	next.Updated = now.Format(time.RFC3339Nano)
	if err := writeBusinessCampaignRevision(ctx, tx, current, next, lease, kind); err != nil {
		return err
	}
	for _, statement := range statements {
		if err := businessCampaignOne(tx.ExecContext(ctx, statement)); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	*c = next
	return nil
}

func (d db) releaseBusinessCampaign(id string, lease businessCampaignLease) error {
	if id == "" || lease.Owner == "" || lease.Fence < 1 {
		return errors.New("campaign release requires its ownership and fencing token")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return businessCampaignOne(d.handle.ExecContext(ctx, `UPDATE business_campaigns SET owner='',lease_until='' WHERE id=? AND owner=? AND fence=? AND lease_until=?`, id, lease.Owner, lease.Fence, lease.Until))
}

func (d db) validateBusinessCampaignLease(id string, lease businessCampaignLease) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var current businessCampaignStored
	if err := d.handle.QueryRowContext(ctx, `SELECT status,owner,fence,lease_until FROM business_campaigns WHERE id=?`, id).Scan(&current.Campaign.Status, &current.Lease.Owner, &current.Lease.Fence, &current.Lease.Until); err != nil {
		return err
	}
	if current.Campaign.Status != "RUNNING" {
		return errors.New("campaign is not running; no operation may dispatch")
	}
	return validateCampaignLease(current, lease, time.Now().UTC())
}

func (d db) readBusinessCampaign(id string) (BusinessCampaign, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tx, err := d.handle.BeginTx(ctx, nil)
	if err != nil {
		return BusinessCampaign{}, err
	}
	defer tx.Rollback()
	stored, err := readBusinessCampaignStored(ctx, tx, id)
	if err != nil {
		return BusinessCampaign{}, err
	}
	if err := tx.Commit(); err != nil {
		return BusinessCampaign{}, err
	}
	return stored.Campaign, nil
}

func (d db) loadBusinessCampaign(id string) (BusinessCampaign, error) {
	c, err := d.readBusinessCampaign(id)
	if err != nil {
		var corrupt *businessCampaignCorruption
		if errors.As(err, &corrupt) {
			return blockedBusinessCampaign(corrupt.Scope, "campaign ledger integrity: "+corrupt.Reason), nil
		}
		return BusinessCampaign{}, err
	}
	verified, err := d.verifyBusinessCampaignEvidence(c)
	if err != nil {
		return blockedBusinessCampaign(c, "campaign evidence verification: "+err.Error()), nil
	}
	return verified, nil
}

func (d db) listBusinessCampaigns(graph, environment, round string) ([]BusinessCampaign, error) {
	if graph == "" {
		return nil, errors.New("campaign list requires a graph scope")
	}
	where := "graph=" + quote(graph)
	if environment != "" {
		where += " AND environment=" + quote(environment)
	}
	if round != "" {
		where += " AND round=" + quote(round)
	}
	rows, err := d.query("SELECT id FROM business_campaigns WHERE "+where+" ORDER BY rowid DESC LIMIT 32", 1)
	if err != nil {
		return nil, err
	}
	result := make([]BusinessCampaign, 0, len(rows))
	for _, row := range rows {
		c, err := d.loadBusinessCampaign(row[0])
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, nil
}

func (d db) abortBusinessCampaign(id, reason string) (BusinessCampaign, error) {
	if id == "" || strings.TrimSpace(reason) == "" {
		return BusinessCampaign{}, errors.New("campaign abort requires an identity and reason")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tx, err := d.handle.BeginTx(ctx, nil)
	if err != nil {
		return BusinessCampaign{}, err
	}
	defer tx.Rollback()
	current, err := readBusinessCampaignStored(ctx, tx, id)
	if err != nil {
		var corrupt *businessCampaignCorruption
		if errors.As(err, &corrupt) {
			// Revocation is still meaningful when the projection is corrupt.
			// Preserve its body and event chain for diagnosis, and do not claim
			// a normal ABORT event or an independently verified recovery.
			if err := businessCampaignOne(tx.ExecContext(ctx, `UPDATE business_campaigns SET owner='',lease_until='',fence=fence+1 WHERE id=?`, id)); err != nil {
				return BusinessCampaign{}, err
			}
			if err := tx.Commit(); err != nil {
				return BusinessCampaign{}, err
			}
			return blockedBusinessCampaign(corrupt.Scope, "campaign worker lease revoked; cannot append a normal abort to corrupt history: "+corrupt.Reason), nil
		}
		return BusinessCampaign{}, err
	}
	if current.Campaign.Status == "ABORTED" {
		return current.Campaign, nil
	}
	var pending int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM business_campaign_dispatches WHERE campaign_id=? AND status<>'RECORDED'`, id).Scan(&pending); err != nil {
		return BusinessCampaign{}, err
	}
	next := current.Campaign
	next.Status, next.EvidenceStatus, next.Reason = "ABORTED", "BLOCKED", reason
	if pending > 0 {
		next.Reason += "; unresolved dispatch intent remains: side effects are unknown and must not be replayed automatically"
	}
	next.Revision++
	next.Updated = time.Now().UTC().Format(time.RFC3339Nano)
	lease := businessCampaignLease{Fence: current.Lease.Fence + 1}
	if lease.Fence < 1 {
		return BusinessCampaign{}, errors.New("campaign fencing token exhausted")
	}
	if err := writeBusinessCampaignRevision(ctx, tx, current, next, lease, "ABORT"); err != nil {
		return BusinessCampaign{}, err
	}
	if err := tx.Commit(); err != nil {
		return BusinessCampaign{}, err
	}
	return next, nil
}
