// Independent course-enrollment process. File-backed store, not the in-process
// seat httptest. Started by tests as a separate OS process.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
)

type row struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type store struct {
	mu           sync.Mutex
	path         string
	Rows         []row `json:"rows"`
	Capacity     int   `json:"capacity"`
	LoggedOut    bool  `json:"logged_out"`
	CourseClosed bool  `json:"course_closed"`
	faults       map[string]bool
	race         *barrier
}

type barrier struct {
	mu       sync.Mutex
	waiting  int
	need     int
	released chan struct{}
}

func newBarrier(need int) *barrier {
	return &barrier{need: need, released: make(chan struct{})}
}

func (b *barrier) isReleased() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.waiting >= b.need
}

func (b *barrier) arrive() {
	b.mu.Lock()
	b.waiting++
	if b.waiting == b.need {
		close(b.released)
		b.mu.Unlock()
		return
	}
	ch := b.released
	b.mu.Unlock()
	<-ch
}

func main() {
	path := os.Getenv("ENROLL_SUT_STORE")
	if path == "" {
		path = "enroll-store.json"
	}
	listen := os.Getenv("ENROLL_SUT_LISTEN")
	if listen == "" {
		listen = "127.0.0.1:0"
	}
	faults := map[string]bool{}
	for _, item := range strings.Split(os.Getenv("ENROLL_SUT_FAULTS"), ",") {
		if item = strings.TrimSpace(item); item != "" {
			faults[item] = true
		}
	}
	s := &store{path: path, Capacity: 1, faults: faults}
	s.load()
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if dest := os.Getenv("ENROLL_SUT_ADDR_FILE"); dest != "" {
		_ = os.WriteFile(dest, []byte("http://"+listener.Addr().String()), 0600)
	}
	http.HandleFunc("/", s.serve)
	if err := http.Serve(listener, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (s *store) load() {
	body, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	_ = json.Unmarshal(body, s)
	if s.Capacity < 1 {
		s.Capacity = 1
	}
}

func (s *store) save() {
	body, err := json.Marshal(s)
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, body, 0600)
}

func (s *store) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/claim":
		s.claim(w, r)
	case "/approve":
		s.mu.Lock()
		defer s.mu.Unlock()
		s.approve(w, r)
	case "/force":
		s.mu.Lock()
		defer s.mu.Unlock()
		s.force(w, r)
	case "/reset":
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.faults["reset_broken"] {
			http.Error(w, `{"error":"restore failed"}`, http.StatusServiceUnavailable)
			return
		}
		s.Rows = nil
		s.save()
		fmt.Fprint(w, `{}`)
	case "/oracle":
		s.mu.Lock()
		defer s.mu.Unlock()
		s.oracle(w, r)
	case "/debug/faults":
		s.mu.Lock()
		defer s.mu.Unlock()
		raw, _ := io.ReadAll(r.Body)
		s.faults = map[string]bool{}
		for _, item := range strings.Split(string(raw), ",") {
			if item = strings.TrimSpace(item); item != "" {
				s.faults[item] = true
			}
		}
		fmt.Fprint(w, `{}`)
	default:
		http.NotFound(w, r)
	}
}

func (s *store) actor(r *http.Request) string {
	switch r.Header.Get("X-Fixture-Actor") {
	case "admin":
		return "admin"
	case "contender":
		return "contender"
	default:
		return "student"
	}
}

func (s *store) requestID(r *http.Request) string {
	var body struct {
		RequestID string `json:"request_id"`
	}
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &body)
	if body.RequestID == "" {
		return "default"
	}
	return body.RequestID
}

func (s *store) has(id string) bool {
	for _, item := range s.Rows {
		if item.ID == id {
			return true
		}
	}
	return false
}

func (s *store) claim(w http.ResponseWriter, r *http.Request) {
	if s.faults["hang"] {
		<-r.Context().Done()
		return
	}
	if s.faults["race"] {
		s.mu.Lock()
		if s.race == nil || s.race.isReleased() {
			s.race = newBarrier(2)
		}
		gate := s.race
		observed := len(s.Rows)
		s.mu.Unlock()
		gate.arrive()
		s.mu.Lock()
		defer s.mu.Unlock()
		s.claimLocked(w, r, observed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claimLocked(w, r, len(s.Rows))
}

func (s *store) claimLocked(w http.ResponseWriter, r *http.Request, occupancy int) {
	if s.LoggedOut {
		http.Error(w, `{"error":"login required"}`, http.StatusUnauthorized)
		return
	}
	if s.CourseClosed && !s.faults["accept_closed"] {
		http.Error(w, `{"error":"course closed"}`, http.StatusForbidden)
		return
	}
	id := s.requestID(r)
	if s.has(id) && !s.faults["duplicate"] {
		fmt.Fprint(w, `{}`)
		return
	}
	if occupancy >= s.Capacity {
		http.Error(w, `{"error":"no seats"}`, http.StatusForbidden)
		return
	}
	status := "pending"
	if s.faults["skip_review"] {
		status = "enrolled"
	}
	s.Rows = append(s.Rows, row{ID: id, Status: status})
	s.save()
	fmt.Fprint(w, `{}`)
}

func (s *store) approve(w http.ResponseWriter, r *http.Request) {
	if s.actor(r) != "admin" || s.LoggedOut {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	for i, item := range s.Rows {
		if item.Status == "pending" {
			s.Rows[i].Status = "enrolled"
			s.save()
			fmt.Fprint(w, `{}`)
			return
		}
	}
	http.Error(w, `{"error":"nothing to approve"}`, http.StatusForbidden)
}

func (s *store) force(w http.ResponseWriter, r *http.Request) {
	if s.LoggedOut {
		http.Error(w, `{"error":"login required"}`, http.StatusUnauthorized)
		return
	}
	if s.actor(r) != "admin" && !s.faults["unauthorized"] {
		http.Error(w, `{"error":"admin only"}`, http.StatusForbidden)
		return
	}
	id := s.requestID(r)
	if !s.has(id) && len(s.Rows) >= s.Capacity {
		http.Error(w, `{"error":"no seats"}`, http.StatusForbidden)
		return
	}
	if s.has(id) {
		for i, item := range s.Rows {
			if item.ID == id {
				s.Rows[i].Status = "enrolled"
			}
		}
	} else {
		s.Rows = append(s.Rows, row{ID: id, Status: "enrolled"})
	}
	s.save()
	fmt.Fprint(w, `{}`)
}

func (s *store) oracle(w http.ResponseWriter, r *http.Request) {
	principal, role, admin := "synthetic-STUDENT", "STUDENT", false
	switch s.actor(r) {
	case "admin":
		principal, role, admin = "synthetic-ADMIN", "ADMIN", true
	case "contender":
		principal = "synthetic-CONTENDER"
	}
	records, enrolled := 0, 0
	state := "idle"
	pending := false
	for _, item := range s.Rows {
		records++
		if item.Status == "enrolled" {
			enrolled++
			state = "enrolled"
		} else {
			pending = true
		}
	}
	if state != "enrolled" && pending {
		state = "pending"
	}
	fmt.Fprintf(w, `{"state":%q,"principal":%q,"role":%q,"scope":%q,"login":%t,"course_open":%t,"is_admin":%t,"enrolled":%d,"capacity":%d,"records":%d}`,
		state, principal, role, "seat-fixture", !s.LoggedOut, !s.CourseClosed, admin, enrolled, s.Capacity, records)
}
