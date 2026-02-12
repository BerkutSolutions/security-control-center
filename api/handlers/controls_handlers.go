package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/auth"
	"berkut-scc/core/controls"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
	"berkut-scc/tasks"
	"github.com/gorilla/mux"
)

type ControlsHandler struct {
	store     store.ControlsStore
	links     store.EntityLinksStore
	users     store.UsersStore
	docs      store.DocsStore
	incidents store.IncidentsStore
	tasks     tasks.Store
	audits    store.AuditStore
	policy    *rbac.Policy
	logger    *utils.Logger
}

func NewControlsHandler(cs store.ControlsStore, links store.EntityLinksStore, us store.UsersStore, ds store.DocsStore, is store.IncidentsStore, ts tasks.Store, audits store.AuditStore, policy *rbac.Policy, logger *utils.Logger) *ControlsHandler {
	return &ControlsHandler{store: cs, links: links, users: us, docs: ds, incidents: is, tasks: ts, audits: audits, policy: policy, logger: logger}
}

func (h *ControlsHandler) ListControls(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.view")
	if !ok {
		return
	}
	if _, err := h.userFromSession(r.Context(), sess); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	q := r.URL.Query()
	filter := store.ControlFilter{
		Search:    q.Get("q"),
		Status:    strings.ToLower(strings.TrimSpace(q.Get("status"))),
		RiskLevel: strings.ToLower(strings.TrimSpace(q.Get("risk"))),
		Domain:    strings.ToLower(strings.TrimSpace(q.Get("domain"))),
		Tag:       q.Get("tag"),
	}
	if raw := strings.TrimSpace(q.Get("owner")); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			filter.OwnerUserID = &id
		}
	}
	if raw := strings.TrimSpace(q.Get("active")); raw != "" {
		val := strings.ToLower(raw)
		active := val == "1" || val == "true" || val == "yes"
		if val == "0" || val == "false" || val == "no" {
			active = false
		}
		filter.Active = &active
	}
	items, err := h.store.ListControls(r.Context(), filter)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *ControlsHandler) CreateControl(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.manage")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var payload struct {
		Code            string   `json:"code"`
		Title           string   `json:"title"`
		DescriptionMD   string   `json:"description_md"`
		ControlType     string   `json:"control_type"`
		Domain          string   `json:"domain"`
		OwnerUserID     *int64   `json:"owner_user_id"`
		ReviewFrequency string   `json:"review_frequency"`
		Status          string   `json:"status"`
		RiskLevel       string   `json:"risk_level"`
		Tags            []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	code := strings.TrimSpace(payload.Code)
	title := strings.TrimSpace(payload.Title)
	if code == "" || title == "" {
		http.Error(w, "controls.error.required", http.StatusBadRequest)
		return
	}
	controlType := strings.TrimSpace(payload.ControlType)
	if controlType == "" {
		http.Error(w, "controls.error.controlTypeInvalid", http.StatusBadRequest)
		return
	}
	controlTypeItem, err := h.store.GetControlTypeByName(r.Context(), controlType)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if controlTypeItem == nil {
		http.Error(w, "controls.error.controlTypeInvalid", http.StatusBadRequest)
		return
	}
	status, ok := controls.NormalizeInList(payload.Status, controls.Statuses)
	if !ok {
		http.Error(w, "controls.error.statusInvalid", http.StatusBadRequest)
		return
	}
	risk, ok := controls.NormalizeInList(payload.RiskLevel, controls.RiskLevels)
	if !ok {
		http.Error(w, "controls.error.riskInvalid", http.StatusBadRequest)
		return
	}
	freq, ok := controls.NormalizeInList(payload.ReviewFrequency, controls.Frequencies)
	if !ok {
		http.Error(w, "controls.error.frequencyInvalid", http.StatusBadRequest)
		return
	}
	domain := strings.ToLower(strings.TrimSpace(payload.Domain))
	if domain == "" {
		domain = "other"
	}
	var owner *int64
	if payload.OwnerUserID != nil && *payload.OwnerUserID > 0 {
		owner = payload.OwnerUserID
	}
	control := &store.Control{
		Code:            code,
		Title:           title,
		DescriptionMD:   strings.TrimSpace(payload.DescriptionMD),
		ControlType:     controlTypeItem.Name,
		Domain:          domain,
		OwnerUserID:     owner,
		ReviewFrequency: freq,
		Status:          status,
		RiskLevel:       risk,
		Tags:            payload.Tags,
		CreatedBy:       user.ID,
		IsActive:        true,
	}
	if _, err := h.store.CreateControl(r.Context(), control); err != nil {
		if isUniqueViolation(err) {
			http.Error(w, "controls.error.codeExists", http.StatusBadRequest)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.logAudit(r.Context(), user.Username, "control.create", control.Code)
	writeJSON(w, http.StatusCreated, control)
}

func (h *ControlsHandler) GetControl(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.view")
	if !ok {
		return
	}
	if _, err := h.userFromSession(r.Context(), sess); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := parseInt64Default(mux.Vars(r)["id"], 0)
	if id == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	control, err := h.store.GetControl(r.Context(), id)
	if err != nil || control == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, control)
}

func (h *ControlsHandler) UpdateControl(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.manage")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := parseInt64Default(mux.Vars(r)["id"], 0)
	if id == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	existing, err := h.store.GetControl(r.Context(), id)
	if err != nil || existing == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var payload struct {
		Code            string   `json:"code"`
		Title           string   `json:"title"`
		DescriptionMD   string   `json:"description_md"`
		ControlType     string   `json:"control_type"`
		Domain          string   `json:"domain"`
		OwnerUserID     *int64   `json:"owner_user_id"`
		ReviewFrequency string   `json:"review_frequency"`
		Status          string   `json:"status"`
		RiskLevel       string   `json:"risk_level"`
		Tags            []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	code := strings.TrimSpace(payload.Code)
	title := strings.TrimSpace(payload.Title)
	if code == "" || title == "" {
		http.Error(w, "controls.error.required", http.StatusBadRequest)
		return
	}
	controlType := strings.TrimSpace(payload.ControlType)
	if controlType == "" {
		http.Error(w, "controls.error.controlTypeInvalid", http.StatusBadRequest)
		return
	}
	controlTypeItem, err := h.store.GetControlTypeByName(r.Context(), controlType)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if controlTypeItem == nil {
		http.Error(w, "controls.error.controlTypeInvalid", http.StatusBadRequest)
		return
	}
	status, ok := controls.NormalizeInList(payload.Status, controls.Statuses)
	if !ok {
		http.Error(w, "controls.error.statusInvalid", http.StatusBadRequest)
		return
	}
	risk, ok := controls.NormalizeInList(payload.RiskLevel, controls.RiskLevels)
	if !ok {
		http.Error(w, "controls.error.riskInvalid", http.StatusBadRequest)
		return
	}
	freq, ok := controls.NormalizeInList(payload.ReviewFrequency, controls.Frequencies)
	if !ok {
		http.Error(w, "controls.error.frequencyInvalid", http.StatusBadRequest)
		return
	}
	domain := strings.ToLower(strings.TrimSpace(payload.Domain))
	if domain == "" {
		domain = "other"
	}
	var owner *int64
	if payload.OwnerUserID != nil && *payload.OwnerUserID > 0 {
		owner = payload.OwnerUserID
	}
	existing.Code = code
	existing.Title = title
	existing.DescriptionMD = strings.TrimSpace(payload.DescriptionMD)
	existing.ControlType = controlTypeItem.Name
	existing.Domain = domain
	existing.OwnerUserID = owner
	existing.ReviewFrequency = freq
	existing.Status = status
	existing.RiskLevel = risk
	existing.Tags = payload.Tags
	if err := h.store.UpdateControl(r.Context(), existing); err != nil {
		if isUniqueViolation(err) {
			http.Error(w, "controls.error.codeExists", http.StatusBadRequest)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.logAudit(r.Context(), user.Username, "control.update", existing.Code)
	writeJSON(w, http.StatusOK, existing)
}

func (h *ControlsHandler) DeleteControl(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.manage")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := parseInt64Default(mux.Vars(r)["id"], 0)
	if id == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	control, err := h.store.GetControl(r.Context(), id)
	if err != nil || control == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := h.store.SoftDeleteControl(r.Context(), id); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.logAudit(r.Context(), user.Username, "control.delete", control.Code)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ControlsHandler) ListControlLinks(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.view")
	if !ok {
		return
	}
	if _, err := h.userFromSession(r.Context(), sess); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	controlID := parseInt64Default(mux.Vars(r)["id"], 0)
	if controlID == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	control, err := h.store.GetControl(r.Context(), controlID)
	if err != nil || control == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	links, err := h.links.ListBySource(r.Context(), "control", strconv.FormatInt(control.ID, 10))
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	type linkView struct {
		ID           int64  `json:"id"`
		TargetType   string `json:"target_type"`
		TargetID     string `json:"target_id"`
		RelationType string `json:"relation_type"`
		TargetTitle  string `json:"target_title,omitempty"`
	}
	items := make([]linkView, 0, len(links))
	for _, l := range links {
		view := linkView{
			ID:           l.ID,
			TargetType:   l.TargetType,
			TargetID:     l.TargetID,
			RelationType: l.RelationType,
		}
		if targetTitle := h.resolveTargetTitle(r.Context(), l.TargetType, l.TargetID); targetTitle != "" {
			view.TargetTitle = targetTitle
		}
		items = append(items, view)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *ControlsHandler) CreateControlLink(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.manage")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	controlID := parseInt64Default(mux.Vars(r)["id"], 0)
	if controlID == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	control, err := h.store.GetControl(r.Context(), controlID)
	if err != nil || control == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var payload struct {
		TargetType   string `json:"target_type"`
		TargetID     string `json:"target_id"`
		Type         string `json:"type"`
		RelationType string `json:"relation_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	targetType := strings.ToLower(strings.TrimSpace(payload.TargetType))
	if targetType == "" {
		targetType = strings.ToLower(strings.TrimSpace(payload.Type))
	}
	targetID := strings.TrimSpace(payload.TargetID)
	if targetType == "" || targetID == "" {
		http.Error(w, "controls.links.required", http.StatusBadRequest)
		return
	}
	relationType, ok := normalizeRelationType(payload.RelationType)
	if !ok {
		http.Error(w, "controls.links.relationInvalid", http.StatusBadRequest)
		return
	}
	if err := h.validateLinkTarget(r.Context(), targetType, targetID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	link := &store.EntityLink{
		SourceType:   "control",
		SourceID:     strconv.FormatInt(controlID, 10),
		TargetType:   targetType,
		TargetID:     targetID,
		RelationType: relationType,
	}
	if _, err := h.links.Add(r.Context(), link); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if relationType == "violates" && targetType == "incident" {
		h.logAudit(r.Context(), user.Username, "link.violates.add", control.Code+"|"+targetID)
		if h.policy != nil && h.policy.Allowed(sess.Roles, "controls.violations.manage") {
			h.autoCreateViolationFromIncident(r.Context(), user, control, targetID)
		}
	}
	h.logAudit(r.Context(), user.Username, "control.link.add", control.Code+"|"+targetType+"|"+targetID)
	writeJSON(w, http.StatusCreated, link)
}

func (h *ControlsHandler) DeleteControlLink(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.manage")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	controlID := parseInt64Default(mux.Vars(r)["id"], 0)
	if controlID == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	control, err := h.store.GetControl(r.Context(), controlID)
	if err != nil || control == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	linkID := parseInt64Default(mux.Vars(r)["link_id"], 0)
	if linkID == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	links, err := h.links.ListBySource(r.Context(), "control", strconv.FormatInt(controlID, 10))
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	var target *store.EntityLink
	for i := range links {
		if links[i].ID == linkID {
			target = &links[i]
			break
		}
	}
	if target == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	hasOtherViolatesLink := false
	if target.RelationType == "violates" && target.TargetType == "incident" {
		for i := range links {
			if links[i].ID == target.ID {
				continue
			}
			if links[i].RelationType == "violates" && links[i].TargetType == "incident" && links[i].TargetID == target.TargetID {
				hasOtherViolatesLink = true
				break
			}
		}
	}
	if err := h.links.Delete(r.Context(), linkID); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if target.RelationType == "violates" && target.TargetType == "incident" {
		h.logAudit(r.Context(), user.Username, "link.violates.remove", control.Code+"|"+target.TargetID)
		if !hasOtherViolatesLink && h.policy != nil && h.policy.Allowed(sess.Roles, "controls.violations.manage") {
			h.autoDisableViolation(r.Context(), user.Username, control.ID, target.TargetID)
		}
	}
	h.logAudit(r.Context(), user.Username, "control.link.remove", control.Code+"|"+target.TargetType+"|"+target.TargetID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ControlsHandler) ListControlChecks(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.checks.view")
	if !ok {
		return
	}
	if _, err := h.userFromSession(r.Context(), sess); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	controlID := parseInt64Default(mux.Vars(r)["id"], 0)
	if controlID == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if c, _ := h.store.GetControl(r.Context(), controlID); c == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	items, err := h.store.ListControlChecks(r.Context(), controlID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *ControlsHandler) CreateControlCheck(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.checks.manage")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	controlID := parseInt64Default(mux.Vars(r)["id"], 0)
	if controlID == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	control, err := h.store.GetControl(r.Context(), controlID)
	if err != nil || control == nil || !control.IsActive {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var payload struct {
		CheckedAt     string   `json:"checked_at"`
		Result        string   `json:"result"`
		NotesMD       string   `json:"notes_md"`
		EvidenceLinks []string `json:"evidence_links"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	result, ok := controls.NormalizeInList(payload.Result, controls.CheckResults)
	if !ok {
		http.Error(w, "controls.error.checkResultInvalid", http.StatusBadRequest)
		return
	}
	checkedAt := time.Now().UTC()
	if payload.CheckedAt != "" {
		if parsed, err := parseTime(payload.CheckedAt); err == nil && parsed != nil {
			checkedAt = parsed.UTC()
		} else {
			http.Error(w, "controls.error.dateInvalid", http.StatusBadRequest)
			return
		}
	}
	check := &store.ControlCheck{
		ControlID:     controlID,
		CheckedAt:     checkedAt,
		CheckedBy:     user.ID,
		Result:        result,
		NotesMD:       strings.TrimSpace(payload.NotesMD),
		EvidenceLinks: payload.EvidenceLinks,
	}
	if _, err := h.store.CreateControlCheck(r.Context(), check); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	check.ControlCode = control.Code
	check.ControlTitle = control.Title
	h.logAudit(r.Context(), user.Username, "control.check.create", control.Code)
	writeJSON(w, http.StatusCreated, check)
}

func (h *ControlsHandler) ListChecks(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.checks.view")
	if !ok {
		return
	}
	if _, err := h.userFromSession(r.Context(), sess); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	q := r.URL.Query()
	filter := store.ControlCheckFilter{
		Result: strings.ToLower(strings.TrimSpace(q.Get("result"))),
	}
	if raw := strings.TrimSpace(q.Get("control_id")); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			filter.ControlID = id
		}
	}
	if raw := strings.TrimSpace(q.Get("owner")); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			filter.CheckedBy = &id
		}
	}
	if raw := strings.TrimSpace(q.Get("date_from")); raw != "" {
		parsed, err := parseTime(raw)
		if err != nil || parsed == nil {
			http.Error(w, "controls.error.dateInvalid", http.StatusBadRequest)
			return
		}
		filter.DateFrom = parsed
	}
	if raw := strings.TrimSpace(q.Get("date_to")); raw != "" {
		parsed, err := parseTime(raw)
		if err != nil || parsed == nil {
			http.Error(w, "controls.error.dateInvalid", http.StatusBadRequest)
			return
		}
		filter.DateTo = parsed
	}
	items, err := h.store.ListChecks(r.Context(), filter)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *ControlsHandler) DeleteCheck(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.checks.manage")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := parseInt64Default(mux.Vars(r)["id"], 0)
	if id == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	check, err := h.store.GetControlCheck(r.Context(), id)
	if err != nil || check == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := h.store.DeleteControlCheck(r.Context(), id); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.logAudit(r.Context(), user.Username, "control.check.delete", strconv.FormatInt(check.ID, 10))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ControlsHandler) ListControlViolations(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.violations.view")
	if !ok {
		return
	}
	if _, err := h.userFromSession(r.Context(), sess); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	controlID := parseInt64Default(mux.Vars(r)["id"], 0)
	if controlID == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if c, _ := h.store.GetControl(r.Context(), controlID); c == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	items, err := h.store.ListControlViolations(r.Context(), controlID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *ControlsHandler) CreateControlViolation(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.violations.manage")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	controlID := parseInt64Default(mux.Vars(r)["id"], 0)
	if controlID == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	control, err := h.store.GetControl(r.Context(), controlID)
	if err != nil || control == nil || !control.IsActive {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var payload struct {
		IncidentID any    `json:"incident_id"`
		HappenedAt string `json:"happened_at"`
		Severity   string `json:"severity"`
		Summary    string `json:"summary"`
		ImpactMD   string `json:"impact_md"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	severity, ok := controls.NormalizeInList(payload.Severity, controls.ViolationSeverities())
	if !ok {
		http.Error(w, "controls.error.severityInvalid", http.StatusBadRequest)
		return
	}
	summary := strings.TrimSpace(payload.Summary)
	if summary == "" {
		http.Error(w, "controls.error.summaryRequired", http.StatusBadRequest)
		return
	}
	happenedAt := time.Now().UTC()
	if payload.HappenedAt != "" {
		if parsed, err := parseTime(payload.HappenedAt); err == nil && parsed != nil {
			happenedAt = parsed.UTC()
		} else {
			http.Error(w, "controls.error.dateInvalid", http.StatusBadRequest)
			return
		}
	}
	var incidentID *int64
	if ref, ok := parseIncidentRef(payload.IncidentID); ok {
		incident, err := h.resolveIncidentByRef(r.Context(), ref)
		if err != nil || incident == nil {
			http.Error(w, "controls.error.incidentNotFound", http.StatusBadRequest)
			return
		}
		incidentID = &incident.ID
	}
	if incidentID != nil {
		if existing, _ := h.store.GetControlViolationByLink(r.Context(), controlID, *incidentID); existing != nil {
			http.Error(w, "controls.error.violationExists", http.StatusBadRequest)
			return
		}
	}
	v := &store.ControlViolation{
		ControlID:  controlID,
		IncidentID: incidentID,
		HappenedAt: happenedAt,
		Severity:   severity,
		Summary:    summary,
		ImpactMD:   strings.TrimSpace(payload.ImpactMD),
		CreatedBy:  user.ID,
		IsAuto:     false,
		IsActive:   true,
	}
	if _, err := h.store.CreateControlViolation(r.Context(), v); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	v.ControlCode = control.Code
	v.ControlTitle = control.Title
	h.logAudit(r.Context(), user.Username, "control.violation.create", control.Code)
	writeJSON(w, http.StatusCreated, v)
}

func (h *ControlsHandler) ListViolations(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.violations.view")
	if !ok {
		return
	}
	if _, err := h.userFromSession(r.Context(), sess); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	q := r.URL.Query()
	filter := store.ControlViolationFilter{
		Severity: strings.ToLower(strings.TrimSpace(q.Get("severity"))),
	}
	if raw := strings.TrimSpace(q.Get("control_id")); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			filter.ControlID = id
		}
	}
	if raw := strings.TrimSpace(q.Get("incident_id")); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			filter.IncidentID = &id
		}
	}
	if raw := strings.TrimSpace(q.Get("date_from")); raw != "" {
		parsed, err := parseTime(raw)
		if err != nil || parsed == nil {
			http.Error(w, "controls.error.dateInvalid", http.StatusBadRequest)
			return
		}
		filter.DateFrom = parsed
	}
	if raw := strings.TrimSpace(q.Get("date_to")); raw != "" {
		parsed, err := parseTime(raw)
		if err != nil || parsed == nil {
			http.Error(w, "controls.error.dateInvalid", http.StatusBadRequest)
			return
		}
		filter.DateTo = parsed
	}
	items, err := h.store.ListViolations(r.Context(), filter)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *ControlsHandler) DeleteViolation(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.violations.manage")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := parseInt64Default(mux.Vars(r)["id"], 0)
	if id == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	violation, err := h.store.GetControlViolation(r.Context(), id)
	if err != nil || violation == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := h.store.SoftDeleteControlViolation(r.Context(), id); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.logAudit(r.Context(), user.Username, "control.violation.delete", strconv.FormatInt(violation.ID, 10))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ControlsHandler) ListFrameworks(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.frameworks.view")
	if !ok {
		return
	}
	if _, err := h.userFromSession(r.Context(), sess); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	items, err := h.store.ListFrameworks(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *ControlsHandler) CreateFramework(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.frameworks.manage")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var payload struct {
		Name     string `json:"name"`
		Version  string `json:"version"`
		IsActive bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		http.Error(w, "controls.error.frameworkNameRequired", http.StatusBadRequest)
		return
	}
	f := &store.ControlFramework{
		Name:     name,
		Version:  strings.TrimSpace(payload.Version),
		IsActive: payload.IsActive,
	}
	if _, err := h.store.CreateFramework(r.Context(), f); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.logAudit(r.Context(), user.Username, "control.framework.create", f.Name)
	writeJSON(w, http.StatusCreated, f)
}

func (h *ControlsHandler) ListFrameworkItems(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.frameworks.view")
	if !ok {
		return
	}
	if _, err := h.userFromSession(r.Context(), sess); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	frameworkID := parseInt64Default(mux.Vars(r)["id"], 0)
	if frameworkID == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	framework, err := h.store.GetFramework(r.Context(), frameworkID)
	if err != nil || framework == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	items, err := h.store.ListFrameworkItems(r.Context(), frameworkID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *ControlsHandler) CreateFrameworkItem(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.frameworks.manage")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	frameworkID := parseInt64Default(mux.Vars(r)["id"], 0)
	if frameworkID == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	framework, err := h.store.GetFramework(r.Context(), frameworkID)
	if err != nil || framework == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var payload struct {
		Code         string `json:"code"`
		Title        string `json:"title"`
		DescriptionMD string `json:"description_md"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	code := strings.TrimSpace(payload.Code)
	title := strings.TrimSpace(payload.Title)
	if code == "" || title == "" {
		http.Error(w, "controls.error.required", http.StatusBadRequest)
		return
	}
	item := &store.ControlFrameworkItem{
		FrameworkID:  frameworkID,
		Code:         code,
		Title:        title,
		DescriptionMD: strings.TrimSpace(payload.DescriptionMD),
	}
	if _, err := h.store.CreateFrameworkItem(r.Context(), item); err != nil {
		if isUniqueViolation(err) {
			http.Error(w, "controls.error.frameworkItemExists", http.StatusBadRequest)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.logAudit(r.Context(), user.Username, "control.framework.item.create", framework.Name+" "+item.Code)
	writeJSON(w, http.StatusCreated, item)
}

func (h *ControlsHandler) CreateFrameworkMap(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.frameworks.manage")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var payload struct {
		FrameworkItemID int64 `json:"framework_item_id"`
		ControlID       int64 `json:"control_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if payload.FrameworkItemID == 0 || payload.ControlID == 0 {
		http.Error(w, "controls.error.required", http.StatusBadRequest)
		return
	}
	item, err := h.store.GetFrameworkItem(r.Context(), payload.FrameworkItemID)
	if err != nil || item == nil {
		http.Error(w, "controls.error.frameworkItemNotFound", http.StatusBadRequest)
		return
	}
	control, err := h.store.GetControl(r.Context(), payload.ControlID)
	if err != nil || control == nil {
		http.Error(w, "controls.error.controlNotFound", http.StatusBadRequest)
		return
	}
	m := &store.ControlFrameworkMap{
		FrameworkItemID: payload.FrameworkItemID,
		ControlID:       payload.ControlID,
	}
	if _, err := h.store.AddFrameworkMap(r.Context(), m); err != nil {
		if isUniqueViolation(err) {
			http.Error(w, "controls.error.frameworkMapExists", http.StatusBadRequest)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.logAudit(r.Context(), user.Username, "control.framework.map.create", control.Code)
	writeJSON(w, http.StatusCreated, m)
}

func (h *ControlsHandler) ListFrameworkMap(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.frameworks.view")
	if !ok {
		return
	}
	if _, err := h.userFromSession(r.Context(), sess); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	frameworkID := parseInt64Default(mux.Vars(r)["id"], 0)
	if frameworkID == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	framework, err := h.store.GetFramework(r.Context(), frameworkID)
	if err != nil || framework == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	items, err := h.store.ListFrameworkMap(r.Context(), frameworkID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *ControlsHandler) requirePermission(w http.ResponseWriter, r *http.Request, perm rbac.Permission) (*store.SessionRecord, bool) {
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

func (h *ControlsHandler) userFromSession(ctx context.Context, sess *store.SessionRecord) (*store.User, error) {
	if sess == nil {
		return nil, errors.New("no session")
	}
	u, _, err := h.users.FindByUsername(ctx, sess.Username)
	return u, err
}

func (h *ControlsHandler) logAudit(ctx context.Context, username, action, details string) {
	if h.audits == nil {
		return
	}
	_ = h.audits.Log(ctx, username, action, details)
}

func normalizeRelationType(raw string) (string, bool) {
	val := strings.ToLower(strings.TrimSpace(raw))
	if val == "" {
		return "related", true
	}
	switch val {
	case "evidence", "implements", "violates", "related":
		return val, true
	default:
		return "", false
	}
}

func parseIncidentRef(raw any) (string, bool) {
	switch v := raw.(type) {
	case float64:
		if v <= 0 {
			return "", false
		}
		return strconv.FormatInt(int64(v), 10), true
	case string:
		ref := strings.TrimSpace(v)
		if ref == "" {
			return "", false
		}
		return ref, true
	default:
		return "", false
	}
}

func (h *ControlsHandler) resolveIncidentByRef(ctx context.Context, raw string) (*store.Incident, error) {
	ref := strings.TrimSpace(raw)
	if ref == "" || h.incidents == nil {
		return nil, errors.New("controls.error.incidentNotFound")
	}
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil && id > 0 {
		inc, err := h.incidents.GetIncident(ctx, id)
		if err != nil || inc == nil || inc.DeletedAt != nil {
			return nil, errors.New("controls.error.incidentNotFound")
		}
		return inc, nil
	}
	inc, err := h.incidents.GetIncidentByRegNo(ctx, ref)
	if err != nil || inc == nil || inc.DeletedAt != nil {
		return nil, errors.New("controls.error.incidentNotFound")
	}
	return inc, nil
}

func (h *ControlsHandler) validateLinkTarget(ctx context.Context, targetType, targetID string) error {
	switch targetType {
	case "doc":
		id, err := strconv.ParseInt(targetID, 10, 64)
		if err != nil || id == 0 {
			return errors.New("controls.links.targetInvalid")
		}
		doc, err := h.docs.GetDocument(ctx, id)
		if err != nil || doc == nil {
			return errors.New("controls.links.targetNotFound")
		}
	case "incident":
		if strings.TrimSpace(targetID) == "" {
			return errors.New("controls.links.targetInvalid")
		}
		inc, err := h.resolveIncidentByRef(ctx, targetID)
		if err != nil || inc == nil || inc.DeletedAt != nil {
			return errors.New("controls.links.targetNotFound")
		}
	case "task":
		id, err := strconv.ParseInt(targetID, 10, 64)
		if err != nil || id == 0 {
			return errors.New("controls.links.targetInvalid")
		}
		task, err := h.tasks.GetTask(ctx, id)
		if err != nil || task == nil {
			return errors.New("controls.links.targetNotFound")
		}
	default:
		return errors.New("controls.links.typeInvalid")
	}
	return nil
}

func (h *ControlsHandler) resolveTargetTitle(ctx context.Context, targetType, targetID string) string {
	switch strings.ToLower(strings.TrimSpace(targetType)) {
	case "doc":
		id, err := strconv.ParseInt(targetID, 10, 64)
		if err != nil || id == 0 {
			return ""
		}
		doc, err := h.docs.GetDocument(ctx, id)
		if err != nil || doc == nil {
			return ""
		}
		return doc.Title
	case "incident":
		if strings.TrimSpace(targetID) == "" {
			return ""
		}
		inc, err := h.resolveIncidentByRef(ctx, targetID)
		if err != nil || inc == nil {
			return ""
		}
		if inc.RegNo != "" {
			return inc.RegNo + " - " + inc.Title
		}
		return inc.Title
	case "task":
		id, err := strconv.ParseInt(targetID, 10, 64)
		if err != nil || id == 0 {
			return ""
		}
		task, err := h.tasks.GetTask(ctx, id)
		if err != nil || task == nil {
			return ""
		}
		return task.Title
	default:
		return ""
	}
}

func (h *ControlsHandler) autoCreateViolationFromIncident(ctx context.Context, user *store.User, control *store.Control, incidentIDRaw string) {
	if h.incidents == nil || control == nil || user == nil {
		return
	}
	incident, err := h.resolveIncidentByRef(ctx, incidentIDRaw)
	if err != nil || incident == nil || incident.DeletedAt != nil {
		return
	}
	incidentID := incident.ID
	existing, err := h.store.GetControlViolationByLink(ctx, control.ID, incidentID)
	if err != nil {
		return
	}
	if existing != nil && !existing.IsAuto {
		return
	}
	happenedAt := incident.CreatedAt
	if incident.Meta.DetectedAt != "" {
		if parsed, err := parseTime(incident.Meta.DetectedAt); err == nil && parsed != nil {
			happenedAt = parsed.UTC()
		}
	}
	severity, ok := controls.NormalizeInList(incident.Severity, controls.ViolationSeverities())
	if !ok {
		severity = "medium"
	}
	summary := strings.TrimSpace(incident.Title)
	impact := strings.TrimSpace(incident.Description)
	if impact == "" {
		impact = strings.TrimSpace(incident.Meta.WhatHappened)
	}
	if existing != nil {
		existing.HappenedAt = happenedAt
		existing.Severity = severity
		existing.Summary = summary
		existing.ImpactMD = impact
		existing.IsAuto = true
		existing.IsActive = true
		_ = h.store.UpdateControlViolationAuto(ctx, existing)
		h.logAudit(ctx, user.Username, "violation.auto_create", control.Code+"|"+strconv.FormatInt(incidentID, 10))
		return
	}
	v := &store.ControlViolation{
		ControlID:  control.ID,
		IncidentID: &incidentID,
		HappenedAt: happenedAt,
		Severity:   severity,
		Summary:    summary,
		ImpactMD:   impact,
		CreatedBy:  user.ID,
		IsAuto:     true,
		IsActive:   true,
	}
	if _, err := h.store.CreateControlViolation(ctx, v); err != nil {
		return
	}
	h.logAudit(ctx, user.Username, "violation.auto_create", control.Code+"|"+strconv.FormatInt(incidentID, 10))
}

func (h *ControlsHandler) autoDisableViolation(ctx context.Context, username string, controlID int64, incidentIDRaw string) {
	if controlID == 0 {
		return
	}
	incidentID, err := strconv.ParseInt(strings.TrimSpace(incidentIDRaw), 10, 64)
	if err != nil || incidentID == 0 {
		return
	}
	existing, err := h.store.GetControlViolationByLink(ctx, controlID, incidentID)
	if err != nil || existing == nil || !existing.IsAuto || !existing.IsActive {
		return
	}
	existing.IsActive = false
	_ = h.store.UpdateControlViolationAuto(ctx, existing)
	h.logAudit(ctx, username, "violation.auto_disable", strconv.FormatInt(controlID, 10)+"|"+strconv.FormatInt(incidentID, 10))
}

func parseTime(raw string) (*time.Time, error) {
	val := strings.TrimSpace(raw)
	if val == "" {
		return nil, nil
	}
	if ts, err := time.Parse(time.RFC3339, val); err == nil {
		return &ts, nil
	}
	if ts, err := time.Parse("2006-01-02", val); err == nil {
		return &ts, nil
	}
	return nil, errors.New("invalid time")
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") || strings.Contains(msg, "unique")
}

func parseInt64Default(raw string, def int64) int64 {
	if raw == "" {
		return def
	}
	val, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return def
	}
	return val
}
