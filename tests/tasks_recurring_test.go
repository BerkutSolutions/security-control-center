package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"berkut-scc/config"
	"berkut-scc/tasks"
)

func TestComputeNextRunAt(t *testing.T) {
	base := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC)
	next, err := tasks.ComputeNextRunAt(base, tasks.ScheduleDaily, json.RawMessage(`{}`), "09:00")
	if err != nil {
		t.Fatalf("daily: %v", err)
	}
	want := time.Date(2026, 1, 6, 9, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("daily expected %v, got %v", want, next)
	}

	weeklyCfg, _ := json.Marshal(tasks.WeeklyScheduleConfig{Weekdays: []int{1}})
	next, err = tasks.ComputeNextRunAt(base, tasks.ScheduleWeekly, weeklyCfg, "11:30")
	if err != nil {
		t.Fatalf("weekly: %v", err)
	}
	want = time.Date(2026, 1, 5, 11, 30, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("weekly expected %v, got %v", want, next)
	}

	monthlyCfg, _ := json.Marshal(tasks.MonthlyScheduleConfig{Day: 31})
	next, err = tasks.ComputeNextRunAt(time.Date(2026, 2, 10, 8, 0, 0, 0, time.UTC), tasks.ScheduleMonthly, monthlyCfg, "09:00")
	if err != nil {
		t.Fatalf("monthly: %v", err)
	}
	want = time.Date(2026, 2, 28, 9, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("monthly expected %v, got %v", want, next)
	}

	quarterCfg, _ := json.Marshal(tasks.MonthDayScheduleConfig{Month: 1, Day: 15})
	next, err = tasks.ComputeNextRunAt(time.Date(2026, 2, 10, 8, 0, 0, 0, time.UTC), tasks.ScheduleQuarterly, quarterCfg, "09:00")
	if err != nil {
		t.Fatalf("quarterly: %v", err)
	}
	want = time.Date(2026, 4, 15, 9, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("quarterly expected %v, got %v", want, next)
	}

	annualCfg, _ := json.Marshal(tasks.MonthDayScheduleConfig{Month: 12, Day: 31})
	next, err = tasks.ComputeNextRunAt(time.Date(2026, 12, 30, 8, 0, 0, 0, time.UTC), tasks.ScheduleAnnual, annualCfg, "09:00")
	if err != nil {
		t.Fatalf("annual: %v", err)
	}
	want = time.Date(2026, 12, 31, 9, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("annual expected %v, got %v", want, next)
	}
}

func TestRecurringSchedulerCreatesOnce(t *testing.T) {
	env := setupTasksEnv(t)
	defer env.cleanup()

	tpl := &tasks.TaskTemplate{
		BoardID:       env.board.ID,
		ColumnID:      env.todo.ID,
		TitleTemplate: "Daily check",
		Priority:      tasks.PriorityMedium,
		IsActive:      true,
		CreatedBy:     &env.admin.ID,
	}
	if _, err := env.tasksStore.CreateTaskTemplate(env.ctx, tpl); err != nil {
		t.Fatalf("template create: %v", err)
	}

	next := time.Now().UTC().Add(-1 * time.Minute)
	cfg, _ := tasks.NormalizeScheduleConfig(tasks.ScheduleDaily, json.RawMessage(`{}`))
	rule := &tasks.TaskRecurringRule{
		TemplateID:     tpl.ID,
		ScheduleType:   tasks.ScheduleDaily,
		ScheduleConfig: cfg,
		TimeOfDay:      "09:00",
		NextRunAt:      &next,
		IsActive:       true,
		CreatedBy:      &env.admin.ID,
	}
	if _, err := env.tasksStore.CreateTaskRecurringRule(env.ctx, rule); err != nil {
		t.Fatalf("rule create: %v", err)
	}

	scheduler := tasks.NewRecurringScheduler(config.SchedulerConfig{Enabled: true, IntervalSeconds: 1, MaxJobsPerTick: 5}, env.tasksStore, nil, nil)
	now := time.Now().UTC()
	if err := scheduler.RunOnce(env.ctx, now); err != nil {
		t.Fatalf("run once: %v", err)
	}
	if err := scheduler.RunOnce(env.ctx, now); err != nil {
		t.Fatalf("run twice: %v", err)
	}

	var count int
	row := env.tasksStore.DB().QueryRowContext(env.ctx, `SELECT COUNT(*) FROM tasks WHERE recurring_rule_id=?`, rule.ID)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 recurring task, got %d", count)
	}
}

func TestRecurringInstanceUnique(t *testing.T) {
	env := setupTasksEnv(t)
	defer env.cleanup()

	tpl := &tasks.TaskTemplate{
		BoardID:       env.board.ID,
		ColumnID:      env.todo.ID,
		TitleTemplate: "Weekly check",
		Priority:      tasks.PriorityMedium,
		IsActive:      true,
		CreatedBy:     &env.admin.ID,
	}
	if _, err := env.tasksStore.CreateTaskTemplate(env.ctx, tpl); err != nil {
		t.Fatalf("template create: %v", err)
	}
	cfg, _ := tasks.NormalizeScheduleConfig(tasks.ScheduleWeekly, json.RawMessage(`{"weekdays":[1]}`))
	rule := &tasks.TaskRecurringRule{
		TemplateID:     tpl.ID,
		ScheduleType:   tasks.ScheduleWeekly,
		ScheduleConfig: cfg,
		TimeOfDay:      "09:00",
		NextRunAt:      nil,
		IsActive:       true,
		CreatedBy:      &env.admin.ID,
	}
	if _, err := env.tasksStore.CreateTaskRecurringRule(env.ctx, rule); err != nil {
		t.Fatalf("rule create: %v", err)
	}
	scheduled := time.Now().UTC()
	_, created, err := env.tasksStore.CreateRecurringInstanceTask(env.ctx, rule, tpl, scheduled)
	if err != nil || !created {
		t.Fatalf("first run: created=%v err=%v", created, err)
	}
	_, created, err = env.tasksStore.CreateRecurringInstanceTask(env.ctx, rule, tpl, scheduled)
	if err != nil {
		t.Fatalf("second run err: %v", err)
	}
	if created {
		t.Fatalf("expected no duplicate instance")
	}
}

func TestRecurringPermissions(t *testing.T) {
	env := setupTasksEnv(t)
	defer env.cleanup()

	payload, _ := json.Marshal(map[string]any{
		"board_id":         env.board.ID,
		"column_id":        env.todo.ID,
		"title_template":   "Template",
		"priority":         "medium",
		"default_due_days": 1,
		"is_active":        true,
	})
	req := authedRequest("POST", "/api/tasks/templates", payload, env.analyst)
	rr := httptest.NewRecorder()
	env.handler.CreateTemplate(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected template forbidden, got %d", rr.Code)
	}

	reqAdmin := authedRequest("POST", "/api/tasks/templates", payload, env.admin)
	rrAdmin := httptest.NewRecorder()
	env.handler.CreateTemplate(rrAdmin, reqAdmin)
	if rrAdmin.Code != http.StatusCreated {
		t.Fatalf("expected template created, got %d", rrAdmin.Code)
	}
	var tpl tasks.TaskTemplate
	if err := json.Unmarshal(rrAdmin.Body.Bytes(), &tpl); err != nil || tpl.ID == 0 {
		t.Fatalf("template response invalid")
	}

	recPayload, _ := json.Marshal(map[string]any{
		"template_id":     tpl.ID,
		"schedule_type":   "daily",
		"schedule_config": map[string]any{},
		"time_of_day":     "09:00",
		"is_active":       true,
	})
	recReq := authedRequest("POST", "/api/tasks/recurring", recPayload, env.analyst)
	recRR := httptest.NewRecorder()
	env.handler.CreateRecurringRule(recRR, recReq)
	if recRR.Code != http.StatusForbidden {
		t.Fatalf("expected recurring forbidden, got %d", recRR.Code)
	}
}

func TestTemplateDueDateApplied(t *testing.T) {
	env := setupTasksEnv(t)
	defer env.cleanup()

	tpl := &tasks.TaskTemplate{
		BoardID:        env.board.ID,
		ColumnID:       env.todo.ID,
		TitleTemplate:  "Due test",
		Priority:       tasks.PriorityMedium,
		DefaultDueDays: 3,
		IsActive:       true,
		CreatedBy:      &env.admin.ID,
	}
	if _, err := env.tasksStore.CreateTaskTemplate(env.ctx, tpl); err != nil {
		t.Fatalf("template create: %v", err)
	}
	req := authedRequest("POST", "/api/tasks/templates/"+itoa(tpl.ID)+"/create-task", nil, env.admin)
	req = withURLParams(req, map[string]string{"id": itoa(tpl.ID)})
	rr := httptest.NewRecorder()
	env.handler.CreateTaskFromTemplate(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected create task, got %d", rr.Code)
	}
	var created tasks.TaskDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil || created.ID == 0 {
		t.Fatalf("invalid task response")
	}
	if created.DueDate == nil {
		t.Fatalf("expected due date to be set")
	}
	diff := created.DueDate.Sub(created.CreatedAt)
	if diff < 71*time.Hour || diff > 73*time.Hour {
		t.Fatalf("expected due date ~72h after created_at, got %v", diff)
	}
}

func itoa(id int64) string {
	return fmt.Sprintf("%d", id)
}
