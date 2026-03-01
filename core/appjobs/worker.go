package appjobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

const (
	StatusQueued   = "queued"
	StatusRunning  = "running"
	StatusFinished = "finished"
	StatusFailed   = "failed"
	StatusCanceled = "canceled"
)

var errJobCanceled = errors.New("job canceled")

type Worker struct {
	cfg       *config.AppConfig
	db        *sql.DB
	jobs      store.AppJobsStore
	modules   store.AppModuleStateStore
	audits    store.AuditStore
	logger    *utils.Logger
	registry  *Registry

	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
}

func NewWorker(cfg *config.AppConfig, db *sql.DB, jobs store.AppJobsStore, modules store.AppModuleStateStore, audits store.AuditStore, logger *utils.Logger) *Worker {
	return &Worker{
		cfg:      cfg,
		db:       db,
		jobs:     jobs,
		modules: modules,
		audits:   audits,
		logger:   logger,
		registry: DefaultModuleRegistry(),
	}
}

func (w *Worker) StartWithContext(ctx context.Context) {
	if w == nil {
		return
	}
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.running = true
	w.mu.Unlock()
	go w.loop(runCtx)
}

func (w *Worker) StopWithContext(ctx context.Context) error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	cancel := w.cancel
	w.cancel = nil
	w.running = false
	w.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (w *Worker) loop(ctx context.Context) {
	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.tick(ctx); err != nil && w.logger != nil {
				w.logger.Errorf("app jobs tick: %v", err)
			}
		}
	}
}

func (w *Worker) tick(ctx context.Context) error {
	if w == nil || w.jobs == nil {
		return nil
	}
	job, err := w.jobs.NextQueued(ctx)
	if err != nil || job == nil {
		return err
	}
	now := time.Now().UTC()
	claimed, err := w.jobs.MarkRunning(ctx, job.ID, now)
	if err != nil || !claimed {
		return err
	}
	job.Status = StatusRunning
	job.StartedAt = &now
	job.LogJSON = appendLog(job.LogJSON, LogEntry{TS: now, Level: "info", Msg: "job started"})
	_ = w.jobs.UpdateProgress(ctx, job.ID, 0, job.LogJSON)

	_ = w.logAudit(ctx, job.StartedBy, "app.job.start", auditDetails(job, nil))
	if w.logger != nil {
		w.logger.Printf("app job start id=%d type=%s scope=%s module=%s mode=%s by=%s", job.ID, job.Type, job.Scope, job.ModuleID, job.Mode, job.StartedBy)
	}

	if err := w.run(ctx, job); err != nil {
		if errors.Is(err, errJobCanceled) {
			// Status/audit already recorded in run().
			return nil
		}
		finished := time.Now().UTC()
		job.LogJSON = appendLog(job.LogJSON, LogEntry{TS: finished, Level: "error", Msg: "job failed", Fields: map[string]any{"error": err.Error()}})
		_ = w.jobs.Finish(ctx, job.ID, StatusFailed, finished, 100, job.LogJSON)
		_ = w.logAudit(ctx, job.StartedBy, "app.job.finish", auditDetails(job, map[string]any{"status": StatusFailed, "error": err.Error()}))
		if w.logger != nil {
			w.logger.Errorf("app job failed id=%d: %v", job.ID, err)
		}
		return nil
	}

	finished := time.Now().UTC()
	job.LogJSON = appendLog(job.LogJSON, LogEntry{TS: finished, Level: "info", Msg: "job finished"})
	_ = w.jobs.Finish(ctx, job.ID, StatusFinished, finished, 100, job.LogJSON)
	_ = w.logAudit(ctx, job.StartedBy, "app.job.finish", auditDetails(job, map[string]any{"status": StatusFinished}))
	if w.logger != nil {
		w.logger.Printf("app job finished id=%d", job.ID)
	}
	return nil
}

func (w *Worker) run(ctx context.Context, job *store.AppJob) error {
	if job == nil {
		return errors.New("nil job")
	}
	modules := w.resolveModules(job)
	if len(modules) == 0 {
		return errors.New("no modules selected")
	}
	for i, moduleID := range modules {
		if isCanceled(ctx, w.jobs, job.ID) {
			now := time.Now().UTC()
			job.LogJSON = appendLog(job.LogJSON, LogEntry{TS: now, Level: "warn", Msg: "job canceled"})
			_ = w.jobs.Finish(ctx, job.ID, StatusCanceled, now, 100, job.LogJSON)
			_ = w.logAudit(ctx, job.StartedBy, "app.job.finish", auditDetails(job, map[string]any{"status": StatusCanceled}))
			return errJobCanceled
		}

		progress := int(float64(i) / float64(len(modules)) * 100)
		if progress < 0 {
			progress = 0
		}
		if progress > 99 {
			progress = 99
		}
		job.LogJSON = appendLog(job.LogJSON, LogEntry{TS: time.Now().UTC(), Level: "info", Msg: "processing module", Fields: map[string]any{"module_id": moduleID}})
		_ = w.jobs.UpdateProgress(ctx, job.ID, progress, job.LogJSON)

		result, err := w.applyModuleAction(ctx, job, moduleID)
		if err != nil {
			return err
		}
		job.LogJSON = appendLog(job.LogJSON, LogEntry{
			TS:    time.Now().UTC(),
			Level: "info",
			Msg:   "module completed",
			Fields: map[string]any{
				"module_id": moduleID,
				"counts":    result.Counts,
				"files":     result.FilesCounts,
			},
		})
		_ = w.jobs.UpdateProgress(ctx, job.ID, progress, job.LogJSON)
		event := "app.module.reset.partial"
		if strings.EqualFold(strings.TrimSpace(job.Mode), "full") {
			event = "app.module.reset.full"
		}
		_ = w.logAudit(ctx, job.StartedBy, event, auditDetails(job, map[string]any{"module_id": moduleID, "counts": result.Counts, "files": result.FilesCounts}))
	}
	_ = w.jobs.UpdateProgress(ctx, job.ID, 99, job.LogJSON)
	return nil
}

func (w *Worker) resolveModules(job *store.AppJob) []string {
	if job == nil {
		return nil
	}
	scope := strings.ToLower(strings.TrimSpace(job.Scope))
	if scope == "all" {
		if w.registry == nil {
			return nil
		}
		return w.registry.IDs()
	}
	id := strings.TrimSpace(job.ModuleID)
	if id == "" {
		return nil
	}
	if w.registry == nil || w.registry.Get(id) == nil {
		return nil
	}
	return []string{id}
}

func (w *Worker) applyModuleAction(ctx context.Context, job *store.AppJob, moduleID string) (ModuleResult, error) {
	if w == nil || w.registry == nil {
		return ModuleResult{}, errors.New("module registry not configured")
	}
	mod := w.registry.Get(moduleID)
	if mod == nil {
		return ModuleResult{}, errors.New("unknown module")
	}
	if w.db == nil {
		return ModuleResult{}, errors.New("db not configured")
	}

	deps := ModuleDeps{
		DB:      w.db,
		Cfg:     w.cfg,
		Modules: w.modules,
		NowUTC:  time.Now().UTC(),
	}
	isFull := strings.EqualFold(strings.TrimSpace(job.Mode), "full")
	jobType := strings.ToLower(strings.TrimSpace(job.Type))
	if jobType == "adapt" && isFull {
		return ModuleResult{}, errors.New("adapt job cannot be full reset")
	}
	if isFull {
		if !mod.HasFullReset() {
			return ModuleResult{}, errors.New("full reset is not supported for this module")
		}
		res, err := mod.FullReset(ctx, deps)
		return normalizeModuleResult(res), err
	}
	res, err := mod.PartialAdapt(ctx, deps)
	return normalizeModuleResult(res), err
}

func normalizeModuleResult(res ModuleResult) ModuleResult {
	res.Counts = ensureCounts(res.Counts)
	res.FilesCounts = ensureFileCounts(res.FilesCounts)
	return res
}

type LogEntry struct {
	TS     time.Time       `json:"ts"`
	Level  string          `json:"level"`
	Msg    string          `json:"msg"`
	Fields map[string]any  `json:"fields,omitempty"`
}

func appendLog(logJSON string, entry LogEntry) string {
	var items []LogEntry
	raw := strings.TrimSpace(logJSON)
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &items)
	}
	items = append(items, entry)
	b, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func auditDetails(job *store.AppJob, extra map[string]any) string {
	fields := map[string]any{
		"job_id":    job.ID,
		"type":      job.Type,
		"scope":     job.Scope,
		"module_id": job.ModuleID,
		"mode":      job.Mode,
	}
	for k, v := range extra {
		fields[k] = v
	}
	b, _ := json.Marshal(fields)
	return string(b)
}

func (w *Worker) logAudit(ctx context.Context, username, action, details string) error {
	if w == nil || w.audits == nil {
		return nil
	}
	return w.audits.Log(ctx, username, action, details)
}

func isCanceled(ctx context.Context, jobs store.AppJobsStore, id int64) bool {
	if jobs == nil || id <= 0 {
		return false
	}
	job, err := jobs.Get(ctx, id)
	if err != nil || job == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(job.Status), StatusCanceled)
}
