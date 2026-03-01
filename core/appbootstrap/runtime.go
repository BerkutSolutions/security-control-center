package appbootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"berkut-scc/api"
	"berkut-scc/config"
	"berkut-scc/core/bootstrap"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

type Runtime struct {
	DB         *sql.DB
	Server     *api.Server
	background api.BackgroundController

	mu       sync.Mutex
	bgCancel context.CancelFunc
}

func InitRuntime(ctx context.Context, cfg *config.AppConfig, logger *utils.Logger) (*Runtime, error) {
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("db init: %w", err)
	}
	composition, err := composeRuntime(cfg, db, logger)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("compose runtime: %w", err)
	}
	if err := bootstrap.EnsureDefaultAdmin(ctx, db, cfg, logger); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("seed admin: %w", err)
	}
	srv := api.NewServer(cfg, logger, composition.serverDeps)
	return &Runtime{
		DB:         db,
		Server:     srv,
		background: api.BuildBackgroundController(composition.sessions, cfg != nil && cfg.RunMode == "all", logger, composition.workers...),
	}, nil
}

func (r *Runtime) StartBackground(ctx context.Context) {
	if r == nil || r.background == nil || r.Server == nil || r.Server.Config() == nil {
		return
	}
	// In HA deployments, "api" nodes must not run background workers.
	if r.Server.Config().RunMode == "api" {
		return
	}
	r.mu.Lock()
	if r.bgCancel != nil {
		r.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	r.bgCancel = cancel
	r.mu.Unlock()
	r.background.Start(runCtx)
}

func (r *Runtime) StopBackground(ctx context.Context) error {
	if r == nil || r.background == nil {
		return nil
	}
	r.mu.Lock()
	cancel := r.bgCancel
	r.bgCancel = nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return r.background.Stop(ctx)
}
