package tasks

import (
	"context"
	"fmt"
	"time"

	"berkut-scc/config"
	cstore "berkut-scc/core/store"
	"berkut-scc/core/utils"
)

type RecurringScheduler struct {
	cfg     config.SchedulerConfig
	store   Store
	audits  cstore.AuditStore
	logger  *utils.Logger
	stopCh  chan struct{}
	doneCh  chan struct{}
}

func NewRecurringScheduler(cfg config.SchedulerConfig, store Store, audits cstore.AuditStore, logger *utils.Logger) *RecurringScheduler {
	return &RecurringScheduler{
		cfg:    cfg,
		store:  store,
		audits: audits,
		logger: logger,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

func (s *RecurringScheduler) Start() {
	if s == nil || s.store == nil || !s.cfg.Enabled {
		return
	}
	interval := time.Duration(s.cfg.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer close(s.doneCh)
		for {
			select {
			case <-ticker.C:
				_ = s.RunOnce(context.Background(), time.Now().UTC())
			case <-s.stopCh:
				ticker.Stop()
				return
			}
		}
	}()
}

func (s *RecurringScheduler) Stop() {
	if s == nil || !s.cfg.Enabled {
		return
	}
	close(s.stopCh)
	<-s.doneCh
}

func (s *RecurringScheduler) RunOnce(ctx context.Context, now time.Time) error {
	if s == nil || s.store == nil || !s.cfg.Enabled {
		return nil
	}
	limit := s.cfg.MaxJobsPerTick
	if limit <= 0 {
		limit = 20
	}
	rules, err := s.store.ListDueRecurringRules(ctx, now.UTC(), limit)
	if err != nil {
		s.logError("recurring.list", err)
		return err
	}
	for _, rule := range rules {
		if rule.NextRunAt == nil {
			continue
		}
		tpl, err := s.store.GetTaskTemplate(ctx, rule.TemplateID)
		if err != nil || tpl == nil {
			s.logError("recurring.template", err)
			continue
		}
		if !tpl.IsActive {
			next, err := ComputeNextRunAt(*rule.NextRunAt, rule.ScheduleType, rule.ScheduleConfig, rule.TimeOfDay)
			if err == nil {
				_ = s.store.UpdateRecurringRuleRun(ctx, rule.ID, *rule.NextRunAt, next)
			}
			continue
		}
		scheduledFor := rule.NextRunAt.UTC()
		nextRun, err := ComputeNextRunAt(scheduledFor, rule.ScheduleType, rule.ScheduleConfig, rule.TimeOfDay)
		if err != nil {
			s.logError("recurring.next_run", err)
			continue
		}
		task, created, err := s.store.CreateRecurringInstanceTask(ctx, &rule, tpl, scheduledFor)
		if err != nil {
			s.logError("recurring.create", err)
			continue
		}
		if err := s.store.UpdateRecurringRuleRun(ctx, rule.ID, scheduledFor, nextRun); err != nil {
			s.logError("recurring.update", err)
		}
		if created && task != nil {
			Log(s.audits, ctx, "scheduler", AuditTaskRecurringCreate, fmt.Sprintf("%d", task.ID))
		}
	}
	return nil
}

func (s *RecurringScheduler) logError(scope string, err error) {
	if s.logger == nil || err == nil {
		return
	}
	s.logger.Errorf("scheduler %s: %v", scope, err)
}
