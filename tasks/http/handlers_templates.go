package taskshttp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	cstore "berkut-scc/core/store"
	"berkut-scc/tasks"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermTemplatesView) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	filter := tasks.TaskTemplateFilter{
		BoardID:         parseInt64Default(r.URL.Query().Get("board_id"), 0),
		IncludeInactive: r.URL.Query().Get("include_inactive") == "1",
	}
	items, err := h.svc.Store().ListTaskTemplates(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	boardACL := map[int64][]tasks.ACLRule{}
	boardInfo := map[int64]*tasks.Board{}
	spaceACL := map[int64][]tasks.ACLRule{}
	for _, tpl := range items {
		if _, ok := boardACL[tpl.BoardID]; ok {
			continue
		}
		acl, _ := h.svc.Store().GetBoardACL(r.Context(), tpl.BoardID)
		boardACL[tpl.BoardID] = acl
		board, _ := h.svc.Store().GetBoard(r.Context(), tpl.BoardID)
		boardInfo[tpl.BoardID] = board
		if board != nil && board.SpaceID > 0 {
			if _, ok := spaceACL[board.SpaceID]; !ok {
				acl, _ := h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
				spaceACL[board.SpaceID] = acl
			}
		}
	}
	var res []tasks.TaskTemplate
	for _, tpl := range items {
		board := boardInfo[tpl.BoardID]
		var spaceID int64
		if board != nil {
			spaceID = board.SpaceID
		}
		if !boardAllowed(user, roles, groups, spaceACL[spaceID], boardACL[tpl.BoardID], "view") {
			continue
		}
		res = append(res, tpl)
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": res})
}

func (h *Handler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, eff, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermTemplatesManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	var payload struct {
		BoardID             int64                     `json:"board_id"`
		ColumnID            int64                     `json:"column_id"`
		TitleTemplate       string                    `json:"title_template"`
		DescriptionTemplate string                    `json:"description_template"`
		Priority            string                    `json:"priority"`
		DefaultAssignees    []string                  `json:"default_assignees"`
		DefaultDueDays      int                       `json:"default_due_days"`
		ChecklistTemplate   []tasks.TaskChecklistItem `json:"checklist_template"`
		LinksTemplate       []tasks.TaskTemplateLink  `json:"links_template"`
		IsActive            *bool                     `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	title := strings.TrimSpace(payload.TitleTemplate)
	if title == "" {
		respondError(w, http.StatusBadRequest, "tasks.templateTitleRequired")
		return
	}
	if payload.BoardID == 0 {
		respondError(w, http.StatusBadRequest, "tasks.boardRequired")
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
	if column.BoardID != payload.BoardID {
		respondError(w, http.StatusBadRequest, "tasks.boardMismatch")
		return
	}
	board, _ := h.svc.Store().GetBoard(r.Context(), payload.BoardID)
	spaceACL := []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), payload.BoardID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	priority, err := normalizePriority(payload.Priority)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if payload.DefaultDueDays < 0 {
		respondError(w, http.StatusBadRequest, "tasks.templateDueInvalid")
		return
	}
	assignees, err := h.resolveUserIDs(r.Context(), payload.DefaultAssignees)
	if err != nil {
		respondError(w, http.StatusBadRequest, "tasks.userNotFound")
		return
	}
	checklist := normalizeChecklistTemplate(payload.ChecklistTemplate)
	links, err := h.validateTemplateLinks(r.Context(), user, roles, groups, eff, payload.LinksTemplate)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	active := true
	if payload.IsActive != nil {
		active = *payload.IsActive
	}
	tpl := &tasks.TaskTemplate{
		BoardID:             payload.BoardID,
		ColumnID:            payload.ColumnID,
		TitleTemplate:       title,
		DescriptionTemplate: strings.TrimSpace(payload.DescriptionTemplate),
		Priority:            priority,
		DefaultAssignees:    assignees,
		DefaultDueDays:      payload.DefaultDueDays,
		ChecklistTemplate:   checklist,
		LinksTemplate:       links,
		IsActive:            active,
		CreatedBy:           &user.ID,
	}
	if _, err := h.svc.Store().CreateTaskTemplate(r.Context(), tpl); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTemplateCreate, fmt.Sprintf("%d", tpl.ID))
	respondJSON(w, http.StatusCreated, tpl)
}

func (h *Handler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, eff, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermTemplatesManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	id := parseInt64Default(chi.URLParam(r, "id"), 0)
	if id == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	tpl, err := h.svc.Store().GetTaskTemplate(r.Context(), id)
	if err != nil || tpl == nil {
		respondError(w, http.StatusNotFound, "tasks.templateNotFound")
		return
	}
	board, _ := h.svc.Store().GetBoard(r.Context(), tpl.BoardID)
	spaceACL := []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), tpl.BoardID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	var payload struct {
		BoardID             *int64                     `json:"board_id"`
		ColumnID            *int64                     `json:"column_id"`
		TitleTemplate       *string                    `json:"title_template"`
		DescriptionTemplate *string                    `json:"description_template"`
		Priority            *string                    `json:"priority"`
		DefaultAssignees    []string                   `json:"default_assignees"`
		DefaultDueDays      *int                       `json:"default_due_days"`
		ChecklistTemplate   *[]tasks.TaskChecklistItem `json:"checklist_template"`
		LinksTemplate       *[]tasks.TaskTemplateLink  `json:"links_template"`
		IsActive            *bool                      `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	if payload.BoardID != nil {
		tpl.BoardID = *payload.BoardID
	}
	if payload.ColumnID != nil {
		tpl.ColumnID = *payload.ColumnID
	}
	if payload.TitleTemplate != nil {
		tpl.TitleTemplate = strings.TrimSpace(*payload.TitleTemplate)
	}
	if payload.DescriptionTemplate != nil {
		tpl.DescriptionTemplate = strings.TrimSpace(*payload.DescriptionTemplate)
	}
	if payload.Priority != nil {
		priority, err := normalizePriority(*payload.Priority)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		tpl.Priority = priority
	}
	if payload.DefaultDueDays != nil {
		if *payload.DefaultDueDays < 0 {
			respondError(w, http.StatusBadRequest, "tasks.templateDueInvalid")
			return
		}
		tpl.DefaultDueDays = *payload.DefaultDueDays
	}
	if payload.DefaultAssignees != nil {
		assignees, err := h.resolveUserIDs(r.Context(), payload.DefaultAssignees)
		if err != nil {
			respondError(w, http.StatusBadRequest, "tasks.userNotFound")
			return
		}
		tpl.DefaultAssignees = assignees
	}
	if payload.ChecklistTemplate != nil {
		tpl.ChecklistTemplate = normalizeChecklistTemplate(*payload.ChecklistTemplate)
	}
	if payload.LinksTemplate != nil {
		links, err := h.validateTemplateLinks(r.Context(), user, roles, groups, eff, *payload.LinksTemplate)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		tpl.LinksTemplate = links
	}
	if payload.IsActive != nil {
		tpl.IsActive = *payload.IsActive
	}
	if strings.TrimSpace(tpl.TitleTemplate) == "" {
		respondError(w, http.StatusBadRequest, "tasks.templateTitleRequired")
		return
	}
	if tpl.ColumnID == 0 {
		respondError(w, http.StatusBadRequest, "tasks.columnRequired")
		return
	}
	column, err := h.svc.Store().GetColumn(r.Context(), tpl.ColumnID)
	if err != nil || column == nil {
		respondError(w, http.StatusBadRequest, "tasks.columnNotFound")
		return
	}
	if tpl.BoardID == 0 {
		tpl.BoardID = column.BoardID
	}
	if tpl.BoardID != column.BoardID {
		respondError(w, http.StatusBadRequest, "tasks.boardMismatch")
		return
	}
	board, _ = h.svc.Store().GetBoard(r.Context(), tpl.BoardID)
	spaceACL = []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ = h.svc.Store().GetBoardACL(r.Context(), tpl.BoardID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	if err := h.svc.Store().UpdateTaskTemplate(r.Context(), tpl); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTemplateUpdate, fmt.Sprintf("%d", tpl.ID))
	respondJSON(w, http.StatusOK, tpl)
}

func (h *Handler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermTemplatesManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	id := parseInt64Default(chi.URLParam(r, "id"), 0)
	if id == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	tpl, err := h.svc.Store().GetTaskTemplate(r.Context(), id)
	if err != nil || tpl == nil {
		respondError(w, http.StatusNotFound, "tasks.templateNotFound")
		return
	}
	board, _ := h.svc.Store().GetBoard(r.Context(), tpl.BoardID)
	spaceACL := []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), tpl.BoardID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	if err := h.svc.Store().DeleteTaskTemplate(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTemplateDelete, fmt.Sprintf("%d", id))
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) CreateTaskFromTemplate(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermCreate) || !tasks.Allowed(h.policy, roles, tasks.PermTemplatesView) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	id := parseInt64Default(chi.URLParam(r, "id"), 0)
	if id == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	tpl, err := h.svc.Store().GetTaskTemplate(r.Context(), id)
	if err != nil || tpl == nil {
		respondError(w, http.StatusNotFound, "tasks.templateNotFound")
		return
	}
	if !tpl.IsActive {
		respondError(w, http.StatusConflict, "tasks.templateInactive")
		return
	}
	var payload struct {
		ColumnID    *int64  `json:"column_id"`
		SubColumnID *int64  `json:"subcolumn_id"`
		Title       *string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	board, _ := h.svc.Store().GetBoard(r.Context(), tpl.BoardID)
	spaceACL := []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), tpl.BoardID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, "manage") {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	now := time.Now().UTC()
	columnID := tpl.ColumnID
	var subcolumnID *int64
	if payload.ColumnID != nil && *payload.ColumnID > 0 {
		column, err := h.svc.Store().GetColumn(r.Context(), *payload.ColumnID)
		if err != nil || column == nil {
			respondError(w, http.StatusBadRequest, "tasks.columnNotFound")
			return
		}
		if column.BoardID != tpl.BoardID {
			respondError(w, http.StatusBadRequest, "tasks.boardMismatch")
			return
		}
		columnID = column.ID
	}
	if payload.SubColumnID != nil {
		subcolumn, err := h.svc.Store().GetSubColumn(r.Context(), *payload.SubColumnID)
		if err != nil || subcolumn == nil || !subcolumn.IsActive {
			respondError(w, http.StatusBadRequest, "tasks.subcolumnNotFound")
			return
		}
		if subcolumn.ColumnID != columnID {
			respondError(w, http.StatusBadRequest, "tasks.columnMismatch")
			return
		}
		subcolumnID = payload.SubColumnID
	}
	title := strings.TrimSpace(tpl.TitleTemplate)
	if payload.Title != nil {
		if trimmed := strings.TrimSpace(*payload.Title); trimmed != "" {
			title = trimmed
		}
	}
	task := &tasks.Task{
		BoardID:     tpl.BoardID,
		ColumnID:    columnID,
		SubColumnID: subcolumnID,
		Title:       title,
		Description: strings.TrimSpace(tpl.DescriptionTemplate),
		Priority:    strings.ToLower(strings.TrimSpace(tpl.Priority)),
		TemplateID:  &tpl.ID,
		CreatedBy:   &user.ID,
		Checklist:   append([]tasks.TaskChecklistItem{}, tpl.ChecklistTemplate...),
		IsArchived:  false,
	}
	if tpl.DefaultDueDays > 0 {
		due := now.AddDate(0, 0, tpl.DefaultDueDays)
		task.DueDate = &due
	}
	links := []tasks.Link{}
	for _, lt := range tpl.LinksTemplate {
		targetType := strings.ToLower(strings.TrimSpace(lt.TargetType))
		targetID := strings.TrimSpace(lt.TargetID)
		if targetType == "" || targetID == "" {
			continue
		}
		links = append(links, tasks.Link{
			SourceType: "task",
			SourceID:   "",
			TargetType: targetType,
			TargetID:   targetID,
		})
	}
	if _, err := h.svc.Store().CreateTaskWithLinks(r.Context(), task, tpl.DefaultAssignees, links); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskCreate, fmt.Sprintf("%d", task.ID))
	if len(tpl.DefaultAssignees) > 0 {
		tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskAssign, fmt.Sprintf("%d", task.ID))
	}
	respondJSON(w, http.StatusCreated, buildTaskDTO(*task, nil, tpl.DefaultAssignees, nil, nil, true))
}

func normalizeChecklistTemplate(items []tasks.TaskChecklistItem) []tasks.TaskChecklistItem {
	out := make([]tasks.TaskChecklistItem, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		out = append(out, tasks.TaskChecklistItem{Text: text, Done: false})
	}
	return out
}

func (h *Handler) validateTemplateLinks(ctx context.Context, user *cstore.User, roles []string, userGroups []cstore.Group, eff cstore.EffectiveAccess, links []tasks.TaskTemplateLink) ([]tasks.TaskTemplateLink, error) {
	if len(links) == 0 {
		return nil, nil
	}
	var out []tasks.TaskTemplateLink
	for _, link := range links {
		targetType := strings.ToLower(strings.TrimSpace(link.TargetType))
		targetID := strings.TrimSpace(link.TargetID)
		if targetType == "" || targetID == "" {
			return nil, fmt.Errorf("tasks.templateLinkInvalid")
		}
		switch targetType {
		case "doc":
			docID, err := strconv.ParseInt(targetID, 10, 64)
			if err != nil || docID == 0 {
				return nil, fmt.Errorf("tasks.templateLinkInvalid")
			}
			doc, err := h.docsStore.GetDocument(ctx, docID)
			if err != nil || doc == nil {
				return nil, fmt.Errorf("tasks.templateLinkInvalid")
			}
			docACL, _ := h.docsStore.GetDocACL(ctx, doc.ID)
			var folderACL []cstore.ACLRule
			if doc.FolderID != nil {
				folderACL, _ = h.docsStore.GetFolderACL(ctx, *doc.FolderID)
			}
			if !h.docsSvc.CheckACL(user, roles, doc, docACL, folderACL, "view") || !h.canViewByClassification(eff, doc.ClassificationLevel, doc.ClassificationTags) {
				return nil, fmt.Errorf("tasks.templateLinkInvalid")
			}
		case "incident":
			incID, err := strconv.ParseInt(targetID, 10, 64)
			if err != nil || incID == 0 {
				return nil, fmt.Errorf("tasks.templateLinkInvalid")
			}
			inc, err := h.incidentsStore.GetIncident(ctx, incID)
			if err != nil || inc == nil || inc.DeletedAt != nil {
				return nil, fmt.Errorf("tasks.templateLinkInvalid")
			}
			incACL, _ := h.incidentsStore.GetIncidentACL(ctx, inc.ID)
			if !h.incidentsSvc.CheckACL(user, roles, incACL, "view") || !h.canViewByClassification(eff, inc.ClassificationLevel, inc.ClassificationTags) {
				return nil, fmt.Errorf("tasks.templateLinkInvalid")
			}
		case "task":
			taskID, err := strconv.ParseInt(targetID, 10, 64)
			if err != nil || taskID == 0 {
				return nil, fmt.Errorf("tasks.templateLinkInvalid")
			}
			task, err := h.svc.Store().GetTask(ctx, taskID)
			if err != nil || task == nil {
				return nil, fmt.Errorf("tasks.templateLinkInvalid")
			}
			board, _ := h.svc.Store().GetBoard(ctx, task.BoardID)
			spaceACL := []tasks.ACLRule{}
			if board != nil && board.SpaceID > 0 {
				spaceACL, _ = h.svc.Store().GetSpaceACL(ctx, board.SpaceID)
			}
			boardACL, _ := h.svc.Store().GetBoardACL(ctx, task.BoardID)
			if !boardAllowed(user, roles, userGroups, spaceACL, boardACL, "view") {
				return nil, fmt.Errorf("tasks.templateLinkInvalid")
			}
		case "control":
		default:
			return nil, fmt.Errorf("tasks.templateLinkInvalid")
		}
		out = append(out, tasks.TaskTemplateLink{TargetType: targetType, TargetID: targetID})
	}
	return out, nil
}
