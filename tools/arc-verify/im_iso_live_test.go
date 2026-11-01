package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	imISOScope    = "im-iso28886"
	imISOGraphID  = "im-support-v1.1"
	imISOGraphRel = "docs/07-测试质量/业务流/graphs/im-support-v1.json"
)

type imISOWorld struct {
	mu                                            sync.Mutex
	jwt, adminUser, adminPass                     string
	adminTok, customerTok, agentTok, contenderTok string
	customerUID, agentUID, contenderUID           int
	sessionID, workItem, workVer, assigned        int
	humanLive, isoLive                            int
	writerOK                                      bool
}

type imISOClaim struct {
	Rule, Expected, Verdict, Evidence string
}

func TestIMMongoIsolatedKnownRuleExploreAndContrast(t *testing.T) {
	if !lockLiveIMToHost60(t) {
		return
	}
	if err := refuseSharedIMTarget(imISOWriteURL()); err != nil {
		t.Fatal(err)
	}
	if err := refuseOffHost60(imISOWriteURL()); err != nil {
		t.Fatal(err)
	}
	world := requireIMISO(t)
	if err := world.provision(t); err != nil {
		t.Fatal(err)
	}
	claims := world.runKnownRules(t)
	for _, claim := range claims {
		if claim.Verdict == "FAIL" {
			t.Fatalf("%s failed: %s", claim.Rule, claim.Evidence)
		}
		if claim.Verdict != "PASS" {
			t.Fatalf("%s unexpected %s: %s", claim.Rule, claim.Verdict, claim.Evidence)
		}
	}

	exploreWorld := requireIMISO(t)
	if err := exploreWorld.provision(t); err != nil {
		t.Fatal(err)
	}
	proxy := newIMISOProxy(t, exploreWorld)
	graph := loadIMISOGraph(t)
	planner := &ScriptedExplorePlanner{Suggestions: []PlannerSuggestion{{
		Capability: "capability.intake-chain", Action: "claim_work_item", Entry: "/force", Role: "CUSTOMER",
		Reason: "customer may still reach claim after first message", Observe: "assigned", Risk: "high",
	}}}
	runMode := func(mode string, planner ExplorePlanner, seed int) ExploreReport {
		t.Helper()
		fixture := prepareIMISOFixture(t, graph, proxy.URL)
		fixture.round = openGraphRound(t, fixture, imISOGraphID)
		report, err := fixture.d.runExploration(t.Context(), imISOExploreReq(fixture, mode, planner, seed))
		if err != nil || report.Actions == 0 {
			t.Fatalf("%s explore: %+v %v", mode, report, err)
		}
		return report
	}
	det := runMode("deterministic", nil, 1)
	heur := runMode("heuristic", nil, 2)
	assisted := runMode("ai", planner, 3)
	report := map[string]any{
		"env": imIsolationEnv, "target": imISOWriteURL(), "host": collectHostIdentity(),
		"human_live": world.humanLive, "iso_live": world.isoLive, "writer_ok": world.writerOK,
		"customer_uid": world.customerUID, "session_id": world.sessionID, "assigned": world.assigned,
		"known_rules":            claims,
		"deterministic":          map[string]any{"campaign": det.Campaign, "actions": det.Actions, "stop": det.StopReason, "unspecified": len(det.Unspecified)},
		"heuristic":              map[string]any{"campaign": heur.Campaign, "actions": heur.Actions, "unspecified": len(heur.Unspecified)},
		"ai":                     map[string]any{"campaign": assisted.Campaign, "actions": assisted.Actions, "planner_calls": planner.Calls},
		"not":                    []string{"lan50-write", "28885-write", "28895", "production", "off-host-60"},
		"binary_mutation_deploy": "NOT_RUN: living isolation writer was not restarted",
	}
	writeExploreArtifact(t, "reports/im-mongo-iso28886.json", report)
	writeIMISOArtifact(t, "reports/im-mongo-iso28886.json", report)
}

func lockLiveIMToHost60(t *testing.T) bool {
	t.Helper()
	id := collectHostIdentity()
	if !id.On60 {
		writeRemainArtifact(t, "reports/im-host60.json", map[string]any{"verdict": "BLOCKED", "reason": "not on 192.168.0.60", "host": id})
		if host60Required() {
			t.Fatalf("BLOCKED: live IM must run on 192.168.0.60, host=%+v", id)
		}
		return false
	}
	base := imISOWriteURL()
	if err := refuseOffHost60(base); err != nil {
		t.Fatal(err)
		return false
	}
	live := httpStatus(t, base+"/api/im/health/live")
	ready := httpStatus(t, base+"/api/im/health/ready")
	if live != 200 || ready != 200 {
		writeRemainArtifact(t, "reports/im-mongo-chains.json", map[string]any{
			"verdict": "BLOCKED", "reason": "no isolation writer on host 60", "base": base, "live": live, "ready": ready, "host": id,
			"chains": []string{"A-consult-claim", "B-cross-role", "C-concurrency-recovery"},
		})
		writeExploreArtifact(t, "reports/im-mongo-iso28886-blocked.json", map[string]any{
			"verdict": "BLOCKED", "reason": "no isolation writer on host 60", "base": base, "live": live, "ready": ready, "host": id,
		})
		return false
	}
	return true
}

func writeIMISOArtifact(t *testing.T, rel string, value any) {
	t.Helper()
	path := filepath.Join("..", "..", ".arc", "artifacts", "verify-im-mongo-iso", rel)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
}

func requireIMISO(t *testing.T) *imISOWorld {
	t.Helper()
	base := imISOWriteURL()
	if err := refuseOffHost60(base); err != nil {
		t.Fatal(err)
	}
	isoLive := httpStatus(t, base+"/api/im/health/live")
	isoReady := httpStatus(t, base+"/api/im/health/ready")
	if isoLive != 200 || isoReady != 200 {
		writeExploreArtifact(t, "reports/im-mongo-iso28886-blocked.json", map[string]any{
			"verdict": "BLOCKED", "reason": "host-60 isolation writer health is not 200", "iso_live": isoLive, "iso_ready": isoReady, "base": base, "host": collectHostIdentity(),
		})
		t.Fatalf("host-60 isolation writer is not healthy: live=%d ready=%d base=%s", isoLive, isoReady, base)
	}
	world := &imISOWorld{isoLive: isoLive, writerOK: isolationWriterOK(t)}
	world.loadSecrets(t)
	return world
}

func isolationWriterOK(t *testing.T) bool {
	t.Helper()
	if !runningOnHost60() {
		return false
	}
	return httpStatus(t, imISOWriteURL()+"/api/im/health/live") == 200 && httpStatus(t, imISOWriteURL()+"/api/im/health/ready") == 200
}

func (world *imISOWorld) loadSecrets(t *testing.T) {
	t.Helper()
	root := imMongoRoot()
	if root == "" {
		t.Fatal("IM-mongo root is required for isolation credentials")
	}
	script := `import json,sys,yaml
c=yaml.safe_load(open(sys.argv[1]))
print(json.dumps({"jwt":c["auth"]["jwt_secret"],"username":c["console"]["accounts"][0]["username"],"password":c["console"]["accounts"][0]["password"]}))`
	out, err := exec.Command("python3", "-c", script, filepath.Join(root, ".t47-isolated-configs.yaml")).Output()
	if err != nil {
		t.Fatalf("load isolation console: %v", err)
	}
	var loaded struct {
		JWT      string `json:"jwt"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(out, &loaded); err != nil || loaded.JWT == "" || loaded.Username == "" || loaded.Password == "" {
		t.Fatal("isolation console or jwt is missing")
	}
	world.jwt, world.adminUser, world.adminPass = loaded.JWT, loaded.Username, loaded.Password
}

func (world *imISOWorld) provision(t *testing.T) error {
	t.Helper()
	admin, err := world.call("POST", "/api/im/admin/auth/login", "", map[string]any{"username": world.adminUser, "password": world.adminPass})
	if err != nil {
		return err
	}
	world.adminTok = pickString(dataOf(admin), "access_token", "token")
	if world.adminTok == "" {
		return fmt.Errorf("admin login returned no token http=%v", admin["_http"])
	}
	stamp := time.Now().UTC().Format("150405")
	pass := "IsoArc" + stamp
	for idx := 1; idx <= 2; idx++ {
		mobile := fmt.Sprintf("171%s%02d", stamp, idx)
		if len(mobile) != 11 {
			mobile = (mobile + "00000000000")[:11]
		}
		created, err := world.call("POST", "/api/im/admin/customer-service/agents", world.adminTok, map[string]any{
			"mobile": mobile, "nickname": fmt.Sprintf("iso-arc-%d", idx), "password": pass, "user_type": 3,
		})
		if err != nil {
			return err
		}
		uid := pickInt(dataOf(created), "user_id", "id", "uid")
		if uid == 0 {
			return fmt.Errorf("provision agent %d failed http=%v msg=%v", idx, created["_http"], created["message"])
		}
		login, err := world.loginAgent(mobile, pass)
		if err != nil {
			return fmt.Errorf("agent %d login: %w", uid, err)
		}
		tok := pickString(dataOf(login), "token", "access_token")
		if tok == "" {
			return fmt.Errorf("agent %d login returned no token http=%v code=%v msg=%v", uid, login["_http"], login["code"], login["message"])
		}
		profile, err := world.call("GET", "/api/im/v2/support/profile", tok, nil)
		if err != nil || !bizOK(profile) {
			profile, err = world.call("GET", "/api/im/support/profile", tok, nil)
		}
		if err != nil || !bizOK(profile) {
			return fmt.Errorf("agent %d profile rejected login_http=%v login_code=%v profile_http=%v profile_code=%v msg=%v", uid, login["_http"], login["code"], profile["_http"], profile["code"], profile["message"])
		}
		if _, err := world.call("POST", "/api/im/v2/support/online", tok, map[string]any{"is_online": true}); err != nil {
			return err
		}
		_, _ = world.call("PUT", "/api/im/v2/support/presence", tok, map[string]any{"presence": "available"})
		_, _ = world.call("POST", "/api/im/v2/support/heartbeat", tok, nil)
		if idx == 1 {
			world.agentUID, world.agentTok = uid, tok
		} else {
			world.contenderUID, world.contenderTok = uid, tok
		}
	}
	uid, err := isolationCustomerUID(t)
	if err != nil {
		return err
	}
	world.customerUID = uid
	world.customerTok = mintUserJWT(world.jwt, uid)
	return nil
}

func (world *imISOWorld) runKnownRules(t *testing.T) []imISOClaim {
	t.Helper()
	var claims []imISOClaim
	unauth, err := world.call("GET", "/api/im/v2/support/session-line", "", nil)
	claims = append(claims, imClaim("R-LINES-CURRENT/NEG", "unauthenticated current-line is 401", err == nil && asInt(unauth["_http"]) == 401, fmt.Sprintf("http=%v", unauth["_http"])))

	if err := world.openOnly(); err != nil {
		claims = append(claims, imClaim("R-OPEN-V2", "V2 Open returns UID session", false, err.Error()))
		return claims
	}
	claims = append(claims, imClaim("R-OPEN-V2", "V2 Open returns UID session", world.sessionID > 0, fmt.Sprintf("uid=%d sid=%d", world.customerUID, world.sessionID)))

	firstEvidence, err := world.firstOnly()
	if err != nil {
		claims = append(claims, imClaim("R-FIRST-MSG", "customer first text is accepted", false, err.Error()))
		return claims
	}
	st, _ := world.serviceState()
	if world.workItem == 0 {
		world.workItem = st.workItem
	}
	claims = append(claims, imClaim("R-FIRST-MSG", "customer first text is accepted", world.workItem > 0, fmt.Sprintf("sid=%d work_item=%d %s state_http keys=%s", world.sessionID, world.workItem, firstEvidence, strings.Join(isoJSONKeys(st.raw), ","))))

	denied, err := world.call("POST", fmt.Sprintf("/api/im/v2/support/queue/%d/claim", world.workItem), world.customerTok, map[string]any{"expected_version": world.workVer})
	deniedOK := err == nil && asInt(denied["_http"]) != 200
	afterDeny, _ := world.serviceState()
	deniedOK = deniedOK && afterDeny.assigned == 0
	claims = append(claims, imClaim("R-CLAIM/FORCED", "customer cannot claim occupying intake", deniedOK, fmt.Sprintf("http=%v assigned=%d", denied["_http"], afterDeny.assigned)))

	claim, err := world.call("POST", fmt.Sprintf("/api/im/v2/support/queue/%d/claim", world.workItem), world.agentTok, map[string]any{"expected_version": world.workVer})
	if err != nil || asInt(claim["_http"]) != 200 {
		next, nextErr := world.call("POST", "/api/im/v2/support/queue/claim-next", world.agentTok, map[string]any{})
		if nextErr == nil && asInt(next["_http"]) == 200 {
			claim = next
			err = nil
		}
	}
	st, _ = world.serviceState()
	world.assigned = st.assigned
	claimed := err == nil && (asInt(claim["_http"]) == 200 || st.assigned == world.agentUID)
	claims = append(claims, imClaim("R-CLAIM", "qualified agent claims occupying intake", claimed, fmt.Sprintf("http=%v code=%v msg=%v assigned=%d agent=%d work_item=%d", claim["_http"], claim["code"], claim["message"], st.assigned, world.agentUID, world.workItem)))

	steal, err := world.call("POST", fmt.Sprintf("/api/im/v2/support/queue/%d/claim", world.workItem), world.contenderTok, map[string]any{"expected_version": st.workVer})
	afterSteal, _ := world.serviceState()
	noSteal := err == nil && afterSteal.assigned == world.agentUID && afterSteal.assigned != world.contenderUID
	if asInt(steal["_http"]) == 200 && afterSteal.assigned == world.contenderUID {
		noSteal = false
	}
	claims = append(claims, imClaim("R-CLAIM/RACE", "second agent cannot steal an assigned intake", noSteal, fmt.Sprintf("http=%v assigned=%d", steal["_http"], afterSteal.assigned)))

	if world.assigned > 0 {
		sent, err := world.call("POST", fmt.Sprintf("/api/im/v2/support/sessions/%d/messages/text", world.sessionID), world.agentTok, map[string]any{
			"client_message_id": uuidV7(), "text": "iso-arc agent",
		})
		claims = append(claims, imClaim("R-SEND-ASSIGNED", "assigned agent can send text", err == nil && bizOK(sent), fmt.Sprintf("http=%v code=%v", sent["_http"], sent["code"])))
	}
	acl, err := world.call("POST", "/api/im/v2/support/sessions", world.agentTok, map[string]any{"source_type": "novel", "shop_user_id": 1})
	aclOK := err == nil && (asInt(acl["_http"]) == 401 || asInt(acl["_http"]) == 403)
	claims = append(claims, imClaim("ACL-AGENT-USER-ROUTE", "support agent token cannot open a customer session", aclOK, fmt.Sprintf("http=%v", acl["_http"])))
	return claims
}

func imClaim(rule, expected string, ok bool, evidence string) imISOClaim {
	verdict := "FAIL"
	if ok {
		verdict = "PASS"
	}
	return imISOClaim{Rule: rule, Expected: expected, Verdict: verdict, Evidence: evidence}
}

func (world *imISOWorld) loginAgent(mobile, pass string) (map[string]any, error) {
	attempts := []struct {
		path string
		body map[string]any
	}{
		{"/api/im/support/auth/login", map[string]any{"mobile": mobile, "password": passwordEnvelope(pass), "platform": "mac", "client_version": "1.0.0"}},
		{"/api/im/v2/support/auth/login", map[string]any{"mobile": mobile, "password": passwordEnvelope(pass), "platform": "web", "client_version": "chensi-support/iso-arc+1", "login_as": "agent"}},
		{"/api/im/support/auth/login", map[string]any{"mobile": mobile, "password": passwordEnvelope(pass), "platform": "web", "client_version": "chensi-support/iso-arc+1"}},
		{"/api/im/support/auth/login", map[string]any{"mobile": mobile, "password": pass, "platform": "mac", "client_version": "1.0.0"}},
	}
	var last map[string]any
	var lastErr error
	for _, attempt := range attempts {
		resp, err := world.call("POST", attempt.path, "", attempt.body)
		if err != nil {
			lastErr = err
			continue
		}
		last = resp
		if bizOK(resp) && pickString(dataOf(resp), "token", "access_token") != "" {
			return resp, nil
		}
	}
	if last != nil {
		return last, fmt.Errorf("http=%v code=%v msg=%v", last["_http"], last["code"], last["message"])
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("agent login produced no response")
}

func (world *imISOWorld) openOnly() error {
	opened, err := world.call("POST", "/api/im/v2/support/sessions", world.customerTok, map[string]any{"source_type": "novel", "shop_user_id": 1})
	if err != nil {
		return err
	}
	world.sessionID = pickInt(dataOf(opened), "session_id", "id")
	if world.sessionID == 0 || !bizOK(opened) {
		return fmt.Errorf("open http=%v code=%v sid=%d msg=%v", opened["_http"], opened["code"], world.sessionID, opened["message"])
	}
	if st, err := world.serviceState(); err == nil && !isolationSessionIsFresh(st.raw) {
		return fmt.Errorf("open reused existing cycle sid=%d cycle=%d work_item=%d status=%v", world.sessionID, pickInt(st.raw, "cycle_id"), pickInt(st.raw, "work_item_id"), st.raw["cycle_status"])
	}
	return nil
}

func (world *imISOWorld) firstOnly() (string, error) {
	first, err := world.call("POST", fmt.Sprintf("/api/im/v2/support/sessions/%d/messages/text", world.sessionID), world.customerTok, map[string]any{
		"client_message_id": uuidV7(), "text": "iso-arc first",
	})
	if err != nil {
		return "", err
	}
	evidence := fmt.Sprintf("http=%v code=%v keys=%s", first["_http"], first["code"], strings.Join(isoJSONKeys(dataOf(first)), ","))
	if !bizOK(first) {
		return evidence, fmt.Errorf("first %s msg=%v", evidence, first["message"])
	}
	if st, err := world.serviceState(); err == nil {
		world.workItem, world.workVer, world.assigned = st.workItem, st.workVer, st.assigned
	}
	if world.workItem == 0 {
		world.workItem = pickInt(dataOf(first), "work_item_id", "intake_id", "id")
	}
	if world.workItem == 0 {
		if nested, ok := dataOf(first)["work_item"].(map[string]any); ok {
			world.workItem = pickInt(nested, "id", "work_item_id")
		}
	}
	return evidence, nil
}

func (world *imISOWorld) openAndFirst() error {
	if err := world.openOnly(); err != nil {
		return err
	}
	_, err := world.firstOnly()
	return err
}

func isolationCustomerUID(t *testing.T) (int, error) {
	t.Helper()
	if uid, err := unusedIsolationCustomerUID(); err == nil && uid > 0 {
		return uid, nil
	}
	return createIsolationCustomer()
}

func unusedIsolationCustomerUID() (int, error) {
	return 0, fmt.Errorf("BLOCKED: host-lock 60 does not SSH to .50 for customer UID")
}

func createIsolationCustomer() (int, error) {
	return 0, fmt.Errorf("BLOCKED: host-lock 60 does not SSH to .50 to create customers")
}

func isolationSessionIsFresh(state map[string]any) bool {
	return pickInt(state, "cycle_id") == 0 && pickInt(state, "work_item_id") == 0
}

func parseFirstPositiveID(raw string) (int, error) {
	for _, line := range strings.Split(raw, "\n") {
		id, err := strconv.Atoi(strings.TrimSpace(line))
		if err == nil && id > 0 {
			return id, nil
		}
	}
	return 0, fmt.Errorf("missing id")
}

type imService struct {
	workItem, workVer, assigned int
	raw                         map[string]any
}

func (world *imISOWorld) serviceState() (imService, error) {
	var out imService
	resp, err := world.call("GET", fmt.Sprintf("/api/im/v2/support/sessions/%d/service-state", world.sessionID), world.customerTok, nil)
	if err != nil {
		return out, err
	}
	data := dataOf(resp)
	out.raw = data
	out.workItem = pickInt(data, "work_item_id")
	out.workVer = pickInt(data, "work_item_version")
	out.assigned = pickInt(data, "assigned_agent_uid", "assigned_agent_id")
	return out, nil
}

func isoJSONKeys(doc map[string]any) []string {
	keys := make([]string, 0, len(doc))
	for key := range doc {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "password") || strings.Contains(lower, "secret") {
			continue
		}
		keys = append(keys, key)
	}
	return keys
}

func (world *imISOWorld) call(method, path, token string, body any) (map[string]any, error) {
	base := imISOWriteURL()
	if err := refuseSharedIMTarget(base); err != nil {
		return nil, err
	}
	if err := refuseOffHost60(base); err != nil {
		return nil, err
	}
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(method, base+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	doc := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &doc)
	}
	doc["_http"] = resp.StatusCode
	return doc, nil
}

func newIMISOProxy(t *testing.T, world *imISOWorld) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := r.Header.Get("X-Fixture-Actor")
		switch {
		case r.URL.Path == "/oracle" && r.Method == http.MethodGet:
			writeIMISOOracle(w, world, actor)
		case r.URL.Path == "/open":
			if err := world.openOnly(); err != nil && world.sessionID == 0 {
				http.Error(w, `{"error":"open"}`, http.StatusConflict)
				return
			}
			fmt.Fprint(w, `{}`)
		case r.URL.Path == "/first":
			if _, err := world.firstOnly(); err != nil && world.workItem == 0 {
				http.Error(w, `{"error":"first"}`, http.StatusConflict)
				return
			}
			fmt.Fprint(w, `{}`)
		case r.URL.Path == "/force":
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		case r.URL.Path == "/claim":
			tok := world.agentTok
			if actor == "contender" {
				tok = world.contenderTok
			}
			if actor == "customer" {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			resp, err := world.call("POST", fmt.Sprintf("/api/im/v2/support/queue/%d/claim", world.workItem), tok, map[string]any{"expected_version": world.workVer})
			if err != nil || asInt(resp["_http"]) != 200 {
				http.Error(w, `{"error":"denied"}`, http.StatusForbidden)
				return
			}
			if st, err := world.serviceState(); err == nil {
				world.assigned, world.workVer, world.workItem = st.assigned, st.workVer, maxInt(world.workItem, st.workItem)
			}
			fmt.Fprint(w, `{}`)
		case r.URL.Path == "/reset":
			if world.sessionID > 0 && world.assigned > 0 {
				_, _ = world.call("POST", fmt.Sprintf("/api/im/v2/support/sessions/%d/close", world.sessionID), world.agentTok, map[string]any{"resolution_code": "resolved"})
			}
			world.sessionID, world.workItem, world.workVer, world.assigned = 0, 0, 0, 0
			fmt.Fprint(w, `{}`)
		default:
			http.Error(w, `{"error":"unknown"}`, http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestIMISOOracleReadsLiveServiceState(t *testing.T) {
	if !lockLiveIMToHost60(t) {
		return
	}
	if err := refuseSharedIMTarget(imISOWriteURL()); err != nil {
		t.Fatal(err)
	}
	world := requireIMISO(t)
	if err := world.provision(t); err != nil {
		t.Fatal(err)
	}
	if err := world.openOnly(); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	writeIMISOOracle(rec, world, "customer")
	var facts map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &facts); err != nil {
		t.Fatal(err)
	}
	if facts["oracle_source"] != "service-state" {
		t.Fatalf("oracle must re-read live service-state, got %v body=%s", facts["oracle_source"], rec.Body.String())
	}
	if asInt(facts["session_id"]) != world.sessionID || world.sessionID == 0 {
		t.Fatalf("live oracle session_id mismatch: facts=%v world=%d", facts["session_id"], world.sessionID)
	}
}

func runIMMongoAuthorizedChains(t *testing.T) {
	t.Helper()
	if !lockLiveIMToHost60(t) {
		return
	}
	if err := refuseSharedIMTarget(imISOWriteURL()); err != nil {
		t.Fatal(err)
	}
	world := requireIMISO(t)
	if err := world.provision(t); err != nil {
		writeRemainArtifact(t, "reports/im-mongo-chains.json", map[string]any{"verdict": "BLOCKED", "reason": err.Error(), "host": collectHostIdentity()})
		t.Fatal(err)
	}
	matrix := []imISOClaim{world.chainAConsultClaim(t), world.chainBCrossRole(t), world.chainCConcurrencyRecovery(t)}
	for _, claim := range matrix {
		if claim.Verdict == "FAIL" {
			t.Fatalf("%s failed: %s", claim.Rule, claim.Evidence)
		}
		if claim.Verdict != "PASS" && claim.Verdict != "BLOCKED" {
			t.Fatalf("%s unexpected %s: %s", claim.Rule, claim.Verdict, claim.Evidence)
		}
	}
	writeRemainArtifact(t, "reports/im-mongo-chains.json", map[string]any{
		"verdict": "PARTIAL", "note": "live host-60 isolation observations; not a RULE approval",
		"target": imISOWriteURL(), "host": collectHostIdentity(), "chains": matrix,
	})
}

func (world *imISOWorld) chainAConsultClaim(t *testing.T) imISOClaim {
	t.Helper()
	fresh := requireIMISO(t)
	if err := fresh.provision(t); err != nil {
		return imClaim("A-consult-claim", "open → first → agent claim, live service-state", false, err.Error())
	}
	if err := fresh.openAndFirst(); err != nil {
		return imClaim("A-consult-claim", "open → first → agent claim, live service-state", false, err.Error())
	}
	claim, err := fresh.call("POST", fmt.Sprintf("/api/im/v2/support/queue/%d/claim", fresh.workItem), fresh.agentTok, map[string]any{"expected_version": fresh.workVer})
	if err != nil || asInt(claim["_http"]) != 200 {
		next, nextErr := fresh.call("POST", "/api/im/v2/support/queue/claim-next", fresh.agentTok, map[string]any{})
		if nextErr == nil && asInt(next["_http"]) == 200 {
			claim, err = next, nil
		}
	}
	st, stErr := fresh.serviceState()
	ok := err == nil && stErr == nil && asInt(claim["_http"]) == 200 && st.assigned == fresh.agentUID && st.assigned != 0
	return imClaim("A-consult-claim", "open → first → agent claim, live service-state", ok, fmt.Sprintf("http=%v assigned=%d agent=%d work=%d state_err=%v", claim["_http"], st.assigned, fresh.agentUID, st.workItem, stErr))
}

func (world *imISOWorld) chainBCrossRole(t *testing.T) imISOClaim {
	t.Helper()
	fresh := requireIMISO(t)
	if err := fresh.provision(t); err != nil {
		return imClaim("B-cross-role", "customer cannot claim; agent cannot open customer session", false, err.Error())
	}
	if err := fresh.openAndFirst(); err != nil {
		return imClaim("B-cross-role", "customer cannot claim; agent cannot open customer session", false, err.Error())
	}
	denied, err := fresh.call("POST", fmt.Sprintf("/api/im/v2/support/queue/%d/claim", fresh.workItem), fresh.customerTok, map[string]any{"expected_version": fresh.workVer})
	afterDeny, _ := fresh.serviceState()
	acl, aclErr := fresh.call("POST", "/api/im/v2/support/sessions", fresh.agentTok, map[string]any{"source_type": "novel", "shop_user_id": 1})
	ok := err == nil && asInt(denied["_http"]) != 200 && afterDeny.assigned == 0 && aclErr == nil && (asInt(acl["_http"]) == 401 || asInt(acl["_http"]) == 403)
	return imClaim("B-cross-role", "customer cannot claim; agent cannot open customer session", ok, fmt.Sprintf("claim_http=%v assigned=%d acl_http=%v", denied["_http"], afterDeny.assigned, acl["_http"]))
}

func (world *imISOWorld) chainCConcurrencyRecovery(t *testing.T) imISOClaim {
	t.Helper()
	fresh := requireIMISO(t)
	if err := fresh.provision(t); err != nil {
		return imClaim("C-concurrency-recovery", "two agents race claim; close restores live service-state", false, err.Error())
	}
	if err := fresh.openAndFirst(); err != nil {
		return imClaim("C-concurrency-recovery", "two agents race claim; close restores live service-state", false, err.Error())
	}
	type outcome struct {
		doc map[string]any
		err error
	}
	results := make([]outcome, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		results[0].doc, results[0].err = fresh.call("POST", fmt.Sprintf("/api/im/v2/support/queue/%d/claim", fresh.workItem), fresh.agentTok, map[string]any{"expected_version": fresh.workVer})
	}()
	go func() {
		defer workers.Done()
		results[1].doc, results[1].err = fresh.call("POST", fmt.Sprintf("/api/im/v2/support/queue/%d/claim", fresh.workItem), fresh.contenderTok, map[string]any{"expected_version": fresh.workVer})
	}()
	workers.Wait()
	st, stErr := fresh.serviceState()
	owner := st.assigned == fresh.agentUID || st.assigned == fresh.contenderUID
	unique := st.assigned != 0 && st.assigned != fresh.customerUID
	tok := fresh.agentTok
	if st.assigned == fresh.contenderUID {
		tok = fresh.contenderTok
	}
	closed, closeErr := fresh.call("POST", fmt.Sprintf("/api/im/v2/support/sessions/%d/close", fresh.sessionID), tok, map[string]any{"resolution_code": "resolved"})
	after, afterErr := fresh.serviceState()
	recovered := closeErr == nil && (asInt(closed["_http"]) == 200 || after.workItem == 0)
	ok := stErr == nil && afterErr == nil && owner && unique && recovered
	return imClaim("C-concurrency-recovery", "two agents race claim; close or live service-state clears the work item", ok, fmt.Sprintf("a_http=%v b_http=%v assigned=%d close_http=%v after_assigned=%d after_work=%d", results[0].doc["_http"], results[1].doc["_http"], st.assigned, closed["_http"], after.assigned, after.workItem))
}

func writeIMISOOracle(w http.ResponseWriter, world *imISOWorld, actor string) {
	source := "memory"
	world.mu.Lock()
	sid := world.sessionID
	world.mu.Unlock()
	if sid > 0 {
		if st, err := world.serviceState(); err == nil {
			world.mu.Lock()
			if world.sessionID == sid {
				world.assigned, world.workVer = st.assigned, st.workVer
				if st.workItem > 0 {
					world.workItem = st.workItem
				}
			}
			world.mu.Unlock()
			source = "service-state"
		}
	}
	world.mu.Lock()
	defer world.mu.Unlock()
	facts := map[string]any{
		"scope": imISOScope, "session_id": world.sessionID, "assigned": world.assigned,
		"records": 0, "customer_authenticated": false, "agent_authenticated": false,
		"agent_enabled": false, "agent_user_type_matches": false, "capacity_available": false,
		"work_item_claimable": false, "session_owned_by_customer": false,
		"source_type_valid": true, "user_type_matches_source": true, "shop_context_legal": true,
		"oracle_source": source,
	}
	if world.sessionID > 0 {
		facts["records"] = 1
	}
	switch actor {
	case "agent":
		facts["principal"], facts["role"] = "verify-agent", "AGENT"
		facts["agent_authenticated"], facts["agent_enabled"], facts["agent_user_type_matches"], facts["capacity_available"] = true, true, true, true
		facts["work_item_claimable"] = world.workItem > 0 && world.assigned == 0
	case "contender":
		facts["principal"], facts["role"] = "verify-contender", "CONTENDER"
		facts["agent_authenticated"], facts["agent_enabled"], facts["agent_user_type_matches"], facts["capacity_available"] = true, true, true, true
		facts["work_item_claimable"] = world.workItem > 0 && world.assigned == 0
	default:
		facts["principal"], facts["role"] = "verify-customer", "CUSTOMER"
		facts["customer_authenticated"], facts["session_owned_by_customer"] = true, true
	}
	switch {
	case world.assigned > 0:
		facts["state"] = "SESSION_ASSIGNED"
	case world.workItem > 0:
		facts["state"] = "WORK_ITEM_QUEUED"
	case world.sessionID > 0:
		facts["state"] = "SESSION_CURRENT"
	default:
		facts["state"] = "NO_SESSION"
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(facts)
}

func loadIMISOGraph(t *testing.T) Graph {
	t.Helper()
	root := imMongoRoot()
	if root == "" {
		t.Fatal("IM-mongo graph is required")
	}
	g, err := loadGraph(filepath.Join(root, imISOGraphRel))
	if err != nil {
		t.Fatal(err)
	}
	if g.ID != imISOGraphID {
		t.Fatalf("expected %s got %s", imISOGraphID, g.ID)
	}
	// Test-local Turso does not hold im-support-v1; lineage stays in the product catalog.
	g.Previous = ""
	return g
}

func prepareIMISOFixture(t *testing.T, graph Graph, target string) seatFixture {
	t.Helper()
	d := modelTestDB(t, graph)
	root := filepath.Join(t.TempDir(), "candidate")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "business.txt"), []byte("iso28886 IM-mongo live fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "business.txt"}, {"-c", "user.name=verify-iso", "-c", "user.email=iso@example.invalid", "commit", "-q", "-m", "iso28886"}} {
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		command.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1"}
		if body, err := command.CombinedOutput(); err != nil {
			t.Fatalf("fixture git: %v %s", err, body)
		}
	}
	commit, _, err := sourceFingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(t.TempDir(), "config.jsonc")
	if err := os.WriteFile(config, []byte(fmt.Sprintf(`{"environments":{"independent":{"approved":true,"url":%q}}}`, target)), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARC_IM_CUSTOMER", "customer")
	t.Setenv("ARC_IM_AGENT", "agent")
	t.Setenv("ARC_IM_CONTENDER", "contender")
	if _, err := d.importFixtures(Fixtures{Fixtures: []Fixture{
		{ID: "fixture.customer", Kind: "account", State: "ACTIVE", SecretRef: "env://ARC_IM_CUSTOMER", Refs: map[string]string{"role": "CUSTOMER", "principal": "verify-customer", "environment": "independent"}},
		{ID: "fixture.agent", Kind: "account", State: "ACTIVE", SecretRef: "env://ARC_IM_AGENT", Refs: map[string]string{"role": "AGENT", "principal": "verify-agent", "environment": "independent"}},
		{ID: "fixture.contender", Kind: "account", State: "ACTIVE", SecretRef: "env://ARC_IM_CONTENDER", Refs: map[string]string{"role": "CONTENDER", "principal": "verify-contender", "environment": "independent"}},
		{ID: "fixture.session", Kind: "business", State: "NO_SESSION", Refs: map[string]string{"environment": "independent", "scope": imISOScope}},
	}}); err != nil {
		t.Fatal(err)
	}
	return seatFixture{executionFixture: executionFixture{d: d, root: root, config: config, commit: commit, account: "fixture.customer", fixture: "fixture.session"}}
}

func openGraphRound(t *testing.T, fixture seatFixture, graph string) string {
	t.Helper()
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(cliJSON(t, "round", "--db", fixture.d.file, "--graph", graph, "--env", "independent", "--root", fixture.root, "--config", fixture.config, "--commit", fixture.commit), &result); err != nil || result.ID == "" {
		t.Fatalf("round: %v %+v", err, result)
	}
	return result.ID
}

func imISOExploreAdapter() ExploreAdapter {
	oracle := OracleAdapter{
		ID: "im-iso-http", Boundary: "after isolation HTTP and service-state readback",
		Call:      AdapterCall{Kind: adapterHTTP, Method: http.MethodGet, Path: "/oracle", SuccessStatuses: []int{200}, CredentialHeader: "X-Fixture-Actor"},
		StatePath: "/state", PrincipalPath: "/principal", RolePath: "/role", ScopePath: "/scope",
		Conditions: map[string]string{
			"customer_authenticated":    "/customer_authenticated",
			"agent_authenticated":       "/agent_authenticated",
			"agent_enabled":             "/agent_enabled",
			"agent_user_type_matches":   "/agent_user_type_matches",
			"capacity_available":        "/capacity_available",
			"work_item_claimable":       "/work_item_claimable",
			"session_owned_by_customer": "/session_owned_by_customer",
			"source_type_valid":         "/source_type_valid",
			"user_type_matches_source":  "/user_type_matches_source",
			"shop_context_legal":        "/shop_context_legal",
		},
		Measurements: map[string]string{"assigned": "/assigned", "records": "/records", "session_id": "/session_id"},
	}
	recovery := &FixtureRecovery{
		IsolationScope: imISOScope,
		Steps:          []RecoveryStep{{Actor: "CUSTOMER", Call: AdapterCall{Kind: adapterHTTP, Method: http.MethodPost, Path: "/reset", SuccessStatuses: []int{200}}}},
		Oracle:         oracle, State: "NO_SESSION",
	}
	return ExploreAdapter{
		SchemaVersion: 1, Graph: imISOGraphID, IsolationScope: imISOScope,
		TimeoutMS: 30000, StepTimeoutMS: 10000, Parallelism: 2,
		Oracle: oracle, Recovery: recovery,
		Accounts: map[string]string{"primary": "fixture.customer", "CUSTOMER": "fixture.customer", "AGENT": "fixture.agent", "CONTENDER": "fixture.contender"},
		Entries: []ExploreEntry{
			{Action: "open_session", Entry: "/open", Method: "POST", Roles: []string{"CUSTOMER"}},
			{Action: "send_customer_first", Entry: "/first", Method: "POST", Roles: []string{"CUSTOMER"}},
			{Action: "claim_work_item", Entry: "/claim", Method: "POST", Roles: []string{"AGENT", "CONTENDER"}},
			{Action: "claim_work_item", Entry: "/force", Method: "POST", Roles: []string{"CUSTOMER"}},
		},
	}
}

func imISOExploreReq(fixture seatFixture, mode string, planner ExplorePlanner, seed int) ExploreRequest {
	return ExploreRequest{
		Graph: imISOGraphID, Flow: "flow.intake", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: imISOExploreAdapter(),
		Mode: mode, Planner: planner,
		Budget: ExploreRuntimeBudget{MaxStates: 8, MaxPaths: 8, MaxActions: 5, Deadline: 60 * time.Second, Seed: seed},
	}
}

func httpStatus(t *testing.T, raw string) int {
	t.Helper()
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(raw)
	if err != nil {
		return 0
	}
	resp.Body.Close()
	return resp.StatusCode
}

func dataOf(doc map[string]any) map[string]any {
	if data, ok := doc["data"].(map[string]any); ok {
		return data
	}
	return doc
}

func pickString(doc map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := doc[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func pickInt(doc map[string]any, keys ...string) int {
	for _, key := range keys {
		if n := asInt(doc[key]); n > 0 {
			return n
		}
	}
	return 0
}

func bizOK(doc map[string]any) bool {
	if asInt(doc["_http"]) != 200 {
		return false
	}
	code := asInt(doc["code"])
	return code == 0 || code == 200
}

func uuidV7() string {
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	ms := uint64(time.Now().UnixMilli())
	raw[0] = byte(ms >> 40)
	raw[1] = byte(ms >> 32)
	raw[2] = byte(ms >> 24)
	raw[3] = byte(ms >> 16)
	raw[4] = byte(ms >> 8)
	raw[5] = byte(ms)
	raw[6] = (raw[6] & 0x0f) | 0x70
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
}

func asInt(value any) int {
	switch n := value.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		v, _ := n.Int64()
		return int(v)
	case string:
		v, _ := strconv.Atoi(n)
		return v
	default:
		return 0
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func passwordEnvelope(plain string) string {
	window := strconv.FormatInt(time.Now().UnixMilli()/30000, 10)
	sum := sha256.Sum256([]byte(window))
	nonce := make([]byte, 12)
	_, _ = rand.Read(nonce)
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return ""
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}
	ct := aead.Seal(nil, nonce, []byte(plain+"@"+window), nil)
	return base64.StdEncoding.EncodeToString(append(nonce, ct...))
}

func mintUserJWT(secret string, userID int) string {
	now := time.Now().Unix()
	return mintHS256(secret, map[string]any{
		"guard": "api", "version": "1.0", "jti": strconv.Itoa(userID), "sub": strconv.Itoa(userID),
		"iat": now, "nbf": now - 5, "exp": now + 7200, "iss": "gva.novel.shop.login",
		"aud": []string{"novel"}, "token_use": "access",
	})
}

func mintAgentJWT(secret string, userID int) string {
	now := time.Now().Unix()
	return mintHS256(secret, map[string]any{
		"guard": "api", "version": "1.0", "principal_type": "support_agent", "user_id": userID,
		"auth_epoch": 1, "token_use": "access", "token_version": "iso-arc",
		"sub": "support_agent:" + strconv.Itoa(userID), "jti": strconv.Itoa(userID),
		"iat": now, "nbf": now - 5, "exp": now + 7200,
	})
}

func mintHS256(secret string, claims map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, _ := json.Marshal(claims)
	body := header + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
