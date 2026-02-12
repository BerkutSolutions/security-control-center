package taskshttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"berkut-scc/tasks"
	"github.com/gorilla/mux"
)

func (h *Handler) ListSubColumnsByBoard(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	boardID := parseInt64Default(mux.Vars(r)["board_id"], 0)
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
	items, err := h.svc.Store().ListSubColumnsByBoard(r.Context(), boardID, includeInactive)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) ListSubColumns(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	columnID := parseInt64Default(mux.Vars(r)["column_id"], 0)
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
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "view") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	includeInactive := r.URL.Query().Get("include_inactive") == "1"
	items, err := h.svc.Store().ListSubColumns(r.Context(), columnID, includeInactive)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) CreateSubColumn(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	columnID := parseInt64Default(mux.Vars(r)["column_id"], 0)
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
		Name     string `json:"name"`
		Position int    `json:"position"`
		IsActive *bool  `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		respondError(w, http.StatusBadRequest, "tasks.subcolumnNameRequired")
		return
	}
	active := true
	if payload.IsActive != nil {
		active = *payload.IsActive
	}
	sub := &tasks.SubColumn{
		ColumnID: columnID,
		Name:     name,
		Position: payload.Position,
		IsActive: active,
	}
	if _, err := h.svc.Store().CreateSubColumn(r.Context(), sub); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditSubColumnCreate, fmt.Sprintf("%d", sub.ID))
	respondJSON(w, http.StatusCreated, sub)
}

func (h *Handler) UpdateSubColumn(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	subID := parseInt64Default(mux.Vars(r)["id"], 0)
	if subID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	sub, err := h.svc.Store().GetSubColumn(r.Context(), subID)
	if err != nil || sub == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	column, err := h.svc.Store().GetColumn(r.Context(), sub.ColumnID)
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
		IsActive *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	if payload.Name != nil {
		sub.Name = strings.TrimSpace(*payload.Name)
	}
	if payload.Position != nil {
		sub.Position = *payload.Position
	}
	if payload.IsActive != nil {
		sub.IsActive = *payload.IsActive
	}
	if strings.TrimSpace(sub.Name) == "" {
		respondError(w, http.StatusBadRequest, "tasks.subcolumnNameRequired")
		return
	}
	if err := h.svc.Store().UpdateSubColumn(r.Context(), sub); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditSubColumnUpdate, fmt.Sprintf("%d", sub.ID))
	respondJSON(w, http.StatusOK, sub)
}

func (h *Handler) DeleteSubColumn(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	subID := parseInt64Default(mux.Vars(r)["id"], 0)
	if subID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	sub, err := h.svc.Store().GetSubColumn(r.Context(), subID)
	if err != nil || sub == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	column, err := h.svc.Store().GetColumn(r.Context(), sub.ColumnID)
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
	count, err := h.svc.Store().CountTasksInSubColumn(r.Context(), subID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	if count > 0 {
		respondError(w, http.StatusBadRequest, "tasks.subcolumnNotEmpty")
		return
	}
	if err := h.svc.Store().DeleteSubColumn(r.Context(), subID); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditSubColumnDelete, fmt.Sprintf("%d", sub.ID))
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) MoveSubColumn(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	subID := parseInt64Default(mux.Vars(r)["id"], 0)
	if subID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	sub, err := h.svc.Store().GetSubColumn(r.Context(), subID)
	if err != nil || sub == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	column, err := h.svc.Store().GetColumn(r.Context(), sub.ColumnID)
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
	moved, err := h.svc.Store().MoveSubColumn(r.Context(), subID, payload.Position)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditSubColumnMove, fmt.Sprintf("%d|%d", subID, payload.Position))
	respondJSON(w, http.StatusOK, moved)
}
