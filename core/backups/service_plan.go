package backups

import (
	"context"
	"strings"
	"time"
)

func (s *Service) GetPlan(ctx context.Context) (*BackupPlan, error) {
	if s == nil || s.repo == nil {
		return nil, NewDomainError(ErrorCodeInternal, "common.serverError")
	}
	plan, err := s.repo.GetBackupPlan(ctx)
	if err != nil {
		return nil, err
	}
	return normalizePlan(plan), nil
}

func (s *Service) UpdatePlan(ctx context.Context, plan BackupPlan, requestedBy string) (*BackupPlan, error) {
	if s == nil || s.repo == nil {
		return nil, NewDomainError(ErrorCodeInternal, "common.serverError")
	}
	normalized, err := s.normalizeAndValidatePlan(plan)
	if err != nil {
		return nil, err
	}
	saved, err := s.repo.UpsertBackupPlan(ctx, normalized)
	if err != nil {
		return nil, err
	}
	return normalizePlan(saved), nil
}

func (s *Service) EnablePlan(ctx context.Context, requestedBy string) (*BackupPlan, error) {
	if s == nil || s.repo == nil {
		return nil, NewDomainError(ErrorCodeInternal, "common.serverError")
	}
	if err := s.repo.SetBackupPlanEnabled(ctx, true); err != nil {
		return nil, err
	}
	return s.GetPlan(ctx)
}

func (s *Service) DisablePlan(ctx context.Context, requestedBy string) (*BackupPlan, error) {
	if s == nil || s.repo == nil {
		return nil, NewDomainError(ErrorCodeInternal, "common.serverError")
	}
	if err := s.repo.SetBackupPlanEnabled(ctx, false); err != nil {
		return nil, err
	}
	return s.GetPlan(ctx)
}

func (s *Service) normalizeAndValidatePlan(in BackupPlan) (*BackupPlan, error) {
	plan := normalizePlan(&in)
	switch plan.ScheduleType {
	case ScheduleDaily, ScheduleWeekly, ScheduleMonthlyStart, ScheduleMonthlyEnd:
	default:
		return nil, NewDomainError(ErrorCodeInvalidPlan, ErrorKeyInvalidPlan)
	}
	if plan.ScheduleType == ScheduleWeekly {
		if plan.ScheduleWeekday < 0 || plan.ScheduleWeekday > 6 {
			return nil, NewDomainError(ErrorCodeInvalidPlan, ErrorKeyInvalidPlan)
		}
	}
	if plan.ScheduleType == ScheduleMonthlyStart {
		plan.ScheduleMonthAnchor = MonthAnchorStart
	}
	if plan.ScheduleType == ScheduleMonthlyEnd {
		plan.ScheduleMonthAnchor = MonthAnchorEnd
	}
	if plan.ScheduleHour < 0 || plan.ScheduleHour > 23 || plan.ScheduleMinute < 0 || plan.ScheduleMinute > 59 {
		return nil, NewDomainError(ErrorCodeInvalidPlan, ErrorKeyInvalidPlan)
	}
	plan.CronExpression = scheduleCronExpression(*plan)
	return plan, nil
}

func normalizePlan(in *BackupPlan) *BackupPlan {
	if in == nil {
		now := time.Now().UTC()
		return &BackupPlan{
			ID:                 1,
			Enabled:            false,
			CronExpression:     "0 2 * * *",
			RetentionDays:      30,
			KeepLastSuccessful: 5,
			IncludeFiles:       false,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
	}
	out := *in
	if strings.TrimSpace(out.CronExpression) == "" {
		out.CronExpression = "0 2 * * *"
	}
	if strings.TrimSpace(out.ScheduleType) == "" {
		out.ScheduleType = ScheduleDaily
	}
	if out.ScheduleWeekday < 0 || out.ScheduleWeekday > 6 {
		out.ScheduleWeekday = 0
	}
	if out.ScheduleHour < 0 || out.ScheduleHour > 23 {
		out.ScheduleHour = 2
	}
	if out.ScheduleMinute < 0 || out.ScheduleMinute > 59 {
		out.ScheduleMinute = 0
	}
	if strings.TrimSpace(out.ScheduleMonthAnchor) == "" {
		out.ScheduleMonthAnchor = MonthAnchorStart
	}
	if out.RetentionDays <= 0 {
		out.RetentionDays = 30
	}
	if out.KeepLastSuccessful <= 0 {
		out.KeepLastSuccessful = 5
	}
	if out.ID <= 0 {
		out.ID = 1
	}
	out.CronExpression = scheduleCronExpression(out)
	return &out
}
