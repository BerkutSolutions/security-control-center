package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"berkut-scc/core/auth"
	"berkut-scc/core/docs"
	"berkut-scc/core/reports/charts"
	"berkut-scc/core/store"
)

func TestReportChartsUpdateRequiresEdit(t *testing.T) {
	env := setupReportsBuilder(t)
	defer env.cleanup()
	doc := &store.Document{
		Title:               "Charts Report",
		Status:              docs.StatusDraft,
		ClassificationLevel: int(docs.ClassificationInternal),
		DocType:             "report",
		InheritACL:          true,
		CreatedBy:           env.user.ID,
	}
	acl := []store.ACLRule{
		{SubjectType: "user", SubjectID: env.user.Username, Permission: "view"},
		{SubjectType: "user", SubjectID: env.user.Username, Permission: "edit"},
		{SubjectType: "user", SubjectID: env.user.Username, Permission: "manage"},
		{SubjectType: "role", SubjectID: "manager", Permission: "view"},
	}
	docID, err := env.docs.CreateDocument(context.Background(), doc, acl, env.cfg.Docs.RegTemplate, env.cfg.Docs.PerFolderSequence)
	if err != nil {
		t.Fatalf("create report doc: %v", err)
	}
	doc.ID = docID
	if err := env.reports.UpsertReportMeta(context.Background(), &store.ReportMeta{DocID: docID, Status: "draft"}); err != nil {
		t.Fatalf("report meta: %v", err)
	}
	manager := &store.User{
		Username:       "manager1",
		FullName:       "Manager",
		Department:     "Ops",
		Position:       "Manager",
		ClearanceLevel: int(docs.ClassificationInternal),
		PasswordHash:   "hash",
		Salt:           "salt",
		PasswordSet:    true,
		Active:         true,
	}
	managerID, err := env.users.Create(context.Background(), manager, []string{"manager"})
	if err != nil {
		t.Fatalf("manager create: %v", err)
	}
	manager.ID = managerID
	payload := map[string]any{"charts": charts.DefaultCharts()}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/reports/%d/charts", doc.ID), bytes.NewReader(body))
	req = withURLParams(req, map[string]string{"id": fmt.Sprintf("%d", doc.ID)})
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{UserID: manager.ID, Username: manager.Username}))
	rr := httptest.NewRecorder()
	env.handler.UpdateCharts(rr, req)
	if rr.Code == http.StatusOK {
		t.Fatalf("expected charts update to be denied")
	}
}
