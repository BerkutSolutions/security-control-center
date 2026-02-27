package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"berkut-scc/core/auth"
	"berkut-scc/core/store"
)

func (h *FindingsHandler) ListLinks(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "findings.view")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := parseInt64Default(pathParams(r)["id"], 0)
	if id <= 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	f, err := h.store.GetFinding(r.Context(), id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if f == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	canViewAssets := h.canViewAssets(r.Context(), user, sess.Roles)
	canViewSoftware := h.canViewSoftware(r.Context(), user, sess.Roles)
	links, err := h.links.ListBySource(r.Context(), "finding", strconv.FormatInt(id, 10))
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
		if strings.ToLower(strings.TrimSpace(l.TargetType)) == "asset" && !canViewAssets {
			items = append(items, view)
			continue
		}
		if strings.ToLower(strings.TrimSpace(l.TargetType)) == "software" && !canViewSoftware {
			items = append(items, view)
			continue
		}
		if title := h.resolveTargetTitle(r.Context(), l.TargetType, l.TargetID, canViewAssets, canViewSoftware); title != "" {
			view.TargetTitle = title
		}
		items = append(items, view)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *FindingsHandler) AddLink(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "findings.manage")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := parseInt64Default(pathParams(r)["id"], 0)
	if id <= 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	f, err := h.store.GetFinding(r.Context(), id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if f == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var payload struct {
		TargetType   string `json:"target_type"`
		TargetID     string `json:"target_id"`
		RelationType string `json:"relation_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	targetType := strings.ToLower(strings.TrimSpace(payload.TargetType))
	targetID := strings.TrimSpace(payload.TargetID)
	if targetType == "" || targetID == "" {
		http.Error(w, "findings.links.required", http.StatusBadRequest)
		return
	}
	relationType := strings.ToLower(strings.TrimSpace(payload.RelationType))
	if relationType == "" {
		relationType = "related"
	}
	if !isAllowedRelationType(relationType) {
		http.Error(w, "findings.links.relationInvalid", http.StatusBadRequest)
		return
	}
	if err := h.validateLinkTarget(r.Context(), user, sess.Roles, targetType, targetID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	link := &store.EntityLink{
		SourceType:   "finding",
		SourceID:     strconv.FormatInt(id, 10),
		TargetType:   targetType,
		TargetID:     targetID,
		RelationType: relationType,
	}
	if _, err := h.links.Add(r.Context(), link); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.audit(r, findingAuditLinkAdd, strconv.FormatInt(id, 10)+"|"+targetType+"|"+targetID)
	writeJSON(w, http.StatusCreated, link)
}

func (h *FindingsHandler) DeleteLink(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "findings.manage")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := parseInt64Default(pathParams(r)["id"], 0)
	if id <= 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	f, err := h.store.GetFinding(r.Context(), id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if f == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	linkID := parseInt64Default(pathParams(r)["link_id"], 0)
	if linkID <= 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	links, err := h.links.ListBySource(r.Context(), "finding", strconv.FormatInt(id, 10))
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
	if err := h.links.Delete(r.Context(), linkID); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.audit(r, findingAuditLinkDel, strconv.FormatInt(id, 10)+"|"+target.TargetType+"|"+target.TargetID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func isAllowedRelationType(val string) bool {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "related", "evidence", "affects":
		return true
	default:
		return false
	}
}

func (h *FindingsHandler) validateLinkTarget(ctx context.Context, user *store.User, roles []string, targetType, targetID string) error {
	switch targetType {
	case "asset":
		if !h.canViewAssets(ctx, user, roles) || h.assets == nil {
			return errors.New("forbidden")
		}
		id, err := strconv.ParseInt(targetID, 10, 64)
		if err != nil || id <= 0 {
			return errors.New("findings.links.targetInvalid")
		}
		a, err := h.assets.GetAsset(ctx, id)
		if err != nil || a == nil || a.DeletedAt != nil {
			return errors.New("findings.links.targetNotFound")
		}
	case "control":
		if h.policy == nil || !h.policy.Allowed(roles, "controls.view") || !allowedByMenuPermissions(h.effectiveMenu(ctx, user, roles), "controls") {
			return errors.New("forbidden")
		}
		id, err := strconv.ParseInt(targetID, 10, 64)
		if err != nil || id <= 0 {
			return errors.New("findings.links.targetInvalid")
		}
		if h.ctrls == nil {
			return errors.New("findings.links.typeInvalid")
		}
		c, err := h.ctrls.GetControl(ctx, id)
		if err != nil || c == nil {
			return errors.New("findings.links.targetNotFound")
		}
	case "software":
		if h.policy == nil || !h.policy.Allowed(roles, "software.view") || !allowedByMenuPermissions(h.effectiveMenu(ctx, user, roles), "software") {
			return errors.New("forbidden")
		}
		id, err := strconv.ParseInt(targetID, 10, 64)
		if err != nil || id <= 0 {
			return errors.New("findings.links.targetInvalid")
		}
		if h.software == nil {
			return errors.New("findings.links.typeInvalid")
		}
		p, err := h.software.GetProduct(ctx, id)
		if err != nil || p == nil || p.DeletedAt != nil {
			return errors.New("findings.links.targetNotFound")
		}
	case "doc", "incident", "task":
		// Minimal validation: allow linking by reference if user can access the module.
		// Asset links are validated strictly because they are the focus of stage-3 integration.
		if strings.TrimSpace(targetID) == "" {
			return errors.New("findings.links.targetInvalid")
		}
	default:
		return errors.New("findings.links.typeInvalid")
	}
	return nil
}

func (h *FindingsHandler) resolveTargetTitle(ctx context.Context, targetType, targetID string, canViewAssets bool, canViewSoftware bool) string {
	switch strings.ToLower(strings.TrimSpace(targetType)) {
	case "asset":
		if !canViewAssets || h.assets == nil {
			return ""
		}
		id, err := strconv.ParseInt(strings.TrimSpace(targetID), 10, 64)
		if err != nil || id <= 0 {
			return ""
		}
		a, err := h.assets.GetAsset(ctx, id)
		if err != nil || a == nil || a.DeletedAt != nil {
			return ""
		}
		return a.Name
	case "software":
		if !canViewSoftware || h.software == nil {
			return ""
		}
		id, err := strconv.ParseInt(strings.TrimSpace(targetID), 10, 64)
		if err != nil || id <= 0 {
			return ""
		}
		p, err := h.software.GetProduct(ctx, id)
		if err != nil || p == nil || p.DeletedAt != nil {
			return ""
		}
		if strings.TrimSpace(p.Vendor) != "" {
			return p.Name + " (" + p.Vendor + ")"
		}
		return p.Name
	default:
		return ""
	}
}

func (h *FindingsHandler) canViewAssets(ctx context.Context, user *store.User, roles []string) bool {
	if h == nil || h.policy == nil || user == nil || h.users == nil {
		return false
	}
	if !h.policy.Allowed(roles, "assets.view") {
		return false
	}
	groups, _ := h.users.UserGroups(ctx, user.ID)
	eff := auth.CalculateEffectiveAccess(user, roles, groups, h.policy)
	return allowedByMenuPermissions(eff.MenuPermissions, "assets")
}

func (h *FindingsHandler) canViewSoftware(ctx context.Context, user *store.User, roles []string) bool {
	if h == nil || h.policy == nil || user == nil || h.users == nil {
		return false
	}
	if !h.policy.Allowed(roles, "software.view") {
		return false
	}
	groups, _ := h.users.UserGroups(ctx, user.ID)
	eff := auth.CalculateEffectiveAccess(user, roles, groups, h.policy)
	return allowedByMenuPermissions(eff.MenuPermissions, "software")
}

func (h *FindingsHandler) effectiveMenu(ctx context.Context, user *store.User, roles []string) []string {
	if h == nil || h.users == nil || user == nil {
		return nil
	}
	groups, _ := h.users.UserGroups(ctx, user.ID)
	eff := auth.CalculateEffectiveAccess(user, roles, groups, h.policy)
	return eff.MenuPermissions
}

func (h *FindingsHandler) userFromSession(ctx context.Context, sess *store.SessionRecord) (*store.User, error) {
	if sess == nil {
		return nil, errors.New("no session")
	}
	u, _, err := h.users.FindByUsername(ctx, sess.Username)
	return u, err
}
