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

func (h *Handler) ListBlocks(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	task, ok := h.getTaskWithBoardAccess(w, r, user, roles, groups, "view")
	if !ok {
		return
	}
	blocks, err := h.svc.Store().ListTaskBlocks(r.Context(), task.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	assignments, _ := h.svc.Store().ListTaskAssignments(r.Context(), task.ID)
	allowDetails := h.canViewBlockDetails(user.ID, roles, task, assignments)
	if !allowDetails {
		for i := range blocks {
			blocks[i].Reason = nil
			blocks[i].BlockerTaskID = nil
		}
	}
	var blocking []int64
	titles := map[int64]string{}
	if allowDetails {
		active, err := h.svc.Store().ListActiveBlocksByBlocker(r.Context(), task.ID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "server error")
			return
		}
		seen := map[int64]struct{}{}
		for _, b := range active {
			if _, ok := seen[b.TaskID]; ok {
				continue
			}
			seen[b.TaskID] = struct{}{}
			blocking = append(blocking, b.TaskID)
		}
		for _, block := range blocks {
			if block.BlockerTaskID == nil {
				continue
			}
			if _, ok := titles[*block.BlockerTaskID]; ok {
				continue
			}
			ref, _ := h.svc.Store().GetTask(r.Context(), *block.BlockerTaskID)
			if ref != nil && strings.TrimSpace(ref.Title) != "" {
				titles[*block.BlockerTaskID] = ref.Title
			}
		}
		for _, taskID := range blocking {
			if _, ok := titles[taskID]; ok {
				continue
			}
			ref, _ := h.svc.Store().GetTask(r.Context(), taskID)
			if ref != nil && strings.TrimSpace(ref.Title) != "" {
				titles[taskID] = ref.Title
			}
		}
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"items":    blocks,
		"blocking": blocking,
		"titles":   titles,
	})
}

func (h *Handler) AddTextBlock(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermBlockCreate) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	task, ok := h.getTaskWithBoardAccess(w, r, user, roles, groups, "manage")
	if !ok {
		return
	}
	if task.ClosedAt != nil || task.IsArchived {
		respondError(w, http.StatusConflict, "tasks.closedReadOnly")
		return
	}
	var payload struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	reason := strings.TrimSpace(payload.Reason)
	if reason == "" {
		respondError(w, http.StatusBadRequest, "tasks.blocks.reasonRequired")
		return
	}
	block := &tasks.TaskBlock{
		TaskID:    task.ID,
		BlockType: tasks.BlockTypeText,
		Reason:    &reason,
		CreatedBy: &user.ID,
		IsActive:  true,
	}
	if _, err := h.svc.Store().CreateTaskBlock(r.Context(), block); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskBlockCreateText, fmt.Sprintf("%d|%d", task.ID, block.ID))
	respondJSON(w, http.StatusCreated, block)
}

func (h *Handler) AddTaskBlock(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermBlockCreate) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	task, ok := h.getTaskWithBoardAccess(w, r, user, roles, groups, "manage")
	if !ok {
		return
	}
	if task.ClosedAt != nil || task.IsArchived {
		respondError(w, http.StatusConflict, "tasks.closedReadOnly")
		return
	}
	var payload struct {
		BlockerTaskID int64 `json:"blocker_task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	if payload.BlockerTaskID == 0 {
		respondError(w, http.StatusBadRequest, "tasks.blocks.blockerRequired")
		return
	}
	if payload.BlockerTaskID == task.ID {
		respondError(w, http.StatusBadRequest, "tasks.blocks.selfBlock")
		return
	}
	blocker, err := h.svc.Store().GetTask(r.Context(), payload.BlockerTaskID)
	if err != nil || blocker == nil {
		respondError(w, http.StatusNotFound, "tasks.blocks.blockerNotFound")
		return
	}
	board, _ := h.svc.Store().GetBoard(r.Context(), blocker.BoardID)
	spaceACL := []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), blocker.BoardID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "view") {
		respondError(w, http.StatusNotFound, "tasks.blocks.blockerNotFound")
		return
	}
	cycle, err := h.blockCycleExists(r.Context(), payload.BlockerTaskID, task.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	if cycle {
		respondError(w, http.StatusConflict, "tasks.blocks.cycleDetected")
		return
	}
	block := &tasks.TaskBlock{
		TaskID:        task.ID,
		BlockType:     tasks.BlockTypeTask,
		BlockerTaskID: &payload.BlockerTaskID,
		CreatedBy:     &user.ID,
		IsActive:      true,
	}
	if _, err := h.svc.Store().CreateTaskBlock(r.Context(), block); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskBlockCreateTask, fmt.Sprintf("%d|%d", task.ID, block.ID))
	respondJSON(w, http.StatusCreated, block)
}

func (h *Handler) ResolveBlock(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermBlockResolve) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	task, ok := h.getTaskWithBoardAccess(w, r, user, roles, groups, "manage")
	if !ok {
		return
	}
	blockID := parseInt64Default(chi.URLParam(r, "block_id"), 0)
	if blockID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	var payload struct {
		Comment string `json:"comment"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	comment := strings.TrimSpace(payload.Comment)
	block, err := h.svc.Store().ResolveTaskBlock(r.Context(), task.ID, blockID, user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	if block == nil {
		respondError(w, http.StatusNotFound, "tasks.blocks.notFound")
		return
	}
	details := fmt.Sprintf("%d|%d", task.ID, blockID)
	if comment != "" {
		details = fmt.Sprintf("%s|%s", details, comment)
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskBlockResolveManual, details)
	respondJSON(w, http.StatusOK, block)
}

func (h *Handler) blockCycleExists(ctx context.Context, blockerTaskID int64, taskID int64) (bool, error) {
	visited := map[int64]struct{}{}
	stack := []int64{taskID}
	for len(stack) > 0 {
		idx := len(stack) - 1
		current := stack[idx]
		stack = stack[:idx]
		if current == blockerTaskID {
			return true, nil
		}
		if _, ok := visited[current]; ok {
			continue
		}
		visited[current] = struct{}{}
		blocks, err := h.svc.Store().ListActiveBlocksByBlocker(ctx, current)
		if err != nil {
			return false, err
		}
		for _, block := range blocks {
			if block.TaskID == blockerTaskID {
				return true, nil
			}
			if _, ok := visited[block.TaskID]; !ok {
				stack = append(stack, block.TaskID)
			}
		}
	}
	return false, nil
}
