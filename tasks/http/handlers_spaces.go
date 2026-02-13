package taskshttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"berkut-scc/tasks"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) ListSpaces(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	filter := tasks.SpaceFilter{}
	if r.URL.Query().Get("include_inactive") == "1" {
		filter.IncludeInactive = true
	}
	items, err := h.svc.Store().ListSpaces(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	var res []tasks.Space
	for _, sp := range items {
		acl, _ := h.svc.Store().GetSpaceACL(r.Context(), sp.ID)
		if !aclAllowed(user, roles, groups, acl, "view") {
			continue
		}
		res = append(res, sp)
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": res})
}

func (h *Handler) ListSpaceSummary(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	filter := tasks.SpaceFilter{}
	if r.URL.Query().Get("include_inactive") == "1" {
		filter.IncludeInactive = true
	}
	spaces, err := h.svc.Store().ListSpaces(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	type boardInfo struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		TaskCount int    `json:"task_count"`
	}
	type spaceSummary struct {
		ID         int64       `json:"id"`
		Name       string      `json:"name"`
		BoardCount int         `json:"board_count"`
		TaskCount  int         `json:"task_count"`
		Boards     []boardInfo `json:"boards"`
	}
	var res []spaceSummary
	for _, sp := range spaces {
		spaceACL, _ := h.svc.Store().GetSpaceACL(r.Context(), sp.ID)
		if !aclAllowed(user, roles, groups, spaceACL, "view") {
			continue
		}
		boards, err := h.svc.Store().ListBoards(r.Context(), tasks.BoardFilter{SpaceID: sp.ID})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "server error")
			return
		}
		boardACL := map[int64][]tasks.ACLRule{}
		var allowedBoards []tasks.Board
		var boardIDs []int64
		for _, b := range boards {
			acl, _ := h.svc.Store().GetBoardACL(r.Context(), b.ID)
			boardACL[b.ID] = acl
			if !boardAllowed(user, roles, groups, spaceACL, boardACL[b.ID], "view") {
				continue
			}
			allowedBoards = append(allowedBoards, b)
			boardIDs = append(boardIDs, b.ID)
		}
		counts, err := h.svc.Store().CountTasksByBoard(r.Context(), boardIDs)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "server error")
			return
		}
		summary := spaceSummary{
			ID:         sp.ID,
			Name:       sp.Name,
			BoardCount: len(allowedBoards),
		}
		for _, b := range allowedBoards {
			count := counts[b.ID]
			summary.TaskCount += count
			summary.Boards = append(summary.Boards, boardInfo{
				ID:        b.ID,
				Name:      b.Name,
				TaskCount: count,
			})
		}
		res = append(res, summary)
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": res})
}

func (h *Handler) CreateSpace(w http.ResponseWriter, r *http.Request) {
	user, roles, _, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	var payload struct {
		Name           string         `json:"name"`
		Description    string         `json:"description"`
		OrganizationID string         `json:"organization_id"`
		Layout         string         `json:"layout"`
		ACL            []tasks.ACLRule `json:"acl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		respondError(w, http.StatusBadRequest, "tasks.spaceNameRequired")
		return
	}
	layout := normalizeLayout(payload.Layout)
	space := &tasks.Space{
		Name:           name,
		Description:    strings.TrimSpace(payload.Description),
		OrganizationID: strings.TrimSpace(payload.OrganizationID),
		Layout:         layout,
		CreatedBy:      &user.ID,
		IsActive:       true,
	}
	acl := payload.ACL
	if len(acl) == 0 {
		acl = []tasks.ACLRule{{SubjectType: "user", SubjectID: user.Username, Permission: "manage"}}
	}
	if _, err := h.svc.Store().CreateSpace(r.Context(), space, acl); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditSpaceCreate, fmt.Sprintf("%d", space.ID))
	respondJSON(w, http.StatusCreated, space)
}

func (h *Handler) UpdateSpace(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	spaceID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if spaceID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	space, err := h.svc.Store().GetSpace(r.Context(), spaceID)
	if err != nil || space == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	acl, _ := h.svc.Store().GetSpaceACL(r.Context(), space.ID)
	if !aclAllowed(user, roles, groups, acl, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	var payload struct {
		Name           *string         `json:"name"`
		Description    *string         `json:"description"`
		OrganizationID *string         `json:"organization_id"`
		Layout         *string         `json:"layout"`
		IsActive       *bool           `json:"is_active"`
		ACL            []tasks.ACLRule `json:"acl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	if payload.Name != nil {
		space.Name = strings.TrimSpace(*payload.Name)
	}
	if payload.Description != nil {
		space.Description = strings.TrimSpace(*payload.Description)
	}
	if payload.OrganizationID != nil {
		space.OrganizationID = strings.TrimSpace(*payload.OrganizationID)
	}
	if payload.Layout != nil {
		space.Layout = normalizeLayout(*payload.Layout)
	}
	if payload.IsActive != nil {
		space.IsActive = *payload.IsActive
	}
	if strings.TrimSpace(space.Name) == "" {
		respondError(w, http.StatusBadRequest, "tasks.spaceNameRequired")
		return
	}
	if err := h.svc.Store().UpdateSpace(r.Context(), space); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	if payload.ACL != nil {
		if err := h.svc.Store().SetSpaceACL(r.Context(), space.ID, payload.ACL); err != nil {
			respondError(w, http.StatusInternalServerError, "server error")
			return
		}
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditSpaceUpdate, fmt.Sprintf("%d", space.ID))
	respondJSON(w, http.StatusOK, space)
}

func (h *Handler) DeleteSpace(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	spaceID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if spaceID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	space, err := h.svc.Store().GetSpace(r.Context(), spaceID)
	if err != nil || space == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	acl, _ := h.svc.Store().GetSpaceACL(r.Context(), space.ID)
	if !aclAllowed(user, roles, groups, acl, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	if err := h.svc.Store().DeleteSpace(r.Context(), spaceID); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditSpaceDelete, fmt.Sprintf("%d", spaceID))
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func normalizeLayout(raw string) string {
	val := strings.ToLower(strings.TrimSpace(raw))
	switch val {
	case "column", "col", "vertical":
		return "column"
	default:
		return "row"
	}
}
