package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/docs"
	"berkut-scc/core/store"
)

func (h *DocsHandler) requiresDualExportApproval(ctx context.Context, user *store.User, roles []string, doc *store.Document) bool {
	if h == nil || doc == nil || h.cfg == nil || !h.cfg.Docs.DLP.Enabled {
		return false
	}
	if h.adminExportBypassEnabled(ctx) && (hasRole(roles, "admin") || hasRole(roles, "superadmin")) {
		return false
	}
	level := docs.ClassificationLevel(doc.ClassificationLevel)
	// DLP-lite: enforce dual control for CONFIDENTIAL, DSP(Restricted), and custom-tagged docs.
	if level == docs.ClassificationConfidential || level == docs.ClassificationRestricted {
		return true
	}
	return len(doc.ClassificationTags) > 0
}

func (h *DocsHandler) adminExportBypassEnabled(ctx context.Context) bool {
	if h == nil {
		return true
	}
	enabled := true
	if h.modules == nil {
		return enabled
	}
	st, err := h.modules.Get(ctx, hardeningSettingsModuleID)
	if err != nil || st == nil || strings.TrimSpace(st.LastError) == "" {
		return enabled
	}
	var payload struct {
		DLP struct {
			AdminExportWithoutApproval *bool `json:"admin_export_without_approval"`
		} `json:"dlp"`
	}
	if err := json.Unmarshal([]byte(st.LastError), &payload); err != nil {
		return enabled
	}
	if payload.DLP.AdminExportWithoutApproval != nil {
		enabled = *payload.DLP.AdminExportWithoutApproval
	}
	return enabled
}

func (h *DocsHandler) ApproveExport(w http.ResponseWriter, r *http.Request) {
	requester, requesterRoles, err := h.currentUser(r)
	if err != nil || requester == nil {
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
	if !h.svc.CheckACL(requester, requesterRoles, doc, docACL, folderACL, "export") {
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
	reason := strings.TrimSpace(payload.Reason)
	if reason == "" {
		http.Error(w, "docs.export.reasonRequired", http.StatusBadRequest)
		return
	}
	var approverUser *store.User
	var approverRoles []string
	if payload.RequestedUserID != nil && *payload.RequestedUserID > 0 {
		approverUser, approverRoles, _ = h.users.Get(r.Context(), *payload.RequestedUserID)
	} else if strings.TrimSpace(payload.RequestedUsername) != "" {
		approverUser, approverRoles, _ = h.users.FindByUsername(r.Context(), strings.ToLower(strings.TrimSpace(payload.RequestedUsername)))
	}
	if approverUser == nil || !approverUser.Active {
		http.Error(w, "docs.export.approverRequired", http.StatusBadRequest)
		return
	}
	if !h.canUserParticipateInExportApproval(approverUser, approverRoles, doc, docACL, folderACL) {
		http.Error(w, "docs.export.approverRequired", http.StatusBadRequest)
		return
	}
	if key, code := h.validateExportApprovalPair(approverUser, approverRoles, requester, requesterRoles); key != "" {
		http.Error(w, key, code)
		return
	}
	expireMin := h.cfg.Docs.DLP.DualApprovalMinutes
	if expireMin <= 0 {
		expireMin = 30
	}
	item := &store.DocExportApproval{
		DocID:       doc.ID,
		RequestedBy: requester.ID,
		ApprovedBy:  approverUser.ID,
		Reason:      reason,
		CreatedAt:   time.Now().UTC(),
		ExpiresAt:   time.Now().UTC().Add(time.Duration(expireMin) * time.Minute),
	}
	idCreated, err := h.store.CreateDocExportApproval(r.Context(), item)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.svc.Log(r.Context(), requester.Username, "doc.export.approval.requested", fmt.Sprintf("%s|approver=%s|approval=%d", doc.RegNumber, approverUser.Username, idCreated))
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":              idCreated,
		"doc_id":          doc.ID,
		"requested_by":    requester.Username,
		"approved_by":     approverUser.Username,
		"expires_at":      item.ExpiresAt,
		"approval_needed": h.requiresDualExportApproval(r.Context(), requester, requesterRoles, doc),
	})
}

func (h *DocsHandler) DecideExportApproval(w http.ResponseWriter, r *http.Request) {
	user, _, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id, _ := strconv.ParseInt(pathParams(r)["approval_id"], 10, 64)
	item, err := h.store.GetDocExportApproval(r.Context(), id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if item == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if item.ApprovedBy != user.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if item.ConsumedAt != nil {
		http.Error(w, "docs.export.alreadyUsed", http.StatusConflict)
		return
	}
	var payload struct {
		Decision string `json:"decision"`
		Comment  string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	decision := strings.ToLower(strings.TrimSpace(payload.Decision))
	comment := strings.TrimSpace(payload.Comment)
	if decision != "approve" && decision != "reject" {
		http.Error(w, "docs.export.invalidDecision", http.StatusBadRequest)
		return
	}
	if comment == "" {
		http.Error(w, "docs.export.reasonRequired", http.StatusBadRequest)
		return
	}
	existing, err := h.store.GetDocExportApprovalDecision(r.Context(), item.ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if existing != nil {
		http.Error(w, "docs.export.alreadyDecided", http.StatusConflict)
		return
	}
	rec := &store.DocExportApprovalDecision{
		ApprovalID: item.ID,
		Decision:   decision,
		Comment:    comment,
		DecidedBy:  user.ID,
		DecidedAt:  time.Now().UTC(),
	}
	if err := h.store.SaveDocExportApprovalDecision(r.Context(), rec); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	action := "doc.export.approval.approved"
	if decision == "reject" {
		action = "doc.export.approval.rejected"
	}
	h.svc.Log(r.Context(), user.Username, action, fmt.Sprintf("%d|%s", item.ID, comment))
	writeJSON(w, http.StatusOK, map[string]any{
		"id":         item.ID,
		"decision":   decision,
		"comment":    comment,
		"decided_by": user.ID,
		"decided_at": rec.DecidedAt,
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
	if h.behavior != nil {
		evt := "dlp.copy_blocked"
		if eventType == "screenshot_attempt" {
			evt = "dlp.screenshot_attempt"
		}
		_ = h.behavior.RecordEvent(r.Context(), &store.BehaviorRiskEvent{
			UserID:     user.ID,
			EventType:  evt,
			Path:       "/api/docs/" + strconv.FormatInt(doc.ID, 10) + "/security-events",
			Method:     http.MethodPost,
			StatusCode: http.StatusOK,
			IP:         docsSecurityRemoteIP(r),
			CreatedAt:  time.Now().UTC(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func docsSecurityRemoteIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	raw := strings.TrimSpace(r.RemoteAddr)
	if raw == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		return strings.TrimSpace(host)
	}
	return raw
}
