package backups

import (
	"testing"
	"time"
)

func TestShouldRunByPlanDaily(t *testing.T) {
	plan := BackupPlan{ScheduleType: ScheduleDaily, ScheduleHour: 2, ScheduleMinute: 0}
	last := time.Date(2026, 2, 13, 2, 0, 0, 0, time.UTC)
	if !shouldRunByPlan(plan, &last, time.Date(2026, 2, 14, 2, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected daily run")
	}
	if shouldRunByPlan(plan, &last, time.Date(2026, 2, 13, 15, 0, 0, 0, time.UTC)) {
		t.Fatalf("must not run before next daily slot")
	}
}

func TestShouldRunByPlanWeeklySunday(t *testing.T) {
	plan := BackupPlan{ScheduleType: ScheduleWeekly, ScheduleWeekday: 0, ScheduleHour: 3, ScheduleMinute: 30}
	last := time.Date(2026, 2, 8, 3, 30, 0, 0, time.UTC) // sunday
	if !shouldRunByPlan(plan, &last, time.Date(2026, 2, 15, 3, 30, 0, 0, time.UTC)) {
		t.Fatalf("expected weekly sunday run")
	}
	if shouldRunByPlan(plan, &last, time.Date(2026, 2, 14, 3, 30, 0, 0, time.UTC)) {
		t.Fatalf("must not run before sunday")
	}
}

func TestShouldRunByPlanMonthlyEnd(t *testing.T) {
	plan := BackupPlan{ScheduleType: ScheduleMonthlyEnd, ScheduleHour: 1, ScheduleMinute: 0}
	last := time.Date(2026, 1, 31, 1, 0, 0, 0, time.UTC)
	if !shouldRunByPlan(plan, &last, time.Date(2026, 2, 28, 1, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected monthly end run")
	}
	if shouldRunByPlan(plan, &last, time.Date(2026, 2, 27, 1, 0, 0, 0, time.UTC)) {
		t.Fatalf("must not run before month end")
	}
}
