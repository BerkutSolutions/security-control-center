package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"berkut-scc/core/store"
	"berkut-scc/tasks"
)

func (h *ReportsHandler) buildTasksSection(ctx context.Context, sec store.ReportSection, user *store.User, roles []string, groups []store.Group, fallbackFrom, fallbackTo *time.Time, totals map[string]int) reportSectionResult {
	res := reportSectionResult{Section: sec}
	if !tasks.Allowed(h.policy, roles, tasks.PermView) {
		res.Denied = true
		res.Markdown = fmt.Sprintf("## %s\n\n_No access._", sectionTitle(sec, "Tasks"))
		return res
	}
	if h.tasksSvc == nil {
		res.Error = "tasks unavailable"
		return res
	}
	from, to := periodOverride(sec.Config, fallbackFrom, fallbackTo)
	limit := configInt(sec.Config, "limit", 20)
	filter := tasks.TaskFilter{
		Status: strings.TrimSpace(configString(sec.Config, "status")),
		Limit:  limit * 5,
	}
	if v := configInt(sec.Config, "board_id", 0); v > 0 {
		filter.BoardID = int64(v)
	}
	if v := configInt(sec.Config, "space_id", 0); v > 0 {
		filter.SpaceID = int64(v)
	}
	items, err := h.tasksSvc.Store().ListTasks(ctx, filter)
	if err != nil {
		res.Error = "load failed"
		return res
	}
	taskIDs := make([]int64, 0, len(items))
	boardIDs := map[int64]struct{}{}
	for _, t := range items {
		taskIDs = append(taskIDs, t.ID)
		boardIDs[t.BoardID] = struct{}{}
	}
	assignments, _ := h.tasksSvc.Store().ListTaskAssignmentsForTasks(ctx, taskIDs)
	tagsByTask, _ := h.tasksSvc.Store().ListTaskTagsForTasks(ctx, taskIDs)
	boardACL := map[int64][]tasks.ACLRule{}
	boardInfo := map[int64]*tasks.Board{}
	spaceACL := map[int64][]tasks.ACLRule{}
	for id := range boardIDs {
		acl, _ := h.tasksSvc.Store().GetBoardACL(ctx, id)
		boardACL[id] = acl
		board, _ := h.tasksSvc.Store().GetBoard(ctx, id)
		boardInfo[id] = board
		if board != nil && board.SpaceID > 0 {
			if _, ok := spaceACL[board.SpaceID]; !ok {
				acl, _ := h.tasksSvc.Store().GetSpaceACL(ctx, board.SpaceID)
				spaceACL[board.SpaceID] = acl
			}
		}
	}
	assigneeFilter := configInt(sec.Config, "assignee", 0)
	tagsFilter := configStrings(sec.Config, "tags")
	var rows []tasks.Task
	for _, t := range items {
		board := boardInfo[t.BoardID]
		var spaceID int64
		if board != nil {
			spaceID = board.SpaceID
		}
		if !taskBoardAllowed(user, roles, groups, spaceACL[spaceID], boardACL[t.BoardID], "view") {
			continue
		}
		if from != nil || to != nil {
			if !withinPeriod(t.CreatedAt, from, to) {
				continue
			}
		}
		if assigneeFilter > 0 {
			matched := false
			for _, a := range assignments[t.ID] {
				if a.UserID == int64(assigneeFilter) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if len(tagsFilter) > 0 {
			taskTags := tagsByTask[t.ID]
			matched := false
			for _, tag := range taskTags {
				for _, want := range tagsFilter {
					if strings.EqualFold(tag, want) {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				continue
			}
		}
		rows = append(rows, t)
	}
	if len(rows) > limit && limit > 0 {
		rows = rows[:limit]
	}
	statusCounts := map[string]int{}
	overdue := 0
	now := time.Now().UTC()
	for _, t := range rows {
		statusCounts[strings.ToLower(t.Status)]++
		if t.DueDate != nil && t.DueDate.Before(now) && t.ClosedAt == nil && !t.IsArchived {
			overdue++
		}
	}
	res.ItemCount = len(rows)
	res.Summary = map[string]any{
		"tasks":         len(rows),
		"tasks_overdue": overdue,
	}
	totals["tasks"] += len(rows)
	totals["tasks_overdue"] += overdue
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## %s\n\n", sectionTitle(sec, "Tasks")))
	b.WriteString(fmt.Sprintf("- Total: %d\n", len(rows)))
	b.WriteString(fmt.Sprintf("- Overdue: %d\n", overdue))
	for key, count := range statusCounts {
		if key == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("- %s: %d\n", strings.Title(key), count))
	}
	if len(rows) == 0 {
		b.WriteString("\n_No tasks for selected period._\n")
		res.Markdown = b.String()
		return res
	}
	b.WriteString("\n| ID | Title | Status | Assignees | Due |\n|---|---|---|---|---|\n")
	userCache := map[int64]string{}
	for _, t := range rows {
		assignees := taskAssignees(assignments[t.ID], userCache, h)
		due := "-"
		if t.DueDate != nil {
			due = t.DueDate.UTC().Format("2006-01-02")
		}
		closedAt := ""
		if t.ClosedAt != nil {
			closedAt = t.ClosedAt.UTC().Format(time.RFC3339)
		}
		b.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s |\n",
			t.ID,
			escapePipes(t.Title),
			escapePipes(t.Status),
			escapePipes(assignees),
			due,
		))
		res.Items = append(res.Items, store.ReportSnapshotItem{
			EntityType: "task",
			EntityID:   fmt.Sprintf("%d", t.ID),
			Entity: map[string]any{
				"id":          t.ID,
				"title":       t.Title,
				"status":      t.Status,
				"board_id":    t.BoardID,
				"assigned_to": assignmentIDs(assignments[t.ID]),
				"due_date":    due,
				"created_at":  t.CreatedAt.UTC().Format(time.RFC3339),
				"updated_at":  t.UpdatedAt.UTC().Format(time.RFC3339),
				"closed_at":   closedAt,
				"is_archived": t.IsArchived,
			},
		})
	}
	res.Markdown = b.String()
	return res
}
