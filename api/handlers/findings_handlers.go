package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/auth"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
)

type FindingsHandler struct {
	store    store.FindingsStore
	links    store.EntityLinksStore
	users    store.UsersStore
	assets   store.AssetsStore
	ctrls    store.ControlsStore
	software store.SoftwareStore
	policy   *rbac.Policy
	audits   store.AuditStore
}

func NewFindingsHandler(fs store.FindingsStore, links store.EntityLinksStore, us store.UsersStore, assets store.AssetsStore, ctrls store.ControlsStore, software store.SoftwareStore, audits store.AuditStore, policy *rbac.Policy) *FindingsHandler {
	return &FindingsHandler{store: fs, links: links, users: us, assets: assets, ctrls: ctrls, software: software, audits: audits, policy: policy}
}

var validFindingStatus = map[string]struct{}{
	"open":           {},
	"in_progress":    {},
	"resolved":       {},
	"accepted_risk":  {},
	"false_positive": {},
}

var validFindingSeverity = map[string]struct{}{
	"low":      {},
	"medium":   {},
	"high":     {},
	"critical": {},
}

var validFindingType = map[string]struct{}{
	"technical":  {},
	"config":     {},
	"process":    {},
	"compliance": {},
	"other":      {},
}

func (h *FindingsHandler) List(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requirePermission(w, r, "findings.view"); !ok {
		return
	}
	q := r.URL.Query()
	filter := store.FindingFilter{
		Search:         q.Get("q"),
		Status:         strings.ToLower(strings.TrimSpace(q.Get("status"))),
		Severity:       strings.ToLower(strings.TrimSpace(q.Get("severity"))),
		Type:           strings.ToLower(strings.TrimSpace(q.Get("type"))),
		Tag:            q.Get("tag"),
		IncludeDeleted: parseBool(q.Get("include_deleted")),
		Limit:          parseIntDefault(q.Get("limit"), 0),
		Offset:         parseIntDefault(q.Get("offset"), 0),
	}
	items, err := h.store.ListFindings(r.Context(), filter)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *FindingsHandler) ListLite(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requirePermission(w, r, "findings.view"); !ok {
		return
	}
	q := r.URL.Query()
	items, err := h.store.ListFindingsLite(r.Context(), q.Get("q"), parseIntDefault(q.Get("limit"), 200))
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *FindingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requirePermission(w, r, "findings.view"); !ok {
		return
	}
	id := parseInt64Default(pathParams(r)["id"], 0)
	if id <= 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	item, err := h.store.GetFinding(r.Context(), id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if item == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *FindingsHandler) Create(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "findings.manage")
	if !ok {
		return
	}
	var payload store.Finding
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		http.Error(w, "findings.titleRequired", http.StatusBadRequest)
		return
	}
	status := strings.ToLower(strings.TrimSpace(payload.Status))
	if status == "" {
		status = "open"
	}
	if _, ok := validFindingStatus[status]; !ok {
		http.Error(w, "findings.statusInvalid", http.StatusBadRequest)
		return
	}
	severity := strings.ToLower(strings.TrimSpace(payload.Severity))
	if severity == "" {
		severity = "medium"
	}
	if _, ok := validFindingSeverity[severity]; !ok {
		http.Error(w, "findings.severityInvalid", http.StatusBadRequest)
		return
	}
	ft := strings.ToLower(strings.TrimSpace(payload.FindingType))
	if ft == "" {
		ft = "other"
	}
	if _, ok := validFindingType[ft]; !ok {
		http.Error(w, "findings.typeInvalid", http.StatusBadRequest)
		return
	}
	now := time.Now().UTC()
	item := &store.Finding{
		Title:         title,
		DescriptionMD: strings.TrimSpace(payload.DescriptionMD),
		Status:        status,
		Severity:      severity,
		FindingType:   ft,
		Owner:         strings.TrimSpace(payload.Owner),
		DueAt:         payload.DueAt,
		Tags:          payload.Tags,
		CreatedBy:     &sess.UserID,
		UpdatedBy:     &sess.UserID,
		CreatedAt:     now,
		UpdatedAt:     now,
		Version:       1,
	}
	id, err := h.store.CreateFinding(r.Context(), item)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	item.ID = id
	h.audit(r, findingAuditCreate, strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusCreated, item)
}

func (h *FindingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "findings.manage")
	if !ok {
		return
	}
	id := parseInt64Default(pathParams(r)["id"], 0)
	if id <= 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	existing, err := h.store.GetFinding(r.Context(), id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var payload store.Finding
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		http.Error(w, "findings.titleRequired", http.StatusBadRequest)
		return
	}
	status := strings.ToLower(strings.TrimSpace(payload.Status))
	if status == "" {
		status = "open"
	}
	if _, ok := validFindingStatus[status]; !ok {
		http.Error(w, "findings.statusInvalid", http.StatusBadRequest)
		return
	}
	severity := strings.ToLower(strings.TrimSpace(payload.Severity))
	if severity == "" {
		severity = "medium"
	}
	if _, ok := validFindingSeverity[severity]; !ok {
		http.Error(w, "findings.severityInvalid", http.StatusBadRequest)
		return
	}
	ft := strings.ToLower(strings.TrimSpace(payload.FindingType))
	if ft == "" {
		ft = "other"
	}
	if _, ok := validFindingType[ft]; !ok {
		http.Error(w, "findings.typeInvalid", http.StatusBadRequest)
		return
	}
	expectedVersion := payload.Version
	if expectedVersion == 0 {
		http.Error(w, "findings.versionRequired", http.StatusBadRequest)
		return
	}
	updated := *existing
	updated.Title = title
	updated.DescriptionMD = strings.TrimSpace(payload.DescriptionMD)
	updated.Status = status
	updated.Severity = severity
	updated.FindingType = ft
	updated.Owner = strings.TrimSpace(payload.Owner)
	updated.DueAt = payload.DueAt
	updated.Tags = payload.Tags
	updated.UpdatedBy = &sess.UserID
	updated.Version = expectedVersion
	if err := h.store.UpdateFinding(r.Context(), &updated); err != nil {
		if err == store.ErrConflict {
			http.Error(w, "findings.conflict", http.StatusConflict)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.audit(r, findingAuditUpdate, strconv.FormatInt(id, 10))
	item, _ := h.store.GetFinding(r.Context(), id)
	writeJSON(w, http.StatusOK, item)
}

func (h *FindingsHandler) Archive(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "findings.manage")
	if !ok {
		return
	}
	id := parseInt64Default(pathParams(r)["id"], 0)
	if id <= 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.store.ArchiveFinding(r.Context(), id, sess.UserID); err != nil {
		if err == store.ErrConflict {
			http.Error(w, "findings.conflict", http.StatusConflict)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.audit(r, findingAuditArchive, strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *FindingsHandler) Restore(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "findings.manage")
	if !ok {
		return
	}
	id := parseInt64Default(pathParams(r)["id"], 0)
	if id <= 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.store.RestoreFinding(r.Context(), id, sess.UserID); err != nil {
		if err == store.ErrConflict {
			http.Error(w, "findings.conflict", http.StatusConflict)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.audit(r, findingAuditRestore, strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func parseBool(raw string) bool {
	val := strings.ToLower(strings.TrimSpace(raw))
	return val == "1" || val == "true" || val == "yes"
}

func (h *FindingsHandler) requirePermission(w http.ResponseWriter, r *http.Request, perm rbac.Permission) (*store.SessionRecord, bool) {
	val := r.Context().Value(auth.SessionContextKey)
	if val == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	sess := val.(*store.SessionRecord)
	if h.policy != nil && !h.policy.Allowed(sess.Roles, perm) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return nil, false
	}
	return sess, true
}
