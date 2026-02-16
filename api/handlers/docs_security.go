package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/docs"
	"berkut-scc/core/store"
)

func (h *DocsHandler) requiresDualExportApproval(doc *store.Document) bool {
	if h == nil || doc == nil || h.cfg == nil || !h.cfg.Docs.DLP.Enabled {
		return false
	}
	level := docs.ClassificationLevel(doc.ClassificationLevel)
	// DLP-lite: enforce dual control for CONFIDENTIAL, DSP(Restricted), and custom-tagged docs.
	if level == docs.ClassificationConfidential || level == docs.ClassificationRestricted {
		return true
	}
	return len(doc.ClassificationTags) > 0
}

func (h *DocsHandler) ApproveExport(w http.ResponseWriter, r *http.Request) {
	approver, roles, err := h.currentUser(r)
	if err != nil || approver == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id, _ := strconv.ParseInt(pathParams(r)["id"], 10, 64)
	doc, err := h.store.GetDocument(r.Context(), id)
	if err != nil || doc == nil || !h.isDocument(doc) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	docACL, _ := h.store.GetDocACL(r.Context(), doc.ID)
	var folderACL []store.ACLRule
	if doc.FolderID != nil {
		folderACL, _ = h.store.GetFolderACL(r.Context(), *doc.FolderID)
	}
	if !h.svc.CheckACL(approver, roles, doc, docACL, folderACL, "export") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var payload struct {
		RequestedUserID   *int64 `json:"requested_user_id"`
		RequestedUsername string `json:"requested_username"`
		Reason            string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var requester *store.User
	if payload.RequestedUserID != nil && *payload.RequestedUserID > 0 {
		requester, _, _ = h.users.Get(r.Context(), *payload.RequestedUserID)
	} else if strings.TrimSpace(payload.RequestedUsername) != "" {
		requester, _, _ = h.users.FindByUsername(r.Context(), strings.ToLower(strings.TrimSpace(payload.RequestedUsername)))
	}
	if requester == nil || !requester.Active {
		http.Error(w, "docs.export.requesterRequired", http.StatusBadRequest)
		return
	}
	if requester.ID == approver.ID {
		http.Error(w, "docs.export.selfApprovalForbidden", http.StatusBadRequest)
		return
	}
	expireMin := h.cfg.Docs.DLP.DualApprovalMinutes
	if expireMin <= 0 {
		expireMin = 30
	}
	item := &store.DocExportApproval{
		DocID:       doc.ID,
		RequestedBy: requester.ID,
		ApprovedBy:  approver.ID,
		Reason:      strings.TrimSpace(payload.Reason),
		CreatedAt:   time.Now().UTC(),
		ExpiresAt:   time.Now().UTC().Add(time.Duration(expireMin) * time.Minute),
	}
	idCreated, err := h.store.CreateDocExportApproval(r.Context(), item)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.svc.Log(r.Context(), approver.Username, "doc.export.approval.granted", fmt.Sprintf("%s|requester=%s|approval=%d", doc.RegNumber, requester.Username, idCreated))
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":              idCreated,
		"doc_id":          doc.ID,
		"requested_by":    requester.Username,
		"approved_by":     approver.Username,
		"expires_at":      item.ExpiresAt,
		"approval_needed": h.requiresDualExportApproval(doc),
	})
}

func (h *DocsHandler) LogSecurityEvent(w http.ResponseWriter, r *http.Request) {
	user, roles, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id, _ := strconv.ParseInt(pathParams(r)["id"], 10, 64)
	doc, err := h.store.GetDocument(r.Context(), id)
	if err != nil || doc == nil || !h.isDocument(doc) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	docACL, _ := h.store.GetDocACL(r.Context(), doc.ID)
	var folderACL []store.ACLRule
	if doc.FolderID != nil {
		folderACL, _ = h.store.GetFolderACL(r.Context(), *doc.FolderID)
	}
	if !h.svc.CheckACL(user, roles, doc, docACL, folderACL, "view") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var payload struct {
		EventType string `json:"event_type"`
		Details   string `json:"details"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	eventType := strings.ToLower(strings.TrimSpace(payload.EventType))
	switch eventType {
	case "copy_blocked", "screenshot_attempt":
	default:
		http.Error(w, "docs.security.invalidEvent", http.StatusBadRequest)
		return
	}
	h.svc.Log(r.Context(), user.Username, "doc.security."+eventType, fmt.Sprintf("%s|%s", doc.RegNumber, strings.TrimSpace(payload.Details)))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
