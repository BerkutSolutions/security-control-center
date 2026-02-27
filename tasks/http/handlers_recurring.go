package taskshttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"berkut-scc/tasks"
	"github.com/go-chi/chi/v5"
)

type recurringRuleDTO struct {
	tasks.TaskRecurringRule
	TemplateTitle  string `json:"template_title"`
	BoardID        int64  `json:"board_id"`
	ColumnID       int64  `json:"column_id"`
	TemplateActive bool   `json:"template_active"`
}

func (h *Handler) ListRecurringRules(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermRecurringView) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	filter := tasks.TaskRecurringRuleFilter{
		TemplateID:      parseInt64Default(r.URL.Query().Get("template_id"), 0),
		IncludeInactive: r.URL.Query().Get("include_inactive") == "1",
	}
	rules, err := h.svc.Store().ListTaskRecurringRules(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	templates, err := h.svc.Store().ListTaskTemplates(r.Context(), tasks.TaskTemplateFilter{IncludeInactive: true})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tplByID := map[int64]tasks.TaskTemplate{}
	for _, tpl := range templates {
		tplByID[tpl.ID] = tpl
	}
	boardACL := map[int64][]tasks.ACLRule{}
	boardInfo := map[int64]*tasks.Board{}
	spaceACL := map[int64][]tasks.ACLRule{}
	var res []recurringRuleDTO
	for _, rule := range rules {
		tpl, ok := tplByID[rule.TemplateID]
		if !ok {
			continue
		}
		acl, ok := boardACL[tpl.BoardID]
		if !ok {
			acl, _ = h.svc.Store().GetBoardACL(r.Context(), tpl.BoardID)
			boardACL[tpl.BoardID] = acl
		}
		board, ok := boardInfo[tpl.BoardID]
		if !ok {
			board, _ = h.svc.Store().GetBoard(r.Context(), tpl.BoardID)
			boardInfo[tpl.BoardID] = board
			if board != nil && board.SpaceID > 0 {
				if _, ok := spaceACL[board.SpaceID]; !ok {
					acl, _ := h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
					spaceACL[board.SpaceID] = acl
				}
			}
		}
		var spaceID int64
		if board != nil {
			spaceID = board.SpaceID
		}
		if !boardAllowed(user, roles, groups, spaceACL[spaceID], acl, "view") {
			continue
		}
		res = append(res, recurringRuleDTO{
			TaskRecurringRule: rule,
			TemplateTitle:     tpl.TitleTemplate,
			BoardID:           tpl.BoardID,
			ColumnID:          tpl.ColumnID,
			TemplateActive:    tpl.IsActive,
		})
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": res})
}

func (h *Handler) CreateRecurringRule(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermRecurringManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	var payload struct {
		TemplateID     int64           `json:"template_id"`
		ScheduleType   string          `json:"schedule_type"`
		ScheduleConfig json.RawMessage `json:"schedule_config"`
		TimeOfDay      string          `json:"time_of_day"`
		IsActive       *bool           `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	if payload.TemplateID == 0 {
		respondError(w, http.StatusBadRequest, "tasks.templateRequired")
		return
	}
	if strings.TrimSpace(payload.TimeOfDay) == "" {
		respondError(w, http.StatusBadRequest, "tasks.recurring.timeRequired")
		return
	}
	tpl, err := h.svc.Store().GetTaskTemplate(r.Context(), payload.TemplateID)
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
	normalized, err := tasks.NormalizeScheduleConfig(payload.ScheduleType, payload.ScheduleConfig)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	active := true
	if payload.IsActive != nil {
		active = *payload.IsActive
	}
	var nextRun *time.Time
	if active {
		next, err := tasks.ComputeNextRunAt(time.Now().UTC(), payload.ScheduleType, normalized, payload.TimeOfDay)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		nextRun = &next
	}
	rule := &tasks.TaskRecurringRule{
		TemplateID:     tpl.ID,
		ScheduleType:   strings.ToLower(strings.TrimSpace(payload.ScheduleType)),
		ScheduleConfig: normalized,
		TimeOfDay:      strings.TrimSpace(payload.TimeOfDay),
		NextRunAt:      nextRun,
		IsActive:       active,
		CreatedBy:      &user.ID,
	}
	if _, err := h.svc.Store().CreateTaskRecurringRule(r.Context(), rule); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditRecurringCreate, fmt.Sprintf("%d", rule.ID))
	respondJSON(w, http.StatusCreated, rule)
}

func (h *Handler) UpdateRecurringRule(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermRecurringManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	ruleID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if ruleID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	rule, err := h.svc.Store().GetTaskRecurringRule(r.Context(), ruleID)
	if err != nil || rule == nil {
		respondError(w, http.StatusNotFound, "tasks.recurringNotFound")
		return
	}
	tpl, err := h.svc.Store().GetTaskTemplate(r.Context(), rule.TemplateID)
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
		ScheduleType   *string         `json:"schedule_type"`
		ScheduleConfig json.RawMessage `json:"schedule_config"`
		TimeOfDay      *string         `json:"time_of_day"`
		IsActive       *bool           `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	if payload.ScheduleType != nil {
		rule.ScheduleType = strings.ToLower(strings.TrimSpace(*payload.ScheduleType))
	}
	if payload.TimeOfDay != nil {
		rule.TimeOfDay = strings.TrimSpace(*payload.TimeOfDay)
	}
	if payload.ScheduleConfig != nil {
		rule.ScheduleConfig = payload.ScheduleConfig
	}
	if payload.IsActive != nil {
		rule.IsActive = *payload.IsActive
	}
	normalized, err := tasks.NormalizeScheduleConfig(rule.ScheduleType, rule.ScheduleConfig)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	rule.ScheduleConfig = normalized
	if rule.IsActive {
		next, err := tasks.ComputeNextRunAt(time.Now().UTC(), rule.ScheduleType, rule.ScheduleConfig, rule.TimeOfDay)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		rule.NextRunAt = &next
	} else {
		rule.NextRunAt = nil
	}
	if err := h.svc.Store().UpdateTaskRecurringRule(r.Context(), rule); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditRecurringUpdate, fmt.Sprintf("%d", rule.ID))
	respondJSON(w, http.StatusOK, rule)
}

func (h *Handler) ToggleRecurringRule(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermRecurringManage) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	ruleID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if ruleID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	rule, err := h.svc.Store().GetTaskRecurringRule(r.Context(), ruleID)
	if err != nil || rule == nil {
		respondError(w, http.StatusNotFound, "tasks.recurringNotFound")
		return
	}
	tpl, err := h.svc.Store().GetTaskTemplate(r.Context(), rule.TemplateID)
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
		IsActive bool `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	rule.IsActive = payload.IsActive
	if rule.IsActive {
		next, err := tasks.ComputeNextRunAt(time.Now().UTC(), rule.ScheduleType, rule.ScheduleConfig, rule.TimeOfDay)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		rule.NextRunAt = &next
	} else {
		rule.NextRunAt = nil
	}
	if err := h.svc.Store().UpdateTaskRecurringRule(r.Context(), rule); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditRecurringToggle, fmt.Sprintf("%d|%t", rule.ID, rule.IsActive))
	respondJSON(w, http.StatusOK, rule)
}

func (h *Handler) RunRecurringRuleNow(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermRecurringRun) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	ruleID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if ruleID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	rule, err := h.svc.Store().GetTaskRecurringRule(r.Context(), ruleID)
	if err != nil || rule == nil {
		respondError(w, http.StatusNotFound, "tasks.recurringNotFound")
		return
	}
	tpl, err := h.svc.Store().GetTaskTemplate(r.Context(), rule.TemplateID)
	if err != nil || tpl == nil {
		respondError(w, http.StatusNotFound, "tasks.templateNotFound")
		return
	}
	if !tpl.IsActive {
		respondError(w, http.StatusConflict, "tasks.templateInactive")
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
	task, created, err := h.svc.Store().CreateRecurringInstanceTask(r.Context(), rule, tpl, now)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	next, err := tasks.ComputeNextRunAt(now, rule.ScheduleType, rule.ScheduleConfig, rule.TimeOfDay)
	if err == nil {
		_ = h.svc.Store().UpdateRecurringRuleRun(r.Context(), rule.ID, now, next)
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditRecurringRunNow, fmt.Sprintf("%d", rule.ID))
	if created && task != nil {
		tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskRecurringCreate, fmt.Sprintf("%d", task.ID))
		respondJSON(w, http.StatusCreated, buildTaskDTO(*task, nil, tpl.DefaultAssignees, nil, nil, true))
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
