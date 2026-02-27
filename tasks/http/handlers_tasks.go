package taskshttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"berkut-scc/tasks"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	filter := tasks.TaskFilter{
		SpaceID:  parseInt64Default(r.URL.Query().Get("space_id"), 0),
		BoardID:  parseInt64Default(r.URL.Query().Get("board_id"), 0),
		ColumnID: parseInt64Default(r.URL.Query().Get("column_id"), 0),
		Status:   strings.TrimSpace(r.URL.Query().Get("status")),
		Limit:    parseIntDefault(r.URL.Query().Get("limit"), 0),
		Offset:   parseIntDefault(r.URL.Query().Get("offset"), 0),
	}
	if v := strings.TrimSpace(r.URL.Query().Get("search")); v != "" {
		filter.Search = v
	}
	if r.URL.Query().Get("include_archived") == "1" {
		filter.IncludeArchived = true
	}
	if v := strings.TrimSpace(r.URL.Query().Get("assigned_to")); v != "" {
		filter.AssignedUserID = parseInt64Default(v, 0)
	}
	if r.URL.Query().Get("mine") == "1" || strings.ToLower(r.URL.Query().Get("mine")) == "true" {
		filter.MineUserID = user.ID
	}
	items, err := h.svc.Store().ListTasks(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	taskIDs := make([]int64, 0, len(items))
	boardIDs := map[int64]struct{}{}
	for _, t := range items {
		taskIDs = append(taskIDs, t.ID)
		boardIDs[t.BoardID] = struct{}{}
	}
	assignments, _ := h.svc.Store().ListTaskAssignmentsForTasks(r.Context(), taskIDs)
	blocksByTask, _ := h.svc.Store().ListActiveTaskBlocksForTasks(r.Context(), taskIDs)
	tagsByTask, _ := h.svc.Store().ListTaskTagsForTasks(r.Context(), taskIDs)
	boardACL := map[int64][]tasks.ACLRule{}
	boardInfo := map[int64]*tasks.Board{}
	spaceACL := map[int64][]tasks.ACLRule{}
	for id := range boardIDs {
		acl, _ := h.svc.Store().GetBoardACL(r.Context(), id)
		boardACL[id] = acl
		board, _ := h.svc.Store().GetBoard(r.Context(), id)
		boardInfo[id] = board
		if board != nil && board.SpaceID > 0 {
			if _, ok := spaceACL[board.SpaceID]; !ok {
				acl, _ := h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
				spaceACL[board.SpaceID] = acl
			}
		}
	}
	var res []tasks.TaskDTO
	for _, t := range items {
		board := boardInfo[t.BoardID]
		var spaceID int64
		if board != nil {
			spaceID = board.SpaceID
		}
		if !boardAllowed(user, roles, groups, spaceACL[spaceID], boardACL[t.BoardID], "view") {
			continue
		}
		taskAssignments := assignments[t.ID]
		allowDetails := h.canViewBlockDetails(user.ID, roles, &t, taskAssignments)
		res = append(res, buildTaskDTO(t, taskAssignments, nil, blocksByTask[t.ID], tagsByTask[t.ID], allowDetails))
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": res})
}

func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermCreate) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	var payload struct {
		BoardID          int64    `json:"board_id"`
		ColumnID         int64    `json:"column_id"`
		SubColumnID      *int64   `json:"subcolumn_id"`
		Title            string   `json:"title"`
		Description      string   `json:"description"`
		Result           string   `json:"result"`
		ExternalLink     string   `json:"external_link"`
		BusinessCustomer string   `json:"business_customer"`
		SizeEstimate     *int     `json:"size_estimate"`
		Priority         string   `json:"priority"`
		AssignedTo       []string `json:"assigned_to"`
		Tags             []string `json:"tags"`
		DueDate          *string  `json:"due_date"`
		Position         int      `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		respondError(w, http.StatusBadRequest, "tasks.titleRequired")
		return
	}
	if payload.ColumnID == 0 {
		respondError(w, http.StatusBadRequest, "tasks.columnRequired")
		return
	}
	column, err := h.svc.Store().GetColumn(r.Context(), payload.ColumnID)
	if err != nil || column == nil {
		respondError(w, http.StatusBadRequest, "tasks.columnNotFound")
		return
	}
	boardID := payload.BoardID
	if boardID == 0 {
		boardID = column.BoardID
	}
	if boardID != column.BoardID {
		respondError(w, http.StatusBadRequest, "tasks.boardMismatch")
		return
	}
	if payload.SubColumnID != nil {
		subcolumn, err := h.svc.Store().GetSubColumn(r.Context(), *payload.SubColumnID)
		if err != nil || subcolumn == nil || !subcolumn.IsActive {
			respondError(w, http.StatusBadRequest, "tasks.subcolumnNotFound")
			return
		}
		if subcolumn.ColumnID != payload.ColumnID {
			respondError(w, http.StatusBadRequest, "tasks.columnMismatch")
			return
		}
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
	priority, err := normalizePriority(payload.Priority)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if payload.SizeEstimate != nil && *payload.SizeEstimate < 0 {
		respondError(w, http.StatusBadRequest, "tasks.sizeInvalid")
		return
	}
	due, err := parseDueDate(payload.DueDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "tasks.dueDateInvalid")
		return
	}
	assignIDs, err := h.resolveUserIDs(r.Context(), payload.AssignedTo)
	if err != nil {
		respondError(w, http.StatusBadRequest, "tasks.userNotFound")
		return
	}
	task := &tasks.Task{
		BoardID:          boardID,
		ColumnID:         payload.ColumnID,
		SubColumnID:      payload.SubColumnID,
		Title:            title,
		Description:      strings.TrimSpace(payload.Description),
		Result:           strings.TrimSpace(payload.Result),
		ExternalLink:     strings.TrimSpace(payload.ExternalLink),
		BusinessCustomer: strings.TrimSpace(payload.BusinessCustomer),
		SizeEstimate:     payload.SizeEstimate,
		Priority:         priority,
		CreatedBy:        &user.ID,
		DueDate:          due,
		Position:         payload.Position,
		IsArchived:       false,
	}
	if _, err := h.svc.Store().CreateTask(r.Context(), task, assignIDs); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	if payload.Tags != nil {
		_ = h.svc.Store().SetTaskTags(r.Context(), task.ID, payload.Tags)
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskCreate, fmt.Sprintf("%d", task.ID))
	if len(assignIDs) > 0 {
		tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskAssign, fmt.Sprintf("%d", task.ID))
	}
	respondJSON(w, http.StatusCreated, buildTaskDTO(*task, nil, assignIDs, nil, payload.Tags, true))
}

func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	taskID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if taskID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	task, err := h.svc.Store().GetTask(r.Context(), taskID)
	if err != nil || task == nil {
		respondError(w, http.StatusNotFound, "tasks.notFound")
		return
	}
	sourceBoard, _ := h.svc.Store().GetBoard(r.Context(), task.BoardID)
	sourceSpaceACL := []tasks.ACLRule{}
	if sourceBoard != nil && sourceBoard.SpaceID > 0 {
		sourceSpaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), sourceBoard.SpaceID)
	}
	sourceBoardACL, _ := h.svc.Store().GetBoardACL(r.Context(), task.BoardID)
	if !boardAllowed(user, roles, groups, sourceSpaceACL, sourceBoardACL, "view") {
		respondError(w, http.StatusNotFound, "tasks.notFound")
		return
	}
	board, _ := h.svc.Store().GetBoard(r.Context(), task.BoardID)
	spaceACL := []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), task.BoardID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "view") {
		respondError(w, http.StatusNotFound, "tasks.notFound")
		return
	}
	assignments, _ := h.svc.Store().ListTaskAssignments(r.Context(), task.ID)
	blocksByTask, _ := h.svc.Store().ListActiveTaskBlocksForTasks(r.Context(), []int64{task.ID})
	allowDetails := h.canViewBlockDetails(user.ID, roles, task, assignments)
	tags, _ := h.svc.Store().ListTaskTagsForTasks(r.Context(), []int64{task.ID})
	respondJSON(w, http.StatusOK, buildTaskDTO(*task, assignments, nil, blocksByTask[task.ID], tags[task.ID], allowDetails))
}

func (h *Handler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermEdit) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	taskID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if taskID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	task, err := h.svc.Store().GetTask(r.Context(), taskID)
	if err != nil || task == nil {
		respondError(w, http.StatusNotFound, "tasks.notFound")
		return
	}
	board, _ := h.svc.Store().GetBoard(r.Context(), task.BoardID)
	spaceACL := []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), task.BoardID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	if task.ClosedAt != nil {
		respondError(w, http.StatusConflict, "tasks.closedReadOnly")
		return
	}
	var payload struct {
		Title            *string                    `json:"title"`
		Description      *string                    `json:"description"`
		Result           *string                    `json:"result"`
		ExternalLink     *string                    `json:"external_link"`
		BusinessCustomer *string                    `json:"business_customer"`
		SizeEstimate     *int                       `json:"size_estimate"`
		Priority         *string                    `json:"priority"`
		DueDate          *string                    `json:"due_date"`
		AssignedTo       []string                   `json:"assigned_to"`
		Tags             []string                   `json:"tags"`
		Checklist        *[]tasks.TaskChecklistItem `json:"checklist"`
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	var payloadRaw map[string]json.RawMessage
	_ = json.Unmarshal(body, &payloadRaw)
	if payloadRaw == nil {
		payloadRaw = map[string]json.RawMessage{}
	}
	if payload.Title != nil {
		task.Title = strings.TrimSpace(*payload.Title)
	}
	if payload.Description != nil {
		task.Description = strings.TrimSpace(*payload.Description)
	}
	if payload.Result != nil {
		task.Result = strings.TrimSpace(*payload.Result)
	}
	if _, ok := payloadRaw["external_link"]; ok {
		if payload.ExternalLink != nil {
			task.ExternalLink = strings.TrimSpace(*payload.ExternalLink)
		} else {
			task.ExternalLink = ""
		}
	}
	if _, ok := payloadRaw["business_customer"]; ok {
		if payload.BusinessCustomer != nil {
			task.BusinessCustomer = strings.TrimSpace(*payload.BusinessCustomer)
		} else {
			task.BusinessCustomer = ""
		}
	}
	if payload.SizeEstimate != nil {
		if *payload.SizeEstimate < 0 {
			respondError(w, http.StatusBadRequest, "tasks.sizeInvalid")
			return
		}
		task.SizeEstimate = payload.SizeEstimate
	}
	if payload.SizeEstimate == nil {
		if _, ok := payloadRaw["size_estimate"]; ok {
			task.SizeEstimate = nil
		}
	}
	if payload.Priority != nil {
		if err := validatePriority(*payload.Priority); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		task.Priority = strings.ToLower(strings.TrimSpace(*payload.Priority))
	}
	if payload.DueDate != nil {
		if strings.TrimSpace(*payload.DueDate) == "" {
			task.DueDate = nil
		} else {
			parsed, err := parseISOTime(*payload.DueDate)
			if err != nil {
				respondError(w, http.StatusBadRequest, "tasks.dueDateInvalid")
				return
			}
			task.DueDate = &parsed
		}
	}
	if _, ok := payloadRaw["checklist"]; ok {
		list := []tasks.TaskChecklistItem{}
		if payload.Checklist != nil {
			list = *payload.Checklist
		}
		task.Checklist = normalizeChecklist(list, task.Checklist, user.ID, time.Now().UTC())
	}
	if strings.TrimSpace(task.Title) == "" {
		respondError(w, http.StatusBadRequest, "tasks.titleRequired")
		return
	}
	if err := h.svc.Store().UpdateTask(r.Context(), task); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	if payload.AssignedTo != nil {
		if !tasks.Allowed(h.policy, roles, tasks.PermAssign) {
			respondError(w, http.StatusForbidden, "forbidden")
			return
		}
		assignIDs, err := h.resolveUserIDs(r.Context(), payload.AssignedTo)
		if err != nil {
			respondError(w, http.StatusBadRequest, "tasks.userNotFound")
			return
		}
		if err := h.svc.Store().SetTaskAssignments(r.Context(), task.ID, assignIDs, user.ID); err != nil {
			respondError(w, http.StatusInternalServerError, "server error")
			return
		}
		tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskAssign, fmt.Sprintf("%d", task.ID))
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskUpdate, fmt.Sprintf("%d", task.ID))
	assignments, _ := h.svc.Store().ListTaskAssignments(r.Context(), task.ID)
	blocksByTask, _ := h.svc.Store().ListActiveTaskBlocksForTasks(r.Context(), []int64{task.ID})
	allowDetails := h.canViewBlockDetails(user.ID, roles, task, assignments)
	if payload.Tags != nil {
		_ = h.svc.Store().SetTaskTags(r.Context(), task.ID, payload.Tags)
	}
	tags, _ := h.svc.Store().ListTaskTagsForTasks(r.Context(), []int64{task.ID})
	respondJSON(w, http.StatusOK, buildTaskDTO(*task, assignments, nil, blocksByTask[task.ID], tags[task.ID], allowDetails))
}

func (h *Handler) MoveTask(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermMove) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	taskID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if taskID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	task, err := h.svc.Store().GetTask(r.Context(), taskID)
	if err != nil || task == nil {
		respondError(w, http.StatusNotFound, "tasks.notFound")
		return
	}
	board, _ := h.svc.Store().GetBoard(r.Context(), task.BoardID)
	spaceACL := []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), task.BoardID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	if task.ClosedAt != nil {
		respondError(w, http.StatusConflict, "tasks.closedReadOnly")
		return
	}
	var payload struct {
		ColumnID    int64  `json:"column_id"`
		SubColumnID *int64 `json:"subcolumn_id"`
		Position    int    `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	if payload.ColumnID == 0 {
		respondError(w, http.StatusBadRequest, "tasks.columnRequired")
		return
	}
	column, err := h.svc.Store().GetColumn(r.Context(), payload.ColumnID)
	if err != nil || column == nil {
		respondError(w, http.StatusBadRequest, "tasks.columnNotFound")
		return
	}
	if column.BoardID != task.BoardID {
		respondError(w, http.StatusBadRequest, "tasks.boardMismatch")
		return
	}
	if payload.SubColumnID != nil {
		subcolumn, err := h.svc.Store().GetSubColumn(r.Context(), *payload.SubColumnID)
		if err != nil || subcolumn == nil || !subcolumn.IsActive {
			respondError(w, http.StatusBadRequest, "tasks.subcolumnNotFound")
			return
		}
		if subcolumn.ColumnID != payload.ColumnID {
			respondError(w, http.StatusBadRequest, "tasks.columnMismatch")
			return
		}
	}
	if column.IsFinal {
		activeBlocks, err := h.activeBlocksForTask(r.Context(), task.ID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "server error")
			return
		}
		if len(activeBlocks) > 0 {
			tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskMoveDeniedBlockedFinal, fmt.Sprintf("%d|%d", task.ID, payload.ColumnID))
			respondError(w, http.StatusConflict, "tasks.blockedFinal")
			return
		}
	}
	moved, err := h.svc.Store().MoveTask(r.Context(), task.ID, payload.ColumnID, payload.SubColumnID, payload.Position)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskMove, fmt.Sprintf("%d", task.ID))
	if column.IsFinal {
		autoBlocks, err := h.svc.Store().ResolveTaskBlocksByBlocker(r.Context(), task.ID, user.ID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "server error")
			return
		}
		for _, block := range autoBlocks {
			tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskBlockResolveAuto, fmt.Sprintf("%d|%d", block.TaskID, block.ID))
		}
	}
	assignments, _ := h.svc.Store().ListTaskAssignments(r.Context(), task.ID)
	blocksByTask, _ := h.svc.Store().ListActiveTaskBlocksForTasks(r.Context(), []int64{task.ID})
	allowDetails := h.canViewBlockDetails(user.ID, roles, moved, assignments)
	tags, _ := h.svc.Store().ListTaskTagsForTasks(r.Context(), []int64{task.ID})
	respondJSON(w, http.StatusOK, buildTaskDTO(*moved, assignments, nil, blocksByTask[task.ID], tags[task.ID], allowDetails))
}

func (h *Handler) RelocateTask(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermMove) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	taskID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if taskID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	task, err := h.svc.Store().GetTask(r.Context(), taskID)
	if err != nil || task == nil {
		respondError(w, http.StatusNotFound, "tasks.notFound")
		return
	}
	if task.ClosedAt != nil {
		respondError(w, http.StatusConflict, "tasks.closedReadOnly")
		return
	}
	var payload struct {
		BoardID     int64  `json:"board_id"`
		ColumnID    int64  `json:"column_id"`
		SubColumnID *int64 `json:"subcolumn_id"`
		Position    int    `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	if payload.ColumnID == 0 {
		respondError(w, http.StatusBadRequest, "tasks.columnRequired")
		return
	}
	column, err := h.svc.Store().GetColumn(r.Context(), payload.ColumnID)
	if err != nil || column == nil {
		respondError(w, http.StatusBadRequest, "tasks.columnNotFound")
		return
	}
	targetBoardID := payload.BoardID
	if targetBoardID == 0 {
		targetBoardID = column.BoardID
	}
	if column.BoardID != targetBoardID {
		respondError(w, http.StatusBadRequest, "tasks.boardMismatch")
		return
	}
	if payload.SubColumnID != nil {
		subcolumn, err := h.svc.Store().GetSubColumn(r.Context(), *payload.SubColumnID)
		if err != nil || subcolumn == nil || !subcolumn.IsActive {
			respondError(w, http.StatusBadRequest, "tasks.subcolumnNotFound")
			return
		}
		if subcolumn.ColumnID != payload.ColumnID {
			respondError(w, http.StatusBadRequest, "tasks.columnMismatch")
			return
		}
	}
	sourceBoard, _ := h.svc.Store().GetBoard(r.Context(), task.BoardID)
	targetBoard, _ := h.svc.Store().GetBoard(r.Context(), targetBoardID)
	sourceSpaceACL := []tasks.ACLRule{}
	targetSpaceACL := []tasks.ACLRule{}
	if sourceBoard != nil && sourceBoard.SpaceID > 0 {
		sourceSpaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), sourceBoard.SpaceID)
	}
	if targetBoard != nil && targetBoard.SpaceID > 0 {
		targetSpaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), targetBoard.SpaceID)
	}
	sourceBoardACL, _ := h.svc.Store().GetBoardACL(r.Context(), task.BoardID)
	targetBoardACL, _ := h.svc.Store().GetBoardACL(r.Context(), targetBoardID)
	if !boardAllowed(user, roles, groups, sourceSpaceACL, sourceBoardACL, "manage") || !boardAllowed(user, roles, groups, targetSpaceACL, targetBoardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	if column.IsFinal {
		activeBlocks, err := h.activeBlocksForTask(r.Context(), task.ID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "server error")
			return
		}
		if len(activeBlocks) > 0 {
			tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskMoveDeniedBlockedFinal, fmt.Sprintf("%d|%d", task.ID, payload.ColumnID))
			respondError(w, http.StatusConflict, "tasks.blockedFinal")
			return
		}
	}
	moved, err := h.svc.Store().RelocateTask(r.Context(), task.ID, targetBoardID, payload.ColumnID, payload.SubColumnID, payload.Position)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskRelocate, fmt.Sprintf("%d|%d|%d", task.ID, targetBoardID, payload.ColumnID))
	if column.IsFinal {
		autoBlocks, err := h.svc.Store().ResolveTaskBlocksByBlocker(r.Context(), task.ID, user.ID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "server error")
			return
		}
		for _, block := range autoBlocks {
			tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskBlockResolveAuto, fmt.Sprintf("%d|%d", block.TaskID, block.ID))
		}
	}
	assignments, _ := h.svc.Store().ListTaskAssignments(r.Context(), task.ID)
	blocksByTask, _ := h.svc.Store().ListActiveTaskBlocksForTasks(r.Context(), []int64{task.ID})
	allowDetails := h.canViewBlockDetails(user.ID, roles, moved, assignments)
	tags, _ := h.svc.Store().ListTaskTagsForTasks(r.Context(), []int64{task.ID})
	respondJSON(w, http.StatusOK, buildTaskDTO(*moved, assignments, nil, blocksByTask[task.ID], tags[task.ID], allowDetails))
}

func (h *Handler) CloneTask(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermCreate) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	taskID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if taskID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	task, err := h.svc.Store().GetTask(r.Context(), taskID)
	if err != nil || task == nil {
		respondError(w, http.StatusNotFound, "tasks.notFound")
		return
	}
	var payload struct {
		BoardID     int64  `json:"board_id"`
		ColumnID    int64  `json:"column_id"`
		SubColumnID *int64 `json:"subcolumn_id"`
		Position    int    `json:"position"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	targetColumnID := payload.ColumnID
	if targetColumnID == 0 {
		targetColumnID = task.ColumnID
	}
	column, err := h.svc.Store().GetColumn(r.Context(), targetColumnID)
	if err != nil || column == nil {
		respondError(w, http.StatusBadRequest, "tasks.columnNotFound")
		return
	}
	targetBoardID := payload.BoardID
	if targetBoardID == 0 {
		targetBoardID = column.BoardID
	}
	if column.BoardID != targetBoardID {
		respondError(w, http.StatusBadRequest, "tasks.boardMismatch")
		return
	}
	targetSubColumnID := payload.SubColumnID
	if targetSubColumnID == nil {
		targetSubColumnID = task.SubColumnID
	}
	if targetSubColumnID != nil {
		subcolumn, err := h.svc.Store().GetSubColumn(r.Context(), *targetSubColumnID)
		if err != nil || subcolumn == nil || !subcolumn.IsActive {
			respondError(w, http.StatusBadRequest, "tasks.subcolumnNotFound")
			return
		}
		if subcolumn.ColumnID != targetColumnID {
			targetSubColumnID = nil
		}
	}
	board, _ := h.svc.Store().GetBoard(r.Context(), targetBoardID)
	spaceACL := []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), targetBoardID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	assignments, _ := h.svc.Store().ListTaskAssignments(r.Context(), task.ID)
	var assignIDs []int64
	for _, a := range assignments {
		assignIDs = append(assignIDs, a.UserID)
	}
	links, _ := h.svc.Store().ListEntityLinks(r.Context(), "task", fmt.Sprintf("%d", task.ID))
	var cloneLinks []tasks.Link
	for _, link := range links {
		cloneLinks = append(cloneLinks, tasks.Link{
			SourceType: "task",
			SourceID:   "",
			TargetType: link.TargetType,
			TargetID:   link.TargetID,
		})
	}
	newTask := &tasks.Task{
		BoardID:          targetBoardID,
		ColumnID:         targetColumnID,
		SubColumnID:      targetSubColumnID,
		Title:            task.Title,
		Description:      task.Description,
		Result:           task.Result,
		ExternalLink:     task.ExternalLink,
		BusinessCustomer: task.BusinessCustomer,
		SizeEstimate:     task.SizeEstimate,
		Status:           "",
		Priority:         task.Priority,
		Checklist:        append([]tasks.TaskChecklistItem{}, task.Checklist...),
		CreatedBy:        &user.ID,
		DueDate:          task.DueDate,
		Position:         payload.Position,
		IsArchived:       false,
	}
	if _, err := h.svc.Store().CreateTaskWithLinks(r.Context(), newTask, assignIDs, cloneLinks); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tags, _ := h.svc.Store().ListTaskTagsForTasks(r.Context(), []int64{task.ID})
	if len(tags[task.ID]) > 0 {
		_ = h.svc.Store().SetTaskTags(r.Context(), newTask.ID, tags[task.ID])
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskClone, fmt.Sprintf("%d|%d", task.ID, newTask.ID))
	if len(assignIDs) > 0 {
		tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskAssign, fmt.Sprintf("%d", newTask.ID))
	}
	blocksByTask := map[int64][]tasks.TaskBlock{}
	respondJSON(w, http.StatusCreated, buildTaskDTO(*newTask, nil, assignIDs, blocksByTask[newTask.ID], tags[task.ID], true))
}

func (h *Handler) CloseTask(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermClose) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	taskID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if taskID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	task, err := h.svc.Store().GetTask(r.Context(), taskID)
	if err != nil || task == nil {
		respondError(w, http.StatusNotFound, "tasks.notFound")
		return
	}
	board, _ := h.svc.Store().GetBoard(r.Context(), task.BoardID)
	spaceACL := []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), task.BoardID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	activeBlocks, err := h.activeBlocksForTask(r.Context(), task.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	if len(activeBlocks) > 0 {
		tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskCloseDeniedBlocked, fmt.Sprintf("%d", task.ID))
		respondError(w, http.StatusConflict, "tasks.blockedClose")
		return
	}
	closed, err := h.svc.Store().CloseTask(r.Context(), task.ID, user.ID)
	if err != nil {
		if errors.Is(err, tasks.ErrConflict) {
			respondError(w, http.StatusConflict, "tasks.alreadyClosed")
			return
		}
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskClose, fmt.Sprintf("%d", task.ID))
	autoBlocks, err := h.svc.Store().ResolveTaskBlocksByBlocker(r.Context(), task.ID, user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	for _, block := range autoBlocks {
		tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskBlockResolveAuto, fmt.Sprintf("%d|%d", block.TaskID, block.ID))
	}
	assignments, _ := h.svc.Store().ListTaskAssignments(r.Context(), task.ID)
	blocksByTask, _ := h.svc.Store().ListActiveTaskBlocksForTasks(r.Context(), []int64{task.ID})
	allowDetails := h.canViewBlockDetails(user.ID, roles, closed, assignments)
	tags, _ := h.svc.Store().ListTaskTagsForTasks(r.Context(), []int64{task.ID})
	respondJSON(w, http.StatusOK, buildTaskDTO(*closed, assignments, nil, blocksByTask[task.ID], tags[task.ID], allowDetails))
}

func (h *Handler) ArchiveTask(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermArchive) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	taskID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if taskID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	task, err := h.svc.Store().GetTask(r.Context(), taskID)
	if err != nil || task == nil {
		respondError(w, http.StatusNotFound, "tasks.notFound")
		return
	}
	board, _ := h.svc.Store().GetBoard(r.Context(), task.BoardID)
	spaceACL := []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), task.BoardID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	archived, err := h.svc.Store().ArchiveTask(r.Context(), task.ID, user.ID)
	if err != nil {
		if errors.Is(err, tasks.ErrConflict) {
			respondError(w, http.StatusConflict, "tasks.alreadyArchived")
			return
		}
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskArchive, fmt.Sprintf("%d", task.ID))
	assignments, _ := h.svc.Store().ListTaskAssignments(r.Context(), task.ID)
	blocksByTask, _ := h.svc.Store().ListActiveTaskBlocksForTasks(r.Context(), []int64{task.ID})
	allowDetails := h.canViewBlockDetails(user.ID, roles, archived, assignments)
	_ = h.svc.Store().SetTaskTags(r.Context(), task.ID, nil)
	tags, _ := h.svc.Store().ListTaskTagsForTasks(r.Context(), []int64{task.ID})
	respondJSON(w, http.StatusOK, buildTaskDTO(*archived, assignments, nil, blocksByTask[task.ID], tags[task.ID], allowDetails))
}

func (h *Handler) ListArchivedTasks(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermArchive) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	filter := tasks.TaskFilter{
		SpaceID:  parseInt64Default(r.URL.Query().Get("space_id"), 0),
		BoardID:  parseInt64Default(r.URL.Query().Get("board_id"), 0),
		ColumnID: parseInt64Default(r.URL.Query().Get("column_id"), 0),
		Limit:    parseIntDefault(r.URL.Query().Get("limit"), 0),
		Offset:   parseIntDefault(r.URL.Query().Get("offset"), 0),
	}
	if v := strings.TrimSpace(r.URL.Query().Get("search")); v != "" {
		filter.Search = v
	}
	items, err := h.svc.Store().ListArchivedTasks(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	boardACL := map[int64][]tasks.ACLRule{}
	spaceACL := map[int64][]tasks.ACLRule{}
	boardInfo := map[int64]*tasks.Board{}
	for _, item := range items {
		boardID := item.ArchivedBoardID
		if boardID == 0 {
			boardID = item.BoardID
		}
		if _, ok := boardInfo[boardID]; !ok {
			board, _ := h.svc.Store().GetBoard(r.Context(), boardID)
			boardInfo[boardID] = board
		}
		if _, ok := boardACL[boardID]; !ok {
			acl, _ := h.svc.Store().GetBoardACL(r.Context(), boardID)
			boardACL[boardID] = acl
		}
		spaceID := int64(0)
		if boardInfo[boardID] != nil {
			spaceID = boardInfo[boardID].SpaceID
		}
		if spaceID > 0 {
			if _, ok := spaceACL[spaceID]; !ok {
				acl, _ := h.svc.Store().GetSpaceACL(r.Context(), spaceID)
				spaceACL[spaceID] = acl
			}
		}
	}
	res := make([]tasks.TaskArchiveEntry, 0, len(items))
	for _, item := range items {
		boardID := item.ArchivedBoardID
		if boardID == 0 {
			boardID = item.BoardID
		}
		spaceID := int64(0)
		if boardInfo[boardID] != nil {
			spaceID = boardInfo[boardID].SpaceID
		}
		if !boardAllowed(user, roles, groups, spaceACL[spaceID], boardACL[boardID], "view") {
			continue
		}
		res = append(res, item)
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": res})
}

func (h *Handler) RestoreTask(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermArchive) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	taskID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if taskID <= 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	task, err := h.svc.Store().GetTask(r.Context(), taskID)
	if err != nil || task == nil {
		respondError(w, http.StatusNotFound, "tasks.notFound")
		return
	}
	if !task.IsArchived {
		respondError(w, http.StatusConflict, "tasks.notArchived")
		return
	}
	var payload struct {
		BoardID     int64  `json:"board_id"`
		ColumnID    int64  `json:"column_id"`
		SubColumnID *int64 `json:"subcolumn_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)

	sourceBoard, _ := h.svc.Store().GetBoard(r.Context(), task.BoardID)
	sourceSpaceACL := []tasks.ACLRule{}
	if sourceBoard != nil && sourceBoard.SpaceID > 0 {
		sourceSpaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), sourceBoard.SpaceID)
	}
	sourceBoardACL, _ := h.svc.Store().GetBoardACL(r.Context(), task.BoardID)
	if !boardAllowed(user, roles, groups, sourceSpaceACL, sourceBoardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	if payload.BoardID > 0 {
		targetBoard, _ := h.svc.Store().GetBoard(r.Context(), payload.BoardID)
		targetSpaceACL := []tasks.ACLRule{}
		if targetBoard != nil && targetBoard.SpaceID > 0 {
			targetSpaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), targetBoard.SpaceID)
		}
		targetBoardACL, _ := h.svc.Store().GetBoardACL(r.Context(), payload.BoardID)
		if !boardAllowed(user, roles, groups, targetSpaceACL, targetBoardACL, "manage") {
			respondError(w, http.StatusForbidden, "forbidden")
			return
		}
	}

	restored, err := h.svc.Store().RestoreTask(r.Context(), taskID, payload.BoardID, payload.ColumnID, payload.SubColumnID, user.ID)
	if err != nil {
		if errors.Is(err, tasks.ErrNotFound) {
			respondError(w, http.StatusNotFound, "tasks.restoreTargetNotFound")
			return
		}
		if errors.Is(err, tasks.ErrInvalidInput) {
			respondError(w, http.StatusBadRequest, "tasks.restoreTargetRequired")
			return
		}
		if errors.Is(err, tasks.ErrConflict) {
			respondError(w, http.StatusConflict, "tasks.notArchived")
			return
		}
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskRestore, fmt.Sprintf("%d|%d|%d", taskID, restored.BoardID, restored.ColumnID))
	assignments, _ := h.svc.Store().ListTaskAssignments(r.Context(), taskID)
	blocksByTask, _ := h.svc.Store().ListActiveTaskBlocksForTasks(r.Context(), []int64{taskID})
	allowDetails := h.canViewBlockDetails(user.ID, roles, restored, assignments)
	tags, _ := h.svc.Store().ListTaskTagsForTasks(r.Context(), []int64{taskID})
	respondJSON(w, http.StatusOK, buildTaskDTO(*restored, assignments, nil, blocksByTask[taskID], tags[taskID], allowDetails))
}

func (h *Handler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermArchive) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	taskID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if taskID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	task, err := h.svc.Store().GetTask(r.Context(), taskID)
	if err != nil || task == nil {
		respondError(w, http.StatusNotFound, "tasks.notFound")
		return
	}
	board, _ := h.svc.Store().GetBoard(r.Context(), task.BoardID)
	spaceACL := []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), task.BoardID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	if err := h.svc.Store().DeleteTask(r.Context(), task.ID); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskDelete, fmt.Sprintf("%d", task.ID))
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func buildTaskDTO(task tasks.Task, assignments []tasks.Assignment, assignIDs []int64, blocks []tasks.TaskBlock, tags []string, allowDetails bool) tasks.TaskDTO {
	ids := assignIDs
	if ids == nil {
		for _, a := range assignments {
			ids = append(ids, a.UserID)
		}
	}
	blockInfos, blockedBy := taskBlockInfo(blocks, allowDetails)
	return tasks.TaskDTO{
		Task:           task,
		AssignedTo:     ids,
		IsBlocked:      len(blocks) > 0,
		Blocks:         blockInfos,
		BlockedByTasks: blockedBy,
		Tags:           tags,
	}
}
