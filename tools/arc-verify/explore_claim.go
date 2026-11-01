package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const exploreLeaseDefault = 45 * time.Second

type ExploreLease struct {
	Campaign   string `json:"campaign"`
	Worker     string `json:"worker"`
	Fence      int64  `json:"fence"`
	ClaimedAt  string `json:"claimed_at"`
	LeaseUntil string `json:"lease_until"`
}

func exploreLeaseCutoff(now time.Time, ttl time.Duration) string {
	if ttl <= 0 {
		ttl = exploreLeaseDefault
	}
	return now.UTC().Add(-ttl).Format(time.RFC3339Nano)
}

func (d db) migrateExploreLeaseColumns() error {
	for _, statement := range []string{
		"ALTER TABLE exploration_state ADD COLUMN fence INTEGER NOT NULL DEFAULT 0;",
		"ALTER TABLE exploration_state ADD COLUMN lease_until TEXT NOT NULL DEFAULT '';",
	} {
		if _, err := d.handle.Exec(statement); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return err
		}
	}
	return nil
}

// claimExploreCampaign atomically assigns a campaign to worker with a new fencing token.
func (d db) claimExploreCampaign(campaign, worker string) (ExploreLease, error) {
	return d.claimExploreCampaignAt(campaign, worker, time.Now().UTC(), exploreLeaseDefault, nil)
}

func (d db) claimExploreCampaignThen(campaign, worker string, after func(*sql.Tx) error) (ExploreLease, error) {
	return d.claimExploreCampaignAt(campaign, worker, time.Now().UTC(), exploreLeaseDefault, after)
}

func (d db) claimExploreCampaignAt(campaign, worker string, now time.Time, ttl time.Duration, after func(*sql.Tx) error) (ExploreLease, error) {
	if campaign == "" || worker == "" {
		return ExploreLease{}, errors.New("claim needs a campaign and worker")
	}
	if err := d.migrateExploreLeaseColumns(); err != nil {
		return ExploreLease{}, err
	}
	if ttl <= 0 {
		ttl = exploreLeaseDefault
	}
	claimedAt := now.UTC().Format(time.RFC3339Nano)
	leaseUntil := now.UTC().Add(ttl).Format(time.RFC3339Nano)
	cutoff := exploreLeaseCutoff(now, ttl)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := d.handle.BeginTx(ctx, nil)
	if err != nil {
		return ExploreLease{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, fmt.Sprintf(
		`UPDATE exploration_state SET claimed_by=%s, claimed_at=%s, lease_until=%s, fence=fence+1, status=%s, updated=%s
WHERE campaign=%s AND status NOT IN (%s,%s) AND (
  claimed_by='' OR claimed_by=%s OR status IN (%s,%s,%s) OR (status=%s AND claimed_at<>'' AND claimed_at<%s)
)`,
		quote(worker), quote(claimedAt), quote(leaseUntil), quote(exploreRunning), quote(claimedAt),
		quote(campaign), quote(exploreCompleted), quote(exploreAborted),
		quote(worker), quote(exploreNew), quote(exploreInterrupted), quote(exploreBlocked),
		quote(exploreRunning), quote(cutoff),
	))
	if err != nil {
		return ExploreLease{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return ExploreLease{}, err
	}
	if changed != 1 {
		return ExploreLease{}, errors.New("campaign is already claimed by another worker")
	}
	row := tx.QueryRowContext(ctx, "SELECT fence FROM exploration_state WHERE campaign="+quote(campaign)+" AND claimed_by="+quote(worker))
	var fence int64
	if err := row.Scan(&fence); err != nil {
		return ExploreLease{}, err
	}
	if after != nil {
		if err := after(tx); err != nil {
			return ExploreLease{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ExploreLease{}, err
	}
	return ExploreLease{Campaign: campaign, Worker: worker, Fence: fence, ClaimedAt: claimedAt, LeaseUntil: leaseUntil}, nil
}

func (d db) saveExploreCheckpointOwned(state ExploreCheckpoint, lease ExploreLease) error {
	if state.Campaign == "" || lease.Worker == "" || lease.Fence < 1 {
		return errors.New("owned checkpoint needs campaign, worker and fencing token")
	}
	if err := d.migrateExploreLeaseColumns(); err != nil {
		return err
	}
	state.ClaimedBy = lease.Worker
	state.Fence = lease.Fence
	state.Updated = time.Now().UTC().Format(time.RFC3339Nano)
	if state.Checkpoint == "" {
		state.Checkpoint = newID("CK")
	}
	body, err := jsonMarshalCheckpoint(state)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := d.handle.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if lease.LeaseUntil == "" || lease.LeaseUntil <= now {
		return errors.New("stale worker cannot commit this checkpoint")
	}
	result, err := tx.ExecContext(ctx, fmt.Sprintf(
		`UPDATE exploration_state SET graph=%s, status=%s, model_sha=%s, source_commit=%s, claimed_by=%s, claimed_at=%s, lease_until=%s, checkpoint=%s, body=%s, updated=%s
WHERE campaign=%s AND claimed_by=%s AND fence=%d AND lease_until<>'' AND lease_until>%s AND status IN (%s,%s,%s,%s)`,
		quote(state.Graph), quote(state.Status), quote(state.ModelSHA), quote(state.SourceCommit),
		quote(lease.Worker), quote(state.Updated), quote(lease.LeaseUntil), quote(state.Checkpoint), quote(string(body)), quote(state.Updated),
		quote(state.Campaign), quote(lease.Worker), lease.Fence, quote(now),
		quote(exploreNew), quote(exploreRunning), quote(exploreInterrupted), quote(exploreBlocked),
	))
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return errors.New("stale worker cannot commit this checkpoint")
	}
	return tx.Commit()
}

func jsonMarshalCheckpoint(state ExploreCheckpoint) ([]byte, error) {
	return json.Marshal(state)
}

func (d db) renewExploreLease(lease ExploreLease, now time.Time, ttl time.Duration) (ExploreLease, error) {
	if lease.Campaign == "" || lease.Worker == "" || lease.Fence < 1 {
		return lease, errors.New("lease renew needs campaign, worker and fencing token")
	}
	if err := d.migrateExploreLeaseColumns(); err != nil {
		return lease, err
	}
	if ttl <= 0 {
		ttl = exploreLeaseDefault
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	claimedAt := now.UTC().Format(time.RFC3339Nano)
	leaseUntil := now.UTC().Add(ttl).Format(time.RFC3339Nano)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := d.handle.BeginTx(ctx, nil)
	if err != nil {
		return lease, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, fmt.Sprintf(
		`UPDATE exploration_state SET claimed_at=%s, lease_until=%s, updated=%s
WHERE campaign=%s AND claimed_by=%s AND fence=%d AND status=%s AND lease_until<>'' AND lease_until>%s`,
		quote(claimedAt), quote(leaseUntil), quote(claimedAt),
		quote(lease.Campaign), quote(lease.Worker), lease.Fence, quote(exploreRunning), quote(claimedAt),
	))
	if err != nil {
		return lease, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return lease, err
	}
	if changed != 1 {
		return lease, errors.New("lease renew failed; fencing token is no longer current")
	}
	if err := tx.Commit(); err != nil {
		return lease, err
	}
	lease.ClaimedAt = claimedAt
	lease.LeaseUntil = leaseUntil
	return lease, nil
}

func (d db) ensureExploreLease(drive *exploreDrive) error {
	if drive == nil {
		return errors.New("explore drive is missing")
	}
	now := time.Now().UTC()
	until, err := time.Parse(time.RFC3339Nano, drive.lease.LeaseUntil)
	if err != nil || !until.After(now) {
		return errors.New("explore lease has expired")
	}
	if until.Sub(now) <= exploreLeaseDefault/3 {
		renewed, err := d.renewExploreLease(drive.lease, now, exploreLeaseDefault)
		if err != nil {
			return err
		}
		drive.lease = renewed
	}
	return nil
}

func expireExploreLease(d db, campaign string) error {
	past := time.Now().UTC().Add(-2 * exploreLeaseDefault).Format(time.RFC3339Nano)
	return d.tx([]string{fmt.Sprintf("UPDATE exploration_state SET claimed_at=%s, lease_until=%s WHERE campaign=%s", quote(past), quote(past), quote(campaign))})
}
