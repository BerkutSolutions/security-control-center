package api

import (
	"context"
	"errors"

	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

type BackgroundWorker interface {
	StartWithContext(context.Context)
	StopWithContext(context.Context) error
}

type BackgroundController interface {
	Start(context.Context)
	Stop(context.Context) error
}

type backgroundManager struct {
	sessions        store.SessionStore
	revokeOnStartup bool
	logger          *utils.Logger
	workers         []BackgroundWorker
}

func newBackgroundManager(sessions store.SessionStore, revokeOnStartup bool, logger *utils.Logger, workers ...BackgroundWorker) *backgroundManager {
	out := make([]BackgroundWorker, 0, len(workers))
	for _, w := range workers {
		if w == nil {
			continue
		}
		out = append(out, w)
	}
	return &backgroundManager{
		sessions:        sessions,
		revokeOnStartup: revokeOnStartup,
		logger:          logger,
		workers:         out,
	}
}

func BuildBackgroundController(sessions store.SessionStore, revokeOnStartup bool, logger *utils.Logger, workers ...BackgroundWorker) BackgroundController {
	return newBackgroundManager(sessions, revokeOnStartup, logger, workers...)
}

func (m *backgroundManager) Start(ctx context.Context) {
	if m == nil {
		return
	}
	if m.revokeOnStartup && m.sessions != nil {
		if err := m.sessions.DeleteAll(ctx, "system_startup"); err != nil && m.logger != nil {
			m.logger.Errorf("revoke sessions on startup: %v", err)
		}
	}
	for _, w := range m.workers {
		w.StartWithContext(ctx)
	}
}

func (m *backgroundManager) Stop(ctx context.Context) error {
	if m == nil {
		return nil
	}
	var errs []error
	for _, w := range m.workers {
		if err := w.StopWithContext(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
