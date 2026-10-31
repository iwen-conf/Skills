package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Implementation struct {
	Graph     string     `json:"graph"`
	Commit    string     `json:"commit,omitempty"`
	Entrances []Entrance `json:"entrances"`
}
type Entrance struct {
	Action   string `json:"action"`
	Status   string `json:"status"`
	Evidence string `json:"evidence"`
}
type Fixtures struct {
	Fixtures []Fixture `json:"fixtures"`
}
type Fixture struct {
	ID        string            `json:"id"`
	Kind      string            `json:"kind"`
	State     string            `json:"state"`
	SecretRef string            `json:"secretRef,omitempty"`
	Refs      map[string]string `json:"refs,omitempty"`
}

func validateFixtures(f Fixtures) error {
	if len(f.Fixtures) == 0 {
		return errors.New("fixture list empty")
	}
	seen := map[string]bool{}
	for _, v := range f.Fixtures {
		if v.ID == "" || v.Kind == "" || v.State == "" || seen[v.ID] {
			return fmt.Errorf("invalid/duplicate fixture %q", v.ID)
		}
		seen[v.ID] = true
		for key := range v.Refs {
			k := strings.ToLower(key)
			for _, secret := range []string{"password", "secret", "token", "cookie", "authorization", "apikey", "api_key"} {
				if strings.Contains(k, secret) {
					return fmt.Errorf("fixture %s contains credential-like metadata key %s", v.ID, key)
				}
			}
		}
		if v.SecretRef != "" && !strings.HasPrefix(v.SecretRef, "env://") && !strings.HasPrefix(v.SecretRef, "secret://") {
			return fmt.Errorf("fixture %s must use a secret reference, never plaintext", v.ID)
		}
	}
	return nil
}
func (d db) importFixtures(f Fixtures) (int, error) {
	if e := validateFixtures(f); e != nil {
		return 0, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tx, e := d.handle.BeginTx(ctx, nil)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	added := 0
	for _, v := range f.Fixtures {
		b, e := json.Marshal(v)
		if e != nil {
			return 0, e
		}
		var existing string
		e = tx.QueryRowContext(ctx, "SELECT body FROM fixtures WHERE id="+quote(v.ID)).Scan(&existing)
		if e == nil {
			if existing != string(b) {
				return 0, fmt.Errorf("fixture %s already has another recipe; register a new identity", v.ID)
			}
			continue
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return 0, e
		}
		result, e := tx.ExecContext(ctx, fmt.Sprintf("INSERT INTO fixtures(id,body) VALUES(%s,%s);", quote(v.ID), quote(string(b))))
		if e != nil {
			return 0, e
		}
		changed, e := result.RowsAffected()
		if e != nil {
			return 0, e
		}
		if changed != 1 {
			return 0, errors.New("fixture was not inserted")
		}
		added++
	}
	if e = tx.Commit(); e != nil {
		return 0, e
	}
	return added, nil
}
func (d db) fixture(id string) ([][]string, error) {
	return d.query("SELECT "+fieldsSQL("id", "body")+" FROM fixtures WHERE id="+quote(id)+";", 2)
}
func (d db) scan(i Implementation) (int, error) {
	if i.Graph == "" || len(i.Entrances) == 0 {
		return 0, errors.New("implementation scan requires graph and entrances")
	}
	if i.Commit != "" && !fullCommit(i.Commit) {
		return 0, errors.New("scan commit must be the candidate's full lowercase hexadecimal commit id")
	}
	check, e := d.query("SELECT "+fieldsSQL("id")+" FROM graphs WHERE id="+quote(i.Graph)+";", 1)
	if e != nil {
		return 0, e
	}
	if len(check) != 1 {
		return 0, errors.New("graph not found")
	}
	stmts := []string{}
	for _, entry := range i.Entrances {
		switch entry.Status {
		case "PRESENT", "NOT_FOUND", "UNKNOWN":
		default:
			return 0, fmt.Errorf("invalid scan status %q", entry.Status)
		}
		found, e := d.query("SELECT "+fieldsSQL("id")+" FROM nodes WHERE graph="+quote(i.Graph)+" AND id="+quote(entry.Action)+" AND kind='action';", 1)
		if e != nil {
			return 0, e
		}
		if len(found) != 1 {
			return 0, fmt.Errorf("unknown action %s", entry.Action)
		}
		path, b, e := loadEvidence(entry.Evidence)
		if e != nil {
			return 0, fmt.Errorf("scan evidence: %w", e)
		}
		hash := sha256.Sum256(b)
		stmts = append(stmts, fmt.Sprintf("INSERT INTO implementation_scans(graph,action,status,evidence,sha256) VALUES(%s,%s,%s,%s,%s);", quote(i.Graph), quote(entry.Action), quote(entry.Status), quote(path), quote(hex.EncodeToString(hash[:]))))
		if i.Commit != "" {
			stmts = append(stmts, fmt.Sprintf("INSERT INTO implementation_scan_candidates(scan,commit_sha) SELECT MAX(id),%s FROM implementation_scans WHERE graph=%s AND action=%s;", quote(i.Commit), quote(i.Graph), quote(entry.Action)))
		}
	}
	if e := d.tx(stmts); e != nil {
		return 0, e
	}
	return len(i.Entrances), nil
}

func fullCommit(commit string) bool {
	if len(commit) != 40 && len(commit) != 64 {
		return false
	}
	for _, r := range commit {
		if !('0' <= r && r <= '9' || 'a' <= r && r <= 'f') {
			return false
		}
	}
	return true
}

// ScanObservation is the latest implementation scan of one action.
type ScanObservation struct {
	Status, Evidence, SHA256, Commit string
}

func (d db) latestScans(graph string) (map[string]ScanObservation, error) {
	rows, e := d.query("SELECT "+fieldsSQL("s.action", "s.status", "s.evidence", "s.sha256", "COALESCE(c.commit_sha,'')")+" FROM implementation_scans s LEFT JOIN implementation_scan_candidates c ON c.scan=s.id WHERE s.graph="+quote(graph)+" AND s.id=(SELECT MAX(l.id) FROM implementation_scans l WHERE l.graph=s.graph AND l.action=s.action);", 5)
	if e != nil {
		return nil, e
	}
	result := map[string]ScanObservation{}
	for _, row := range rows {
		result[row[0]] = ScanObservation{Status: row[1], Evidence: row[2], SHA256: row[3], Commit: row[4]}
	}
	return result, nil
}

// existenceVerdict judges an EXISTENCE obligation from implementation evidence.
// NOT_FOUND stays a candidate gap: only a widened search confirmed by the user
// may record missing functionality, so it never becomes FAIL here.
func existenceVerdict(scan ScanObservation, found bool, candidate string, current bool, staleReason string) (string, string) {
	if !found {
		return "NOT_RUN", "no implementation scan recorded for the rule action"
	}
	switch scan.Status {
	case "NOT_FOUND":
		return "BLOCKED", "candidate gap: implementation scan found no entry; widen the search and confirm with the user before recording MISSING"
	case "PRESENT":
	default:
		return "BLOCKED", "implementation scan could not decide whether the entry exists"
	}
	if _, raw, e := loadEvidence(scan.Evidence); e != nil || digestBytes(raw) != scan.SHA256 {
		return "STALE", "implementation scan evidence changed or is unreadable"
	}
	if scan.Commit == "" {
		return "BLOCKED", "implementation scan has no candidate commit; rescan with the candidate commit"
	}
	if !current {
		return "STALE", staleReason
	}
	if scan.Commit != candidate {
		return "STALE", "implementation scan belongs to another candidate commit"
	}
	return "PASS", "current candidate has independent implementation evidence"
}
func (d db) gaps(graph string) ([][]string, error) {
	// NOT_FOUND is a candidate discovery gap, never automatic confirmation of missing functionality.
	return d.query("SELECT "+fieldsSQL("r.id", "r.action", "COALESCE((SELECT s.status FROM implementation_scans s WHERE s.graph=r.graph AND s.action=r.action ORDER BY s.id DESC LIMIT 1),'UNOBSERVED')")+" FROM rules r WHERE r.graph="+quote(graph)+" AND r.approved=1 ORDER BY r.id;", 3)
}
