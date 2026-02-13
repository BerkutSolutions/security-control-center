package taskshttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"berkut-scc/tasks"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) ListColumns(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	boardID := parseInt64Default(chi.URLParam(r, "board_id"), 0)
	if boardID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	board, _ := h.svc.Store().GetBoard(r.Context(), boardID)
	spaceACL := []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), boardID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "view") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	includeInactive := r.URL.Query().Get("include_inactive") == "1"
	items, err := h.svc.Store().ListColumns(r.Context(), boardID, includeInactive)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) CreateColumn(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	boardID := parseInt64Default(chi.URLParam(r, "board_id"), 0)
	if boardID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	board, _ := h.svc.Store().GetBoard(r.Context(), boardID)
	spaceACL := []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), boardID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	var payload struct {
		Name     string `json:"name"`
		Position int    `json:"position"`
		IsFinal  bool   `json:"is_final"`
		WIPLimit *int   `json:"wip_limit"`
		IsActive *bool  `json:"is_active"`
		DefaultTemplateID *int64 `json:"default_template_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		respondError(w, http.StatusBadRequest, "tasks.columnNameRequired")
		return
	}
	active := true
	if payload.IsActive != nil {
		active = *payload.IsActive
	}
	col := &tasks.Column{
		BoardID:  boardID,
		Name:     name,
		Position: payload.Position,
		IsFinal:  payload.IsFinal,
		WIPLimit: payload.WIPLimit,
		IsActive: active,
	}
	if payload.DefaultTemplateID != nil && *payload.DefaultTemplateID != 0 {
		respondError(w, http.StatusBadRequest, "tasks.defaultTemplateInvalid")
		return
	}
	if _, err := h.svc.Store().CreateColumn(r.Context(), col); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditColumnCreate, fmt.Sprintf("%d", col.ID))
	respondJSON(w, http.StatusCreated, col)
}

func (h *Handler) UpdateColumn(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	columnID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if columnID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	column, err := h.svc.Store().GetColumn(r.Context(), columnID)
	if err != nil || column == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	board, _ := h.svc.Store().GetBoard(r.Context(), column.BoardID)
	spaceACL := []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), column.BoardID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	var payload struct {
		Name     *string `json:"name"`
		Position *int    `json:"position"`
		IsFinal  *bool   `json:"is_final"`
		WIPLimit *int    `json:"wip_limit"`
		IsActive *bool   `json:"is_active"`
		DefaultTemplateID *int64 `json:"default_template_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	if payload.Name != nil {
		column.Name = strings.TrimSpace(*payload.Name)
	}
	if payload.Position != nil {
		column.Position = *payload.Position
	}
	if payload.IsFinal != nil {
		column.IsFinal = *payload.IsFinal
	}
	if payload.WIPLimit != nil {
		column.WIPLimit = payload.WIPLimit
	}
	if payload.IsActive != nil {
		column.IsActive = *payload.IsActive
	}
	prevDefault := column.DefaultTemplateID
	if payload.DefaultTemplateID != nil {
		nextDefault, err := h.resolveColumnDefaultTemplate(r.Context(), column, *payload.DefaultTemplateID)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		column.DefaultTemplateID = nextDefault
	}
	if strings.TrimSpace(column.Name) == "" {
		respondError(w, http.StatusBadRequest, "tasks.columnNameRequired")
		return
	}
	if err := h.svc.Store().UpdateColumn(r.Context(), column); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditColumnUpdate, fmt.Sprintf("%d", column.ID))
	if payload.DefaultTemplateID != nil && !sameTemplateID(prevDefault, column.DefaultTemplateID) {
		tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditColumnTemplateDefault, fmt.Sprintf("%d|%v", column.ID, column.DefaultTemplateID))
	}
	respondJSON(w, http.StatusOK, column)
}

func (h *Handler) resolveColumnDefaultTemplate(ctx context.Context, column *tasks.Column, templateID int64) (*int64, error) {
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
	if tpl.BoardID != column.BoardID {
		return nil, fmt.Errorf("tasks.boardMismatch")
	}
	if tpl.ColumnID != column.ID {
		return nil, fmt.Errorf("tasks.columnMismatch")
	}
	return &tpl.ID, nil
}

func (h *Handler) DeleteColumn(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	columnID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if columnID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	column, err := h.svc.Store().GetColumn(r.Context(), columnID)
	if err != nil || column == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	board, _ := h.svc.Store().GetBoard(r.Context(), column.BoardID)
	spaceACL := []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), column.BoardID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	if err := h.svc.Store().DeleteColumn(r.Context(), columnID); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditColumnDelete, fmt.Sprintf("%d", column.ID))
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) MoveColumn(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	columnID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if columnID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	column, err := h.svc.Store().GetColumn(r.Context(), columnID)
	if err != nil || column == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	board, _ := h.svc.Store().GetBoard(r.Context(), column.BoardID)
	spaceACL := []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), column.BoardID)
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
	moved, err := h.svc.Store().MoveColumn(r.Context(), columnID, payload.Position)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditColumnMove, fmt.Sprintf("%d|%d", columnID, payload.Position))
	respondJSON(w, http.StatusOK, moved)
}

func (h *Handler) ArchiveColumnTasks(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermArchive) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	columnID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if columnID <= 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	column, err := h.svc.Store().GetColumn(r.Context(), columnID)
	if err != nil || column == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	board, _ := h.svc.Store().GetBoard(r.Context(), column.BoardID)
	spaceACL := []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), column.BoardID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	count, err := h.svc.Store().ArchiveTasksByColumn(r.Context(), columnID, user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditColumnArchiveTasks, fmt.Sprintf("%d|%d", columnID, count))
	respondJSON(w, http.StatusOK, map[string]any{"status": "ok", "archived": count})
}
