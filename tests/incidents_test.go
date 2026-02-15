package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"berkut-scc/api/handlers"
	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/docs"
	"berkut-scc/core/incidents"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func setupIncidents(t *testing.T) (context.Context, *config.AppConfig, *store.User, store.IncidentsStore, store.DocsStore, store.UsersStore, *incidents.Service, *docs.Service, func()) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.AppConfig{
		DBPath: filepath.Join(dir, "test.db"),
		Incidents: config.IncidentsConfig{
			RegNoFormat: "INC-{year}-{seq:05}",
			StorageDir:  filepath.Join(dir, "incidents"),
		},
		Docs: config.DocsConfig{
			EncryptionKey: "0123456789abcdef0123456789abcdef",
			StorageDir:    filepath.Join(dir, "docs"),
		},
	}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	us := store.NewUsersStore(db)
	ds := store.NewDocsStore(db)
	is := store.NewIncidentsStore(db)
	audits := store.NewAuditStore(db)
	svc, err := incidents.NewService(cfg, audits)
	if err != nil {
		t.Fatalf("incidents service: %v", err)
	}
	docsSvc, err := docs.NewService(cfg, ds, us, audits, logger)
	if err != nil {
		t.Fatalf("docs service: %v", err)
	}
	u := &store.User{
		Username:       "user1",
		FullName:       "User One",
		Department:     "Sec",
		Position:       "Analyst",
		ClearanceLevel: int(docs.ClassificationInternal),
		PasswordHash:   "hash",
		Salt:           "salt",
		PasswordSet:    true,
		Active:         true,
	}
	uid, err := us.Create(context.Background(), u, []string{"security_officer"})
	if err != nil {
		t.Fatalf("user create: %v", err)
	}
	u.ID = uid
	cleanup := func() {
		db.Close()
	}
	return context.Background(), cfg, u, is, ds, us, svc, docsSvc, cleanup
}

func createIncident(t *testing.T, ctx context.Context, is store.IncidentsStore, cfg *config.AppConfig, owner *store.User) *store.Incident {
	t.Helper()
	incident := &store.Incident{
		Title:       "Test incident",
		Description: "Desc",
		Severity:    "medium",
		Status:      "draft",
		OwnerUserID: owner.ID,
		CreatedBy:   owner.ID,
		UpdatedBy:   owner.ID,
		Version:     1,
	}
	if _, err := is.CreateIncident(ctx, incident, nil, nil, cfg.Incidents.RegNoFormat); err != nil {
		t.Fatalf("create incident: %v", err)
	}
	return incident
}

func TestIncidentACLDenyByDefault(t *testing.T) {
	_, _, user, _, _, _, svc, _, cleanup := setupIncidents(t)
	defer cleanup()
	if svc.CheckACL(user, []string{}, []store.ACLRule{}, "view") {
		t.Fatalf("expected deny for empty ACL")
	}
}

func TestIncidentOwnerManageOnCreate(t *testing.T) {
	ctx, cfg, user, is, _, _, _, _, cleanup := setupIncidents(t)
	defer cleanup()
	incident := createIncident(t, ctx, is, cfg, user)
	acl, err := is.GetIncidentACL(ctx, incident.ID)
	if err != nil {
		t.Fatalf("get acl: %v", err)
	}
	needle := strconv.FormatInt(user.ID, 10)
	found := false
	for _, rule := range acl {
		if rule.SubjectType == "user" && rule.SubjectID == needle && rule.Permission == "manage" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected owner manage acl")
	}
}

func TestIncidentCannotDeleteOverviewStage(t *testing.T) {
	ctx, cfg, user, is, _, us, svc, _, cleanup := setupIncidents(t)
	defer cleanup()
	incident := createIncident(t, ctx, is, cfg, user)
	stages, err := is.ListIncidentStages(ctx, incident.ID)
	if err != nil || len(stages) == 0 {
		t.Fatalf("stages missing: %v", err)
	}
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	h := handlers.NewIncidentsHandler(cfg, is, nil, nil, us, nil, policy, svc, nil, nil, utils.NewLogger())
	req := httptest.NewRequest("DELETE", "/api/incidents/"+strconv.FormatInt(incident.ID, 10)+"/stages/"+strconv.FormatInt(stages[0].ID, 10), nil)
	req = withURLParams(req, map[string]string{
		"id":       strconv.FormatInt(incident.ID, 10),
		"stage_id": strconv.FormatInt(stages[0].ID, 10),
	})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	rr := httptest.NewRecorder()
	h.DeleteStage(rr, req)
	if rr.Code != 400 {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestIncidentCompleteStageSetsStatusAndMetadata(t *testing.T) {
	ctx, cfg, user, is, ds, us, svc, docsSvc, cleanup := setupIncidents(t)
	defer cleanup()
	incident := createIncident(t, ctx, is, cfg, user)
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	h := handlers.NewIncidentsHandler(cfg, is, nil, nil, us, ds, policy, svc, docsSvc, nil, utils.NewLogger())

	// create a custom stage via handler to honor ACL/versioning defaults
	body := bytes.NewBufferString(`{"title":"Stage A"}`)
	req := httptest.NewRequest("POST", "/api/incidents/"+strconv.FormatInt(incident.ID, 10)+"/stages", body)
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(incident.ID, 10)})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	rr := httptest.NewRecorder()
	h.AddStage(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("add stage status: %d", rr.Code)
	}
	var created struct {
		Stage store.IncidentStage      `json:"stage"`
		Entry store.IncidentStageEntry `json:"entry"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&created); err != nil {
		t.Fatalf("decode stage: %v", err)
	}

	completeReq := httptest.NewRequest("POST", fmt.Sprintf("/api/incidents/%d/stages/%d/complete", incident.ID, created.Stage.ID), nil)
	completeReq = withURLParams(completeReq, map[string]string{
		"id":       strconv.FormatInt(incident.ID, 10),
		"stage_id": strconv.FormatInt(created.Stage.ID, 10),
	})
	completeReq = completeReq.WithContext(context.WithValue(completeReq.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	rr2 := httptest.NewRecorder()
	h.CompleteStage(rr2, completeReq)
	if rr2.Code != http.StatusOK {
		t.Fatalf("complete stage status: %d", rr2.Code)
	}
	var completed store.IncidentStage
	if err := json.NewDecoder(rr2.Body).Decode(&completed); err != nil {
		t.Fatalf("decode complete response: %v", err)
	}
	if strings.ToLower(completed.Status) != "done" {
		t.Fatalf("expected status done, got %s", completed.Status)
	}
	if completed.ClosedBy == nil || *completed.ClosedBy != user.ID {
		t.Fatalf("expected closed_by to be set")
	}
	if completed.ClosedAt == nil || completed.ClosedAt.IsZero() {
		t.Fatalf("expected closed_at to be set")
	}
}

func TestIncidentCompletedStageIsReadOnlyForContent(t *testing.T) {
	ctx, cfg, user, is, ds, us, svc, docsSvc, cleanup := setupIncidents(t)
	defer cleanup()
	incident := createIncident(t, ctx, is, cfg, user)
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	h := handlers.NewIncidentsHandler(cfg, is, nil, nil, us, ds, policy, svc, docsSvc, nil, utils.NewLogger())

	body := bytes.NewBufferString(`{"title":"Stage B"}`)
	req := httptest.NewRequest("POST", "/api/incidents/"+strconv.FormatInt(incident.ID, 10)+"/stages", body)
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(incident.ID, 10)})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	rr := httptest.NewRecorder()
	h.AddStage(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("add stage status: %d", rr.Code)
	}
	var created struct {
		Stage store.IncidentStage      `json:"stage"`
		Entry store.IncidentStageEntry `json:"entry"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&created); err != nil {
		t.Fatalf("decode stage: %v", err)
	}
	completeReq := httptest.NewRequest("POST", fmt.Sprintf("/api/incidents/%d/stages/%d/complete", incident.ID, created.Stage.ID), nil)
	completeReq = withURLParams(completeReq, map[string]string{
		"id":       strconv.FormatInt(incident.ID, 10),
		"stage_id": strconv.FormatInt(created.Stage.ID, 10),
	})
	completeReq = completeReq.WithContext(context.WithValue(completeReq.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	rr2 := httptest.NewRecorder()
	h.CompleteStage(rr2, completeReq)
	if rr2.Code != http.StatusOK {
		t.Fatalf("complete stage status: %d", rr2.Code)
	}

	updatePayload := map[string]any{
		"content":       `{"stageType":"custom","blocks":[]}`,
		"change_reason": "update after completion",
		"version":       created.Entry.Version,
	}
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(updatePayload); err != nil {
		t.Fatalf("encode payload: %v", err)
	}
	updateReq := httptest.NewRequest("PUT", fmt.Sprintf("/api/incidents/%d/stages/%d/content", incident.ID, created.Stage.ID), buf)
	updateReq = withURLParams(updateReq, map[string]string{
		"id":       strconv.FormatInt(incident.ID, 10),
		"stage_id": strconv.FormatInt(created.Stage.ID, 10),
	})
	updateReq = updateReq.WithContext(context.WithValue(updateReq.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	rr3 := httptest.NewRecorder()
	h.UpdateStageContent(rr3, updateReq)
	if rr3.Code != http.StatusConflict {
		t.Fatalf("expected conflict when updating completed stage, got %d", rr3.Code)
	}
}

func TestIncidentRegNoUniqueness(t *testing.T) {
	ctx, cfg, user, is, _, _, _, _, cleanup := setupIncidents(t)
	defer cleanup()
	inc1 := createIncident(t, ctx, is, cfg, user)
	inc2 := createIncident(t, ctx, is, cfg, user)
	if inc1.RegNo == inc2.RegNo {
		t.Fatalf("reg numbers must differ")
	}
}

func TestIncidentOptimisticConflict(t *testing.T) {
	ctx, cfg, user, is, _, _, _, _, cleanup := setupIncidents(t)
	defer cleanup()
	incident := createIncident(t, ctx, is, cfg, user)
	loaded, err := is.GetIncident(ctx, incident.ID)
	if err != nil || loaded == nil {
		t.Fatalf("load: %v", err)
	}
	loaded.Title = "Updated"
	if err := is.UpdateIncident(ctx, loaded, loaded.Version+1); err == nil {
		t.Fatalf("expected conflict on stale version")
	}
}

func TestIncidentSoftDeleteAndRestore(t *testing.T) {
	ctx, cfg, user, is, _, _, _, _, cleanup := setupIncidents(t)
	defer cleanup()
	incident := createIncident(t, ctx, is, cfg, user)
	if err := is.SoftDeleteIncident(ctx, incident.ID, user.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	list, err := is.ListIncidents(ctx, store.IncidentFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected no incidents in default list")
	}
	list, err = is.ListIncidents(ctx, store.IncidentFilter{IncludeDeleted: true})
	if err != nil {
		t.Fatalf("list deleted: %v", err)
	}
	if len(list) != 1 || list[0].ID != incident.ID {
		t.Fatalf("expected deleted incident to be listed with include_deleted")
	}
	if err := is.RestoreIncident(ctx, incident.ID, user.ID); err != nil {
		t.Fatalf("restore: %v", err)
	}
	list, err = is.ListIncidents(ctx, store.IncidentFilter{})
	if err != nil {
		t.Fatalf("list after restore: %v", err)
	}
	if len(list) != 1 || list[0].ID != incident.ID {
		t.Fatalf("expected incident restored")
	}
}

func TestIncidentStageCompleteLocksContent(t *testing.T) {
	ctx, cfg, user, is, _, us, svc, _, cleanup := setupIncidents(t)
	defer cleanup()
	incident := createIncident(t, ctx, is, cfg, user)
	stage := &store.IncidentStage{
		IncidentID: incident.ID,
		Title:      "Investigation",
		Position:   2,
		CreatedBy:  user.ID,
		UpdatedBy:  user.ID,
	}
	if _, err := is.CreateIncidentStage(ctx, stage); err != nil {
		t.Fatalf("create stage: %v", err)
	}
	entry := &store.IncidentStageEntry{
		StageID:   stage.ID,
		Content:   `{"stageType":"investigation","blocks":[{"type":"note","items":[{"text":"draft"}]}]}`,
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
		Version:   1,
	}
	if _, err := is.CreateStageEntry(ctx, entry); err != nil {
		t.Fatalf("create entry: %v", err)
	}
	h := handlers.NewIncidentsHandler(cfg, is, nil, nil, us, nil, rbac.NewPolicy(rbac.DefaultRoles()), svc, nil, nil, utils.NewLogger())
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/incidents/%d/stages/%d/complete", incident.ID, stage.ID), nil)
	req = withURLParams(req, map[string]string{"id": fmt.Sprintf("%d", incident.ID), "stage_id": fmt.Sprintf("%d", stage.ID)})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	rr := httptest.NewRecorder()
	h.CompleteStage(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("complete stage status: %d", rr.Code)
	}
	updatedStage, err := is.GetIncidentStage(ctx, stage.ID)
	if err != nil || updatedStage == nil || strings.ToLower(updatedStage.Status) != "done" {
		t.Fatalf("stage not marked done: %v", err)
	}
	payload := map[string]any{
		"content":       `{"stageType":"investigation","blocks":[{"type":"note","items":[{"text":"updated"}]}]}`,
		"change_reason": "test",
		"version":       entry.Version,
	}
	body, _ := json.Marshal(payload)
	updReq := httptest.NewRequest("PUT", fmt.Sprintf("/api/incidents/%d/stages/%d/content", incident.ID, stage.ID), bytes.NewReader(body))
	updReq = withURLParams(updReq, map[string]string{"id": fmt.Sprintf("%d", incident.ID), "stage_id": fmt.Sprintf("%d", stage.ID)})
	updReq = updReq.WithContext(context.WithValue(updReq.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	updRR := httptest.NewRecorder()
	h.UpdateStageContent(updRR, updReq)
	if updRR.Code != http.StatusConflict {
		t.Fatalf("expected conflict for done stage, got %d", updRR.Code)
	}
}

func TestIncidentCloseRequiresCompletionStage(t *testing.T) {
	ctx, cfg, user, is, _, us, svc, _, cleanup := setupIncidents(t)
	defer cleanup()
	incident := createIncident(t, ctx, is, cfg, user)
	stage := &store.IncidentStage{
		IncidentID: incident.ID,
		Title:      "Closure",
		Position:   2,
		CreatedBy:  user.ID,
		UpdatedBy:  user.ID,
	}
	if _, err := is.CreateIncidentStage(ctx, stage); err != nil {
		t.Fatalf("create closure stage: %v", err)
	}
	contentPayload := map[string]any{
		"stageType": "closure",
		"blocks": []map[string]any{
			{"type": "decisions", "items": []map[string]any{{"decision": "done"}}},
		},
	}
	contentBytes, _ := json.Marshal(contentPayload)
	entry := &store.IncidentStageEntry{
		StageID:   stage.ID,
		Content:   string(contentBytes),
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
		Version:   1,
	}
	if _, err := is.CreateStageEntry(ctx, entry); err != nil {
		t.Fatalf("create closure entry: %v", err)
	}
	if _, err := is.CompleteIncidentStage(ctx, stage.ID, user.ID); err != nil {
		t.Fatalf("complete closure stage: %v", err)
	}
	h := handlers.NewIncidentsHandler(cfg, is, nil, nil, us, nil, rbac.NewPolicy(rbac.DefaultRoles()), svc, nil, nil, utils.NewLogger())
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/incidents/%d/close", incident.ID), nil)
	req = withURLParams(req, map[string]string{"id": fmt.Sprintf("%d", incident.ID)})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	rr := httptest.NewRecorder()
	h.CloseIncident(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("close incident status: %d", rr.Code)
	}
	closed, err := is.GetIncident(ctx, incident.ID)
	if err != nil || closed == nil || strings.ToLower(closed.Status) != "closed" {
		t.Fatalf("incident not closed: %v", err)
	}
	payload := map[string]any{
		"message":    "after close",
		"event_type": "note",
	}
	body, _ := json.Marshal(payload)
	tlReq := httptest.NewRequest("POST", fmt.Sprintf("/api/incidents/%d/timeline", incident.ID), bytes.NewReader(body))
	tlReq = withURLParams(tlReq, map[string]string{"id": fmt.Sprintf("%d", incident.ID)})
	tlReq = tlReq.WithContext(context.WithValue(tlReq.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	tlRR := httptest.NewRecorder()
	h.AddTimeline(tlRR, tlReq)
	if tlRR.Code != http.StatusConflict {
		t.Fatalf("expected conflict for closed incident edits, got %d", tlRR.Code)
	}
}

func TestIncidentLinksAddRemoveACL(t *testing.T) {
	ctx, cfg, user, is, ds, us, svc, docsSvc, cleanup := setupIncidents(t)
	defer cleanup()
	incident := createIncident(t, ctx, is, cfg, user)
	other := &store.User{
		Username:       "user2",
		FullName:       "User Two",
		Department:     "Sec",
		Position:       "Viewer",
		ClearanceLevel: int(docs.ClassificationInternal),
		PasswordHash:   "hash",
		Salt:           "salt",
		PasswordSet:    true,
		Active:         true,
	}
	otherID, err := us.Create(ctx, other, []string{"doc_viewer"})
	if err != nil {
		t.Fatalf("user2 create: %v", err)
	}
	other.ID = otherID
	doc := &store.Document{
		Title:               "Test Doc",
		Status:              docs.StatusDraft,
		ClassificationLevel: int(docs.ClassificationInternal),
		ClassificationTags:  nil,
		InheritACL:          true,
		CreatedBy:           other.ID,
	}
	acl := []store.ACLRule{{SubjectType: "user", SubjectID: other.Username, Permission: "view"}}
	if _, err := ds.CreateDocument(ctx, doc, acl, cfg.Docs.RegTemplate, cfg.Docs.PerFolderSequence); err != nil {
		t.Fatalf("doc create: %v", err)
	}
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	h := handlers.NewIncidentsHandler(cfg, is, nil, nil, us, ds, policy, svc, docsSvc, nil, utils.NewLogger())
	body, _ := json.Marshal(map[string]string{"target_type": "doc", "target_id": strconv.FormatInt(doc.ID, 10)})
	req := httptest.NewRequest("POST", "/api/incidents/"+strconv.FormatInt(incident.ID, 10)+"/links", bytes.NewReader(body))
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(incident.ID, 10)})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	rr := httptest.NewRecorder()
	h.AddLink(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	acl = []store.ACLRule{{SubjectType: "user", SubjectID: user.Username, Permission: "view"}}
	doc2 := &store.Document{
		Title:               "Doc2",
		Status:              docs.StatusDraft,
		ClassificationLevel: int(docs.ClassificationInternal),
		ClassificationTags:  nil,
		InheritACL:          true,
		CreatedBy:           user.ID,
	}
	if _, err := ds.CreateDocument(ctx, doc2, acl, cfg.Docs.RegTemplate, cfg.Docs.PerFolderSequence); err != nil {
		t.Fatalf("doc2 create: %v", err)
	}
	body, _ = json.Marshal(map[string]string{"target_type": "doc", "target_id": strconv.FormatInt(doc2.ID, 10)})
	req = httptest.NewRequest("POST", "/api/incidents/"+strconv.FormatInt(incident.ID, 10)+"/links", bytes.NewReader(body))
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(incident.ID, 10)})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	rr = httptest.NewRecorder()
	h.AddLink(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
	links, err := is.ListIncidentLinks(ctx, incident.ID)
	if err != nil || len(links) == 0 {
		t.Fatalf("links missing: %v", err)
	}
	req = httptest.NewRequest("DELETE", "/api/incidents/"+strconv.FormatInt(incident.ID, 10)+"/links/"+strconv.FormatInt(links[0].ID, 10), nil)
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(incident.ID, 10), "link_id": strconv.FormatInt(links[0].ID, 10)})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	rr = httptest.NewRecorder()
	h.DeleteLink(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestIncidentAttachmentEncryptDecryptAndClearance(t *testing.T) {
	ctx, cfg, user, is, _, us, svc, docsSvc, cleanup := setupIncidents(t)
	defer cleanup()
	incident := createIncident(t, ctx, is, cfg, user)
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	h := handlers.NewIncidentsHandler(cfg, is, nil, nil, us, nil, policy, svc, docsSvc, nil, utils.NewLogger())
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "artifact.txt")
	_, _ = io.Copy(part, strings.NewReader("secret payload"))
	writer.Close()
	req := httptest.NewRequest("POST", "/api/incidents/"+strconv.FormatInt(incident.ID, 10)+"/attachments/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(incident.ID, 10)})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	rr := httptest.NewRecorder()
	h.UploadAttachment(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
	attachments, err := is.ListIncidentAttachments(ctx, incident.ID)
	if err != nil || len(attachments) == 0 {
		t.Fatalf("attachments missing: %v", err)
	}
	att := attachments[0]
	req = httptest.NewRequest("GET", "/api/incidents/"+strconv.FormatInt(incident.ID, 10)+"/attachments/"+strconv.FormatInt(att.ID, 10)+"/download", nil)
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(incident.ID, 10), "att_id": strconv.FormatInt(att.ID, 10)})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	rr = httptest.NewRecorder()
	h.DownloadAttachment(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "secret payload") {
		t.Fatalf("download mismatch")
	}
	low := &store.User{
		Username:       "low",
		FullName:       "Low Clearance",
		Department:     "Sec",
		Position:       "Trainee",
		ClearanceLevel: int(docs.ClassificationPublic),
		PasswordHash:   "hash",
		Salt:           "salt",
		PasswordSet:    true,
		Active:         true,
	}
	lowID, err := us.Create(ctx, low, []string{"doc_viewer"})
	if err != nil {
		t.Fatalf("low user: %v", err)
	}
	low.ID = lowID
	acl, _ := is.GetIncidentACL(ctx, incident.ID)
	acl = append(acl, store.ACLRule{SubjectType: "user", SubjectID: low.Username, Permission: "view"})
	if err := is.SetIncidentACL(ctx, incident.ID, acl); err != nil {
		t.Fatalf("acl set: %v", err)
	}
	req = httptest.NewRequest("GET", "/api/incidents/"+strconv.FormatInt(incident.ID, 10)+"/attachments/"+strconv.FormatInt(att.ID, 10)+"/download", nil)
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(incident.ID, 10), "att_id": strconv.FormatInt(att.ID, 10)})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: low.ID, Username: low.Username}))
	rr = httptest.NewRecorder()
	h.DownloadAttachment(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestIncidentTimelineOnStatusChange(t *testing.T) {
	ctx, cfg, user, is, _, us, svc, docsSvc, cleanup := setupIncidents(t)
	defer cleanup()
	incident := createIncident(t, ctx, is, cfg, user)
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	h := handlers.NewIncidentsHandler(cfg, is, nil, nil, us, nil, policy, svc, docsSvc, nil, utils.NewLogger())
	body, _ := json.Marshal(map[string]any{"status": "open", "version": incident.Version})
	req := httptest.NewRequest("PUT", "/api/incidents/"+strconv.FormatInt(incident.ID, 10), bytes.NewReader(body))
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(incident.ID, 10)})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	rr := httptest.NewRecorder()
	h.Update(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	events, err := is.ListIncidentTimeline(ctx, incident.ID, 50, "status.change")
	if err != nil || len(events) == 0 {
		t.Fatalf("expected timeline event, err=%v", err)
	}
}

func TestIncidentExportIncludesStages(t *testing.T) {
	ctx, cfg, user, is, _, us, svc, docsSvc, cleanup := setupIncidents(t)
	defer cleanup()
	incident := createIncident(t, ctx, is, cfg, user)
	stages, _ := is.ListIncidentStages(ctx, incident.ID)
	if len(stages) == 0 {
		t.Fatalf("stages missing")
	}
	entry, _ := is.GetStageEntry(ctx, stages[0].ID)
	if entry == nil {
		t.Fatalf("stage entry missing")
	}
	entry.Content = "Stage content sample"
	if err := is.UpdateStageEntry(ctx, entry, entry.Version); err != nil {
		t.Fatalf("stage update: %v", err)
	}
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	h := handlers.NewIncidentsHandler(cfg, is, nil, nil, us, nil, policy, svc, docsSvc, nil, utils.NewLogger())
	req := httptest.NewRequest("GET", "/api/incidents/"+strconv.FormatInt(incident.ID, 10)+"/export?format=md", nil)
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(incident.ID, 10)})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	rr := httptest.NewRecorder()
	h.Export(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Stage content sample") {
		t.Fatalf("export missing stage content")
	}
}

func TestIncidentCreateReportDocRequiresDocsCreateAndLink(t *testing.T) {
	ctx, cfg, user, is, ds, us, svc, docsSvc, cleanup := setupIncidents(t)
	defer cleanup()
	incident := createIncident(t, ctx, is, cfg, user)
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	h := handlers.NewIncidentsHandler(cfg, is, nil, nil, us, ds, policy, svc, docsSvc, nil, utils.NewLogger())
	req := httptest.NewRequest("POST", "/api/incidents/"+strconv.FormatInt(incident.ID, 10)+"/create-report-doc", bytes.NewReader([]byte(`{}`)))
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(incident.ID, 10)})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	rr := httptest.NewRecorder()
	h.CreateReportDoc(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	admin := &store.User{
		Username:       "docadmin",
		FullName:       "Doc Admin",
		Department:     "Sec",
		Position:       "Admin",
		ClearanceLevel: int(docs.ClassificationInternal),
		PasswordHash:   "hash",
		Salt:           "salt",
		PasswordSet:    true,
		Active:         true,
	}
	adminID, err := us.Create(ctx, admin, []string{"doc_admin"})
	if err != nil {
		t.Fatalf("admin create: %v", err)
	}
	admin.ID = adminID
	incident2 := createIncident(t, ctx, is, cfg, admin)
	req = httptest.NewRequest("POST", "/api/incidents/"+strconv.FormatInt(incident2.ID, 10)+"/create-report-doc", bytes.NewReader([]byte(`{}`)))
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(incident2.ID, 10)})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: admin.ID, Username: admin.Username}))
	rr = httptest.NewRecorder()
	h.CreateReportDoc(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
	var resp struct {
		DocID int64 `json:"doc_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil || resp.DocID == 0 {
		t.Fatalf("doc_id missing")
	}
	if doc, _ := ds.GetDocument(ctx, resp.DocID); doc == nil {
		t.Fatalf("doc not created")
	}
	links, _ := is.ListIncidentLinks(ctx, incident2.ID)
	if len(links) == 0 {
		t.Fatalf("incident link not created")
	}
}

func TestIncidentLinkOtherRequiresComment(t *testing.T) {
	ctx, cfg, user, is, ds, us, svc, docsSvc, cleanup := setupIncidents(t)
	defer cleanup()
	incident := createIncident(t, ctx, is, cfg, user)
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	h := handlers.NewIncidentsHandler(cfg, is, nil, nil, us, ds, policy, svc, docsSvc, nil, utils.NewLogger())
	body, _ := json.Marshal(map[string]string{"target_type": "other", "comment": ""})
	req := httptest.NewRequest("POST", "/api/incidents/"+strconv.FormatInt(incident.ID, 10)+"/links", bytes.NewReader(body))
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(incident.ID, 10)})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	rr := httptest.NewRecorder()
	h.AddLink(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing comment, got %d", rr.Code)
	}
}

func TestIncidentArtifactFilesACL(t *testing.T) {
	ctx, cfg, user, is, ds, us, svc, docsSvc, cleanup := setupIncidents(t)
	defer cleanup()
	incident := createIncident(t, ctx, is, cfg, user)
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	h := handlers.NewIncidentsHandler(cfg, is, nil, nil, us, ds, policy, svc, docsSvc, nil, utils.NewLogger())
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "artifact.txt")
	_, _ = io.Copy(part, strings.NewReader("artifact payload"))
	writer.Close()
	req := httptest.NewRequest("POST", "/api/incidents/"+strconv.FormatInt(incident.ID, 10)+"/artifacts/art-1/files", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(incident.ID, 10), "artifact_id": "art-1"})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	rr := httptest.NewRecorder()
	h.UploadArtifactFile(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 upload, got %d", rr.Code)
	}
	listReq := httptest.NewRequest("GET", "/api/incidents/"+strconv.FormatInt(incident.ID, 10)+"/artifacts/art-1/files", nil)
	listReq = withURLParams(listReq, map[string]string{"id": strconv.FormatInt(incident.ID, 10), "artifact_id": "art-1"})
	listReq = listReq.WithContext(context.WithValue(listReq.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	listRR := httptest.NewRecorder()
	h.ListArtifactFiles(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d", listRR.Code)
	}
	var listPayload struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listRR.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("list decode: %v", err)
	}
	if len(listPayload.Items) != 1 {
		t.Fatalf("expected 1 artifact file, got %d", len(listPayload.Items))
	}
	fileID := listPayload.Items[0].ID
	dlReq := httptest.NewRequest("GET", "/api/incidents/"+strconv.FormatInt(incident.ID, 10)+"/artifacts/art-1/files/"+strconv.FormatInt(fileID, 10)+"/download", nil)
	dlReq = withURLParams(dlReq, map[string]string{"id": strconv.FormatInt(incident.ID, 10), "artifact_id": "art-1", "file_id": strconv.FormatInt(fileID, 10)})
	dlReq = dlReq.WithContext(context.WithValue(dlReq.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	dlRR := httptest.NewRecorder()
	h.DownloadArtifactFile(dlRR, dlReq)
	if dlRR.Code != http.StatusOK || !strings.Contains(dlRR.Body.String(), "artifact payload") {
		t.Fatalf("download failed code=%d body=%s", dlRR.Code, dlRR.Body.String())
	}
	low := &store.User{
		Username:       "low-art",
		FullName:       "Low Artifact",
		Department:     "Sec",
		Position:       "Trainee",
		ClearanceLevel: int(docs.ClassificationPublic),
		PasswordHash:   "hash",
		Salt:           "salt",
		PasswordSet:    true,
		Active:         true,
	}
	lowID, err := us.Create(ctx, low, []string{"doc_viewer"})
	if err != nil {
		t.Fatalf("low user: %v", err)
	}
	low.ID = lowID
	acl, _ := is.GetIncidentACL(ctx, incident.ID)
	acl = append(acl, store.ACLRule{SubjectType: "user", SubjectID: low.Username, Permission: "view"})
	if err := is.SetIncidentACL(ctx, incident.ID, acl); err != nil {
		t.Fatalf("acl set: %v", err)
	}
	lowReq := httptest.NewRequest("GET", "/api/incidents/"+strconv.FormatInt(incident.ID, 10)+"/artifacts/art-1/files/"+strconv.FormatInt(fileID, 10)+"/download", nil)
	lowReq = withURLParams(lowReq, map[string]string{"id": strconv.FormatInt(incident.ID, 10), "artifact_id": "art-1", "file_id": strconv.FormatInt(fileID, 10)})
	lowReq = lowReq.WithContext(context.WithValue(lowReq.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: low.ID, Username: low.Username}))
	lowRR := httptest.NewRecorder()
	h.DownloadArtifactFile(lowRR, lowReq)
	if lowRR.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for low clearance, got %d", lowRR.Code)
	}
	delReq := httptest.NewRequest("DELETE", "/api/incidents/"+strconv.FormatInt(incident.ID, 10)+"/artifacts/art-1/files/"+strconv.FormatInt(fileID, 10), nil)
	delReq = withURLParams(delReq, map[string]string{"id": strconv.FormatInt(incident.ID, 10), "artifact_id": "art-1", "file_id": strconv.FormatInt(fileID, 10)})
	delReq = delReq.WithContext(context.WithValue(delReq.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	delRR := httptest.NewRecorder()
	h.DeleteArtifactFile(delRR, delReq)
	if delRR.Code != http.StatusOK {
		t.Fatalf("expected delete 200, got %d", delRR.Code)
	}
	listRR2 := httptest.NewRecorder()
	h.ListArtifactFiles(listRR2, listReq)
	var listPayload2 struct {
		Items []any `json:"items"`
	}
	_ = json.Unmarshal(listRR2.Body.Bytes(), &listPayload2)
	if len(listPayload2.Items) != 0 {
		t.Fatalf("expected no files after delete")
	}
}

func TestIncidentActivityEventAt(t *testing.T) {
	ctx, cfg, user, is, ds, us, svc, docsSvc, cleanup := setupIncidents(t)
	defer cleanup()
	incident := createIncident(t, ctx, is, cfg, user)
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	h := handlers.NewIncidentsHandler(cfg, is, nil, nil, us, ds, policy, svc, docsSvc, nil, utils.NewLogger())
	eventAt := time.Now().Add(-2 * time.Hour).UTC()
	body, _ := json.Marshal(map[string]any{"message": "note", "event_type": "custom", "event_at": eventAt.Format(time.RFC3339)})
	req := httptest.NewRequest("POST", "/api/incidents/"+strconv.FormatInt(incident.ID, 10)+"/timeline", bytes.NewReader(body))
	req = withURLParams(req, map[string]string{"id": strconv.FormatInt(incident.ID, 10)})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	rr := httptest.NewRecorder()
	h.AddTimeline(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
	actReq := httptest.NewRequest("GET", "/api/incidents/"+strconv.FormatInt(incident.ID, 10)+"/activity", nil)
	actReq = withURLParams(actReq, map[string]string{"id": strconv.FormatInt(incident.ID, 10)})
	actReq = actReq.WithContext(context.WithValue(actReq.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: user.ID, Username: user.Username}))
	actRR := httptest.NewRecorder()
	h.ListActivity(actRR, actReq)
	if actRR.Code != http.StatusOK {
		t.Fatalf("activity code %d", actRR.Code)
	}
	var payload struct {
		Items []struct {
			EventAt string `json:"event_at"`
		} `json:"items"`
	}
	if err := json.Unmarshal(actRR.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode activity: %v", err)
	}
	if len(payload.Items) == 0 {
		t.Fatalf("expected activity items")
	}
	got, err := time.Parse(time.RFC3339, payload.Items[0].EventAt)
	if err != nil || got.UTC().Format(time.RFC3339) != eventAt.Format(time.RFC3339) {
		t.Fatalf("event_at mismatch got %v want %v", got, eventAt)
	}
}
