package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"berkut-scc/api/handlers"
	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
	"berkut-scc/tasks"
	taskstore "berkut-scc/tasks/store"
)

type controlsTestEnv struct {
	ctx       context.Context
	cfg       *config.AppConfig
	cs        store.ControlsStore
	links     store.EntityLinksStore
	us        store.UsersStore
	ds        store.DocsStore
	is        store.IncidentsStore
	ts        tasks.Store
	audits    store.AuditStore
	handler   *handlers.ControlsHandler
	adminUser *store.User
	viewUser  *store.User
	cleanup   func()
}

func setupControls(t *testing.T) controlsTestEnv {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "controls.db")}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cs := store.NewControlsStore(db)
	links := store.NewEntityLinksStore(db)
	us := store.NewUsersStore(db)
	ds := store.NewDocsStore(db)
	is := store.NewIncidentsStore(db)
	ts := taskstore.NewStore(db)
	audits := store.NewAuditStore(db)
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	handler := handlers.NewControlsHandler(cs, links, us, ds, is, ts, audits, policy, logger)
	admin := &store.User{
		Username:     "admin_controls",
		FullName:     "Admin",
		Department:   "Sec",
		Position:     "Lead",
		PasswordHash: "hash",
		Salt:         "salt",
		PasswordSet:  true,
		Active:       true,
	}
	adminID, err := us.Create(context.Background(), admin, []string{"admin"})
	if err != nil {
		t.Fatalf("admin create: %v", err)
	}
	admin.ID = adminID
	viewer := &store.User{
		Username:     "viewer_controls",
		FullName:     "Viewer",
		Department:   "Sec",
		Position:     "Analyst",
		PasswordHash: "hash",
		Salt:         "salt",
		PasswordSet:  true,
		Active:       true,
	}
	viewID, err := us.Create(context.Background(), viewer, []string{"analyst"})
	if err != nil {
		t.Fatalf("viewer create: %v", err)
	}
	viewer.ID = viewID
	return controlsTestEnv{
		ctx:       context.Background(),
		cfg:       cfg,
		cs:        cs,
		links:     links,
		us:        us,
		ds:        ds,
		is:        is,
		ts:        ts,
		audits:    audits,
		handler:   handler,
		adminUser: admin,
		viewUser:  viewer,
		cleanup: func() {
			db.Close()
		},
	}
}

func sessionFor(u *store.User, roles []string) *store.SessionRecord {
	return &store.SessionRecord{
		UserID:   u.ID,
		Username: u.Username,
		Roles:    roles,
	}
}

func TestControlsPermissionDenied(t *testing.T) {
	env := setupControls(t)
	defer env.cleanup()
	payload := map[string]any{
		"code":             "CTRL-TEST-001",
		"title":            "Test Control",
		"control_type":     "technical",
		"domain":           "infra",
		"review_frequency": "annual",
		"status":           "implemented",
		"risk_level":       "medium",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/controls", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, sessionFor(env.viewUser, []string{"analyst"})))
	rr := httptest.NewRecorder()
	env.handler.CreateControl(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestControlsCRUDAndFilters(t *testing.T) {
	env := setupControls(t)
	defer env.cleanup()
	payload := map[string]any{
		"code":             "CTRL-ALPHA-001",
		"title":            "Alpha Control",
		"control_type":     "technical",
		"domain":           "infra",
		"review_frequency": "annual",
		"status":           "implemented",
		"risk_level":       "medium",
		"tags":             []string{"TAG1"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/controls", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	rr := httptest.NewRecorder()
	env.handler.CreateControl(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status: %d", rr.Code)
	}
	var created store.Control
	if err := json.NewDecoder(rr.Body).Decode(&created); err != nil || created.ID == 0 {
		t.Fatalf("decode created control: %v", err)
	}
	update := map[string]any{
		"code":             "CTRL-ALPHA-001",
		"title":            "Alpha Control Updated",
		"control_type":     "technical",
		"domain":           "infra",
		"review_frequency": "annual",
		"status":           "partial",
		"risk_level":       "high",
	}
	upBody, _ := json.Marshal(update)
	upReq := httptest.NewRequest("PUT", "/api/controls/"+strconv.FormatInt(created.ID, 10), bytes.NewReader(upBody))
	upReq = withURLParams(upReq, map[string]string{"id": strconv.FormatInt(created.ID, 10)})
	upReq = upReq.WithContext(context.WithValue(upReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	upRR := httptest.NewRecorder()
	env.handler.UpdateControl(upRR, upReq)
	if upRR.Code != http.StatusOK {
		t.Fatalf("update status: %d", upRR.Code)
	}
	listReq := httptest.NewRequest("GET", "/api/controls?q=alpha", nil)
	listReq = listReq.WithContext(context.WithValue(listReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	listRR := httptest.NewRecorder()
	env.handler.ListControls(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("list status: %d", listRR.Code)
	}
	var listResp struct {
		Items []store.Control `json:"items"`
	}
	if err := json.Unmarshal(listRR.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("list decode: %v", err)
	}
	if len(listResp.Items) != 1 || listResp.Items[0].ID != created.ID {
		t.Fatalf("expected 1 filtered control")
	}
	delReq := httptest.NewRequest("DELETE", "/api/controls/"+strconv.FormatInt(created.ID, 10), nil)
	delReq = withURLParams(delReq, map[string]string{"id": strconv.FormatInt(created.ID, 10)})
	delReq = delReq.WithContext(context.WithValue(delReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	delRR := httptest.NewRecorder()
	env.handler.DeleteControl(delRR, delReq)
	if delRR.Code != http.StatusOK {
		t.Fatalf("delete status: %d", delRR.Code)
	}
}

func TestControlsUniqueCodeConstraint(t *testing.T) {
	env := setupControls(t)
	defer env.cleanup()
	payload := map[string]any{
		"code":             "CTRL-UNIQ-001",
		"title":            "Unique Control",
		"control_type":     "technical",
		"domain":           "infra",
		"review_frequency": "annual",
		"status":           "implemented",
		"risk_level":       "medium",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/controls", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	rr := httptest.NewRecorder()
	env.handler.CreateControl(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status: %d", rr.Code)
	}
	req2 := httptest.NewRequest("POST", "/api/controls", bytes.NewReader(body))
	req2 = req2.WithContext(context.WithValue(req2.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	rr2 := httptest.NewRecorder()
	env.handler.CreateControl(rr2, req2)
	if rr2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for duplicate code, got %d", rr2.Code)
	}
}

func TestControlsChecksCRUD(t *testing.T) {
	env := setupControls(t)
	defer env.cleanup()
	ctrl := map[string]any{
		"code":             "CTRL-CHECK-001",
		"title":            "Check Control",
		"control_type":     "technical",
		"domain":           "infra",
		"review_frequency": "annual",
		"status":           "implemented",
		"risk_level":       "medium",
	}
	ctrlBody, _ := json.Marshal(ctrl)
	ctrlReq := httptest.NewRequest("POST", "/api/controls", bytes.NewReader(ctrlBody))
	ctrlReq = ctrlReq.WithContext(context.WithValue(ctrlReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	ctrlRR := httptest.NewRecorder()
	env.handler.CreateControl(ctrlRR, ctrlReq)
	if ctrlRR.Code != http.StatusCreated {
		t.Fatalf("create control: %d", ctrlRR.Code)
	}
	var created store.Control
	_ = json.Unmarshal(ctrlRR.Body.Bytes(), &created)
	checkPayload := map[string]any{
		"result":   "pass",
		"notes_md": "ok",
	}
	checkBody, _ := json.Marshal(checkPayload)
	checkReq := httptest.NewRequest("POST", "/api/controls/"+strconv.FormatInt(created.ID, 10)+"/checks", bytes.NewReader(checkBody))
	checkReq = withURLParams(checkReq, map[string]string{"id": strconv.FormatInt(created.ID, 10)})
	checkReq = checkReq.WithContext(context.WithValue(checkReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	checkRR := httptest.NewRecorder()
	env.handler.CreateControlCheck(checkRR, checkReq)
	if checkRR.Code != http.StatusCreated {
		t.Fatalf("create check: %d", checkRR.Code)
	}
	var check store.ControlCheck
	_ = json.Unmarshal(checkRR.Body.Bytes(), &check)
	listReq := httptest.NewRequest("GET", "/api/checks", nil)
	listReq = listReq.WithContext(context.WithValue(listReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	listRR := httptest.NewRecorder()
	env.handler.ListChecks(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("list checks: %d", listRR.Code)
	}
	delReq := httptest.NewRequest("DELETE", "/api/checks/"+strconv.FormatInt(check.ID, 10), nil)
	delReq = withURLParams(delReq, map[string]string{"id": strconv.FormatInt(check.ID, 10)})
	delReq = delReq.WithContext(context.WithValue(delReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	delRR := httptest.NewRecorder()
	env.handler.DeleteCheck(delRR, delReq)
	if delRR.Code != http.StatusOK {
		t.Fatalf("delete check: %d", delRR.Code)
	}
}

func TestControlsLinksCRUD(t *testing.T) {
	env := setupControls(t)
	defer env.cleanup()

	ctrlPayload := map[string]any{
		"code":             "CTRL-LINK-001",
		"title":            "Link Control",
		"control_type":     "technical",
		"domain":           "infra",
		"review_frequency": "annual",
		"status":           "implemented",
		"risk_level":       "medium",
	}
	ctrlBody, _ := json.Marshal(ctrlPayload)
	ctrlReq := httptest.NewRequest("POST", "/api/controls", bytes.NewReader(ctrlBody))
	ctrlReq = ctrlReq.WithContext(context.WithValue(ctrlReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	ctrlRR := httptest.NewRecorder()
	env.handler.CreateControl(ctrlRR, ctrlReq)
	if ctrlRR.Code != http.StatusCreated {
		t.Fatalf("create control: %d", ctrlRR.Code)
	}
	var ctrl store.Control
	_ = json.Unmarshal(ctrlRR.Body.Bytes(), &ctrl)

	doc := &store.Document{
		Title:                 "Evidence Doc",
		Status:                "draft",
		ClassificationLevel:   1,
		ClassificationTags:    []string{},
		RegNumber:             "DOC-CTRL-1",
		InheritACL:            false,
		InheritClassification: false,
		CreatedBy:             env.adminUser.ID,
		CurrentVersion:        1,
	}
	docID, err := env.ds.CreateDocument(env.ctx, doc, []store.ACLRule{}, "", false)
	if err != nil || docID == 0 {
		t.Fatalf("doc create: %v", err)
	}

	incident := &store.Incident{
		RegNo:       "INC-CTRL-1",
		Title:       "Link Incident",
		Severity:    "low",
		Status:      "draft",
		OwnerUserID: env.adminUser.ID,
		CreatedBy:   env.adminUser.ID,
		UpdatedBy:   env.adminUser.ID,
	}
	incID, err := env.is.CreateIncident(env.ctx, incident, []store.IncidentParticipant{}, []store.ACLRule{}, "INC-{seq}")
	if err != nil || incID == 0 {
		t.Fatalf("incident create: %v", err)
	}

	linkPayload := map[string]any{
		"target_type":   "doc",
		"target_id":     strconv.FormatInt(docID, 10),
		"relation_type": "evidence",
	}
	linkBody, _ := json.Marshal(linkPayload)
	linkReq := httptest.NewRequest("POST", "/api/controls/"+strconv.FormatInt(ctrl.ID, 10)+"/links", bytes.NewReader(linkBody))
	linkReq = withURLParams(linkReq, map[string]string{"id": strconv.FormatInt(ctrl.ID, 10)})
	linkReq = linkReq.WithContext(context.WithValue(linkReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	linkRR := httptest.NewRecorder()
	env.handler.CreateControlLink(linkRR, linkReq)
	if linkRR.Code != http.StatusCreated {
		t.Fatalf("create link: %d", linkRR.Code)
	}

	listReq := httptest.NewRequest("GET", "/api/controls/"+strconv.FormatInt(ctrl.ID, 10)+"/links", nil)
	listReq = withURLParams(listReq, map[string]string{"id": strconv.FormatInt(ctrl.ID, 10)})
	listReq = listReq.WithContext(context.WithValue(listReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	listRR := httptest.NewRecorder()
	env.handler.ListControlLinks(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("list links: %d", listRR.Code)
	}
	var listResp struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal(listRR.Body.Bytes(), &listResp)
	if len(listResp.Items) == 0 {
		t.Fatalf("expected links")
	}

	delReq := httptest.NewRequest("DELETE", "/api/controls/"+strconv.FormatInt(ctrl.ID, 10)+"/links/1", nil)
	delReq = withURLParams(delReq, map[string]string{"id": strconv.FormatInt(ctrl.ID, 10), "link_id": "1"})
	delReq = delReq.WithContext(context.WithValue(delReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	delRR := httptest.NewRecorder()
	linkID := fmt.Sprintf("%v", listResp.Items[0]["id"])
	if linkID != "" {
		delReq = withURLParams(delReq, map[string]string{"id": strconv.FormatInt(ctrl.ID, 10), "link_id": linkID})
	}
	env.handler.DeleteControlLink(delRR, delReq)
	if delRR.Code != http.StatusOK {
		t.Fatalf("delete link: %d", delRR.Code)
	}

	incidentLinkPayload := map[string]any{
		"target_type":   "incident",
		"target_id":     strconv.FormatInt(incID, 10),
		"relation_type": "violates",
	}
	incBody, _ := json.Marshal(incidentLinkPayload)
	incReq := httptest.NewRequest("POST", "/api/controls/"+strconv.FormatInt(ctrl.ID, 10)+"/links", bytes.NewReader(incBody))
	incReq = withURLParams(incReq, map[string]string{"id": strconv.FormatInt(ctrl.ID, 10)})
	incReq = incReq.WithContext(context.WithValue(incReq.Context(), auth.SessionContextKey, sessionFor(env.viewUser, []string{"analyst"})))
	incRR := httptest.NewRecorder()
	env.handler.CreateControlLink(incRR, incReq)
	if incRR.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for viewer, got %d", incRR.Code)
	}
}

func TestControlsAutoViolationFromIncidentLink(t *testing.T) {
	env := setupControls(t)
	defer env.cleanup()

	ctrlPayload := map[string]any{
		"code":             "CTRL-AUTO-001",
		"title":            "Auto Control",
		"control_type":     "technical",
		"domain":           "infra",
		"review_frequency": "annual",
		"status":           "implemented",
		"risk_level":       "medium",
	}
	ctrlBody, _ := json.Marshal(ctrlPayload)
	ctrlReq := httptest.NewRequest("POST", "/api/controls", bytes.NewReader(ctrlBody))
	ctrlReq = ctrlReq.WithContext(context.WithValue(ctrlReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	ctrlRR := httptest.NewRecorder()
	env.handler.CreateControl(ctrlRR, ctrlReq)
	if ctrlRR.Code != http.StatusCreated {
		t.Fatalf("create control: %d", ctrlRR.Code)
	}
	var ctrl store.Control
	_ = json.Unmarshal(ctrlRR.Body.Bytes(), &ctrl)

	incident := &store.Incident{
		RegNo:       "INC-AUTO-1",
		Title:       "Incident Auto",
		Description: "Impact details",
		Severity:    "high",
		Status:      "draft",
		OwnerUserID: env.adminUser.ID,
		CreatedBy:   env.adminUser.ID,
		UpdatedBy:   env.adminUser.ID,
	}
	incID, err := env.is.CreateIncident(env.ctx, incident, []store.IncidentParticipant{}, []store.ACLRule{}, "INC-{seq}")
	if err != nil || incID == 0 {
		t.Fatalf("incident create: %v", err)
	}

	linkPayload := map[string]any{
		"target_type":   "incident",
		"target_id":     strconv.FormatInt(incID, 10),
		"relation_type": "violates",
	}
	linkBody, _ := json.Marshal(linkPayload)
	linkReq := httptest.NewRequest("POST", "/api/controls/"+strconv.FormatInt(ctrl.ID, 10)+"/links", bytes.NewReader(linkBody))
	linkReq = withURLParams(linkReq, map[string]string{"id": strconv.FormatInt(ctrl.ID, 10)})
	linkReq = linkReq.WithContext(context.WithValue(linkReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	linkRR := httptest.NewRecorder()
	env.handler.CreateControlLink(linkRR, linkReq)
	if linkRR.Code != http.StatusCreated {
		t.Fatalf("create control link: %d", linkRR.Code)
	}
	viol, err := env.cs.GetControlViolationByLink(env.ctx, ctrl.ID, incID)
	if err != nil || viol == nil {
		t.Fatalf("auto violation missing: %v", err)
	}
	if !viol.IsAuto || !viol.IsActive {
		t.Fatalf("expected auto active violation")
	}

	// Repeat to ensure no duplicates.
	linkRR2 := httptest.NewRecorder()
	linkReq2 := httptest.NewRequest("POST", "/api/controls/"+strconv.FormatInt(ctrl.ID, 10)+"/links", bytes.NewReader(linkBody))
	linkReq2 = withURLParams(linkReq2, map[string]string{"id": strconv.FormatInt(ctrl.ID, 10)})
	linkReq2 = linkReq2.WithContext(context.WithValue(linkReq2.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	env.handler.CreateControlLink(linkRR2, linkReq2)
	if linkRR2.Code != http.StatusCreated {
		t.Fatalf("repeat link: %d", linkRR2.Code)
	}
	filter := store.ControlViolationFilter{ControlID: ctrl.ID, IncidentID: &incID}
	list, err := env.cs.ListViolations(env.ctx, filter)
	if err != nil {
		t.Fatalf("list violations: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(list))
	}

	linksResp := httptest.NewRecorder()
	listReq := httptest.NewRequest("GET", "/api/controls/"+strconv.FormatInt(ctrl.ID, 10)+"/links", nil)
	listReq = withURLParams(listReq, map[string]string{"id": strconv.FormatInt(ctrl.ID, 10)})
	listReq = listReq.WithContext(context.WithValue(listReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	env.handler.ListControlLinks(linksResp, listReq)
	var linkList struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.Unmarshal(linksResp.Body.Bytes(), &linkList)
	if len(linkList.Items) == 0 {
		t.Fatalf("expected control links")
	}
	for _, item := range linkList.Items {
		linkID := fmt.Sprintf("%v", item["id"])
		delReq := httptest.NewRequest("DELETE", "/api/controls/"+strconv.FormatInt(ctrl.ID, 10)+"/links/"+linkID, nil)
		delReq = withURLParams(delReq, map[string]string{"id": strconv.FormatInt(ctrl.ID, 10), "link_id": linkID})
		delReq = delReq.WithContext(context.WithValue(delReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
		delRR := httptest.NewRecorder()
		env.handler.DeleteControlLink(delRR, delReq)
		if delRR.Code != http.StatusOK {
			t.Fatalf("delete link: %d", delRR.Code)
		}
	}
	after, err := env.cs.GetControlViolationByLink(env.ctx, ctrl.ID, incID)
	if err != nil || after == nil {
		t.Fatalf("violation missing after delete: %v", err)
	}
	if after.IsActive {
		t.Fatalf("expected violation to be inactive")
	}
}

func TestControlsAutoViolationPermission(t *testing.T) {
	env := setupControls(t)
	defer env.cleanup()

	ctrlPayload := map[string]any{
		"code":             "CTRL-AUTO-002",
		"title":            "Auto Control NoPerm",
		"control_type":     "technical",
		"domain":           "infra",
		"review_frequency": "annual",
		"status":           "implemented",
		"risk_level":       "medium",
	}
	ctrlBody, _ := json.Marshal(ctrlPayload)
	ctrlReq := httptest.NewRequest("POST", "/api/controls", bytes.NewReader(ctrlBody))
	ctrlReq = ctrlReq.WithContext(context.WithValue(ctrlReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	ctrlRR := httptest.NewRecorder()
	env.handler.CreateControl(ctrlRR, ctrlReq)
	if ctrlRR.Code != http.StatusCreated {
		t.Fatalf("create control: %d", ctrlRR.Code)
	}
	var ctrl store.Control
	_ = json.Unmarshal(ctrlRR.Body.Bytes(), &ctrl)

	incident := &store.Incident{
		RegNo:       "INC-AUTO-2",
		Title:       "Incident Auto 2",
		Severity:    "low",
		Status:      "draft",
		OwnerUserID: env.adminUser.ID,
		CreatedBy:   env.adminUser.ID,
		UpdatedBy:   env.adminUser.ID,
	}
	incID, err := env.is.CreateIncident(env.ctx, incident, []store.IncidentParticipant{}, []store.ACLRule{}, "INC-{seq}")
	if err != nil || incID == 0 {
		t.Fatalf("incident create: %v", err)
	}

	customPolicy := rbac.NewPolicy([]rbac.Role{{
		Name:        "controls_editor",
		Permissions: []rbac.Permission{"controls.manage"},
	}})
	customHandler := handlers.NewControlsHandler(env.cs, env.links, env.us, env.ds, env.is, env.ts, env.audits, customPolicy, utils.NewLogger())
	linkPayload := map[string]any{
		"target_type":   "incident",
		"target_id":     strconv.FormatInt(incID, 10),
		"relation_type": "violates",
	}
	linkBody, _ := json.Marshal(linkPayload)
	linkReq := httptest.NewRequest("POST", "/api/controls/"+strconv.FormatInt(ctrl.ID, 10)+"/links", bytes.NewReader(linkBody))
	linkReq = withURLParams(linkReq, map[string]string{"id": strconv.FormatInt(ctrl.ID, 10)})
	linkReq = linkReq.WithContext(context.WithValue(linkReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"controls_editor"})))
	linkRR := httptest.NewRecorder()
	customHandler.CreateControlLink(linkRR, linkReq)
	if linkRR.Code != http.StatusCreated {
		t.Fatalf("create control link: %d", linkRR.Code)
	}
	viol, err := env.cs.GetControlViolationByLink(env.ctx, ctrl.ID, incID)
	if err != nil {
		t.Fatalf("fetch violation: %v", err)
	}
	if viol != nil {
		t.Fatalf("violation should not be created without permission")
	}
}

func TestControlsViolationsCRUD(t *testing.T) {
	env := setupControls(t)
	defer env.cleanup()
	ctrl := map[string]any{
		"code":             "CTRL-VIOL-001",
		"title":            "Violation Control",
		"control_type":     "procedural",
		"domain":           "other",
		"review_frequency": "annual",
		"status":           "partial",
		"risk_level":       "high",
	}
	ctrlBody, _ := json.Marshal(ctrl)
	ctrlReq := httptest.NewRequest("POST", "/api/controls", bytes.NewReader(ctrlBody))
	ctrlReq = ctrlReq.WithContext(context.WithValue(ctrlReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	ctrlRR := httptest.NewRecorder()
	env.handler.CreateControl(ctrlRR, ctrlReq)
	if ctrlRR.Code != http.StatusCreated {
		t.Fatalf("create control: %d", ctrlRR.Code)
	}
	var created store.Control
	_ = json.Unmarshal(ctrlRR.Body.Bytes(), &created)
	violPayload := map[string]any{
		"severity": "high",
		"summary":  "manual violation",
	}
	violBody, _ := json.Marshal(violPayload)
	violReq := httptest.NewRequest("POST", "/api/controls/"+strconv.FormatInt(created.ID, 10)+"/violations", bytes.NewReader(violBody))
	violReq = withURLParams(violReq, map[string]string{"id": strconv.FormatInt(created.ID, 10)})
	violReq = violReq.WithContext(context.WithValue(violReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	violRR := httptest.NewRecorder()
	env.handler.CreateControlViolation(violRR, violReq)
	if violRR.Code != http.StatusCreated {
		t.Fatalf("create violation: %d", violRR.Code)
	}
	var viol store.ControlViolation
	_ = json.Unmarshal(violRR.Body.Bytes(), &viol)
	listReq := httptest.NewRequest("GET", "/api/violations", nil)
	listReq = listReq.WithContext(context.WithValue(listReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	listRR := httptest.NewRecorder()
	env.handler.ListViolations(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("list violations: %d", listRR.Code)
	}
	delReq := httptest.NewRequest("DELETE", "/api/violations/"+strconv.FormatInt(viol.ID, 10), nil)
	delReq = withURLParams(delReq, map[string]string{"id": strconv.FormatInt(viol.ID, 10)})
	delReq = delReq.WithContext(context.WithValue(delReq.Context(), auth.SessionContextKey, sessionFor(env.adminUser, []string{"admin"})))
	delRR := httptest.NewRecorder()
	env.handler.DeleteViolation(delRR, delReq)
	if delRR.Code != http.StatusOK {
		t.Fatalf("delete violation: %d", delRR.Code)
	}
}
