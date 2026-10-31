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
	imISOBase     = "http://192.168.1.50:28886"
	imSharedBase  = "http://192.168.1.50:28885"
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
	if err := refuseSharedIMTarget(imISOBase); err != nil {
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
	shared := httpStatus(t, imSharedBase+"/api/im/health/live")
	if shared != 200 {
		t.Fatalf("shared 28885 must stay healthy after isolation writes: %d", shared)
	}
	report := map[string]any{
		"env": imIsolationEnv, "target": imISOBase, "shared": imSharedBase,
		"human_live": world.humanLive, "iso_live": world.isoLive, "writer_ok": world.writerOK,
		"customer_uid": world.customerUID, "session_id": world.sessionID, "assigned": world.assigned,
		"known_rules":            claims,
		"deterministic":          map[string]any{"campaign": det.Campaign, "actions": det.Actions, "stop": det.StopReason, "unspecified": len(det.Unspecified)},
		"heuristic":              map[string]any{"campaign": heur.Campaign, "actions": heur.Actions, "unspecified": len(heur.Unspecified)},
		"ai":                     map[string]any{"campaign": assisted.Campaign, "actions": assisted.Actions, "planner_calls": planner.Calls},
		"not":                    []string{"lan50-write", "28885-write", "28895", "production"},
		"binary_mutation_deploy": "NOT_RUN: living isolation writer was not restarted",
	}
	writeExploreArtifact(t, "reports/im-mongo-iso28886.json", report)
	writeIMISOArtifact(t, "reports/im-mongo-iso28886.json", report)
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
	isoLive := httpStatus(t, imISOBase+"/api/im/health/live")
	isoReady := httpStatus(t, imISOBase+"/api/im/health/ready")
	humanLive := httpStatus(t, imSharedBase+"/api/im/health/live")
	if isoLive != 200 || isoReady != 200 {
		writeExploreArtifact(t, "reports/im-mongo-iso28886-blocked.json", map[string]any{
			"verdict": "BLOCKED", "reason": "iso28886 health is not 200", "iso_live": isoLive, "iso_ready": isoReady,
		})
		t.Fatalf("iso28886 is the authorized writer but is not healthy: live=%d ready=%d", isoLive, isoReady)
	}
	if humanLive != 200 {
		t.Fatalf("shared 28885 live probe failed: %d", humanLive)
	}
	world := &imISOWorld{isoLive: isoLive, humanLive: humanLive, writerOK: isolationWriterOK(t)}
	world.loadSecrets(t)
	return world
}

func isolationWriterOK(t *testing.T) bool {
	t.Helper()
	cmd := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=8", "nulltech@192.168.1.50",
		"grep -E 'port=28886|28886' /home/nulltech/iluwen/chensi-im-t47-ops01-20260913t092552z/run/im-server.identity | head -5")
	body, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("isolation identity ssh failed; HTTP health still required")
		return false
	}
	text := string(body)
	if strings.Contains(text, "port=28885") && !strings.Contains(text, "port=28886") {
		t.Fatal("isolation identity pointed at shared 28885")
	}
	return strings.Contains(text, "28886")
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
	script := `
set -e
ids=$(docker exec im-t47-ops01-20260913t092552z-pg psql -U im_t47_ops01 -d im_t47_ops01_20260913_092552 -tA -c "SELECT id FROM im_users WHERE user_type=1 AND status=1 AND deleted_at IS NULL AND length(mobile)=11 ORDER BY id DESC;")
active=$(/home/nulltech/iluwen/deps/mongosh/bin/mongosh --quiet "mongodb://127.0.0.1:27018/?directConnection=true&replicaSet=t47ops01rs" --eval 'db=db.getSiblingDB("im_t47_ops01_20260913_092552"); print(db.im_support_sessions.find({status:"active"}).toArray().map(function(s){return String(s.user_id)}).join(" "));')
for id in $ids; do
  echo " $active " | grep -q " $id " && continue
  echo "$id"
  exit 0
done
exit 2
`
	cmd := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=12", "nulltech@192.168.1.50", "bash", "-s")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("unused isolation customer: %w", err)
	}
	id, err := parseFirstPositiveID(string(out))
	if err != nil {
		return 0, fmt.Errorf("unused isolation customer missing")
	}
	return id, nil
}

func createIsolationCustomer() (int, error) {
	var last error
	for attempt := 0; attempt < 5; attempt++ {
		mobile := fmt.Sprintf("170%08d", time.Now().UnixNano()%100000000)
		script := fmt.Sprintf(`
set -e
docker exec im-t47-ops01-20260913t092552z-pg psql -U im_t47_ops01 -d im_t47_ops01_20260913_092552 -v ON_ERROR_STOP=1 -tA -c "INSERT INTO im_users (mobile, nickname, avatar, gender, motto, email, birthday, robot, merchant, open_id, is_shop, is_customer_service, user_type, inviter_id, created_at, updated_at, custom_status, account_status, status) VALUES ('%s', 'iso-arc-cust', '', 0, 'iso-arc', '', '', 0, 0, '', false, 0, 1, 0, now(), now(), '', 0, 1) RETURNING id;"
`, mobile)
		cmd := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=12", "nulltech@192.168.1.50", "bash", "-s")
		cmd.Stdin = strings.NewReader(script)
		out, err := cmd.CombinedOutput()
		if err != nil {
			last = fmt.Errorf("create isolation customer: %w: %s", err, strings.TrimSpace(string(out)))
			time.Sleep(20 * time.Millisecond)
			continue
		}
		id, err := parseFirstPositiveID(string(out))
		if err != nil {
			last = fmt.Errorf("create isolation customer: missing id: %s", strings.TrimSpace(string(out)))
			continue
		}
		return id, nil
	}
	if last == nil {
		last = fmt.Errorf("create isolation customer: exhausted retries")
	}
	return 0, last
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
	if err := refuseSharedIMTarget(imISOBase); err != nil {
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
	req, err := http.NewRequest(method, imISOBase+path, bytes.NewReader(payload))
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

func writeIMISOOracle(w http.ResponseWriter, world *imISOWorld, actor string) {
	world.mu.Lock()
	defer world.mu.Unlock()
	facts := map[string]any{
		"scope": imISOScope, "session_id": world.sessionID, "assigned": world.assigned,
		"records": 0, "customer_authenticated": false, "agent_authenticated": false,
		"agent_enabled": false, "agent_user_type_matches": false, "capacity_available": false,
		"work_item_claimable": false, "session_owned_by_customer": false,
		"source_type_valid": true, "user_type_matches_source": true, "shop_context_legal": true,
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
	resp, err := http.Get(raw)
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
