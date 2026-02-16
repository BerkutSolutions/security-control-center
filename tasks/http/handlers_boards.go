package taskshttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"berkut-scc/tasks"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) ListBoards(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	filter := tasks.BoardFilter{}
	filter.SpaceID = parseInt64Default(r.URL.Query().Get("space_id"), 0)
	if r.URL.Query().Get("include_inactive") == "1" {
		filter.IncludeInactive = true
	}
	items, err := h.svc.Store().ListBoards(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	var res []tasks.Board
	spaceACL := map[int64][]tasks.ACLRule{}
	for _, b := range items {
		if _, ok := spaceACL[b.SpaceID]; !ok && b.SpaceID > 0 {
			acl, _ := h.svc.Store().GetSpaceACL(r.Context(), b.SpaceID)
			spaceACL[b.SpaceID] = acl
		}
		boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), b.ID)
		if !boardAllowed(user, roles, groups, spaceACL[b.SpaceID], boardACL, "view") {
			continue
		}
		res = append(res, b)
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": res})
}

func (h *Handler) CreateBoard(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	var payload struct {
		SpaceID       int64          `json:"space_id"`
		Name           string         `json:"name"`
		Description    string         `json:"description"`
		OrganizationID string         `json:"organization_id"`
		DefaultTemplateID *int64      `json:"default_template_id"`
		ACL            []tasks.ACLRule `json:"acl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		respondError(w, http.StatusBadRequest, "tasks.boardNameRequired")
		return
	}
	spaceID := payload.SpaceID
	if spaceID <= 0 {
		respondError(w, http.StatusBadRequest, "tasks.spaceRequired")
		return
	}
	space, err := h.svc.Store().GetSpace(r.Context(), spaceID)
	if err != nil || space == nil || !space.IsActive {
		respondError(w, http.StatusBadRequest, "tasks.spaceNotFound")
		return
	}
	spaceACL, _ := h.svc.Store().GetSpaceACL(r.Context(), spaceID)
	if !boardAllowed(user, roles, groups, spaceACL, nil, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	board := &tasks.Board{
		SpaceID:        spaceID,
		Name:           name,
		Description:    strings.TrimSpace(payload.Description),
		OrganizationID: strings.TrimSpace(payload.OrganizationID),
		CreatedBy:      &user.ID,
		IsActive:       true,
	}
	if payload.DefaultTemplateID != nil && *payload.DefaultTemplateID != 0 {
		respondError(w, http.StatusBadRequest, "tasks.defaultTemplateInvalid")
		return
	}
	acl := payload.ACL
	if len(acl) == 0 {
		acl = []tasks.ACLRule{{SubjectType: "user", SubjectID: user.Username, Permission: "manage"}}
	}
	if _, err := h.svc.Store().CreateBoard(r.Context(), board, acl); err != nil {
		if errors.Is(err, tasks.ErrInvalidInput) {
			respondError(w, http.StatusBadRequest, "tasks.spaceRequired")
			return
		}
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditBoardCreate, fmt.Sprintf("%d", board.ID))
	respondJSON(w, http.StatusCreated, board)
}

func (h *Handler) UpdateBoard(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	boardID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if boardID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	board, err := h.svc.Store().GetBoard(r.Context(), boardID)
	if err != nil || board == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	spaceACL, _ := h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), board.ID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	var payload struct {
		Name           *string         `json:"name"`
		Description    *string         `json:"description"`
		OrganizationID *string         `json:"organization_id"`
		Position       *int            `json:"position"`
		IsActive       *bool           `json:"is_active"`
		DefaultTemplateID *int64       `json:"default_template_id"`
		ACL            []tasks.ACLRule `json:"acl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	if payload.Name != nil {
		board.Name = strings.TrimSpace(*payload.Name)
	}
	if payload.Description != nil {
		board.Description = strings.TrimSpace(*payload.Description)
	}
	if payload.OrganizationID != nil {
		board.OrganizationID = strings.TrimSpace(*payload.OrganizationID)
	}
	if payload.Position != nil {
		board.Position = *payload.Position
	}
	if payload.IsActive != nil {
		board.IsActive = *payload.IsActive
	}
	prevDefault := board.DefaultTemplateID
	if payload.DefaultTemplateID != nil {
		nextDefault, err := h.resolveBoardDefaultTemplate(r.Context(), board.ID, *payload.DefaultTemplateID)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		board.DefaultTemplateID = nextDefault
	}
	if strings.TrimSpace(board.Name) == "" {
		respondError(w, http.StatusBadRequest, "tasks.boardNameRequired")
		return
	}
	if err := h.svc.Store().UpdateBoard(r.Context(), board); err != nil {
		if errors.Is(err, tasks.ErrInvalidInput) {
			respondError(w, http.StatusBadRequest, "tasks.spaceRequired")
			return
		}
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	if payload.ACL != nil {
		if err := h.svc.Store().SetBoardACL(r.Context(), board.ID, payload.ACL); err != nil {
			respondError(w, http.StatusInternalServerError, "server error")
			return
		}
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditBoardUpdate, fmt.Sprintf("%d", board.ID))
	if payload.DefaultTemplateID != nil && !sameTemplateID(prevDefault, board.DefaultTemplateID) {
		tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditBoardTemplateDefault, fmt.Sprintf("%d|%v", board.ID, board.DefaultTemplateID))
	}
	respondJSON(w, http.StatusOK, board)
}

func (h *Handler) resolveBoardDefaultTemplate(ctx context.Context, boardID int64, templateID int64) (*int64, error) {
	if templateID <= 0 {
		return nil, nil
	}
	tpl, err := h.svc.Store().GetTaskTemplate(ctx, templateID)
	if err != nil || tpl == nil {
		return nil, fmt.Errorf("tasks.templateNotFound")
	}
	if !tpl.IsActive {
		return nil, fmt.Errorf("tasks.templateInactive")
	}
	if tpl.BoardID != boardID {
		return nil, fmt.Errorf("tasks.boardMismatch")
	}
	return &tpl.ID, nil
}

func (h *Handler) DeleteBoard(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	boardID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if boardID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	board, err := h.svc.Store().GetBoard(r.Context(), boardID)
	if err != nil || board == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	spaceACL, _ := h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), board.ID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	if err := h.svc.Store().DeleteBoard(r.Context(), boardID); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditBoardDelete, fmt.Sprintf("%d", board.ID))
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) MoveBoard(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	boardID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if boardID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	board, err := h.svc.Store().GetBoard(r.Context(), boardID)
	if err != nil || board == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	spaceACL, _ := h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), board.ID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	var payload struct {
		Position int `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	moved, err := h.svc.Store().MoveBoard(r.Context(), boardID, payload.Position)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditBoardMove, fmt.Sprintf("%d|%d", boardID, payload.Position))
	respondJSON(w, http.StatusOK, moved)
}
