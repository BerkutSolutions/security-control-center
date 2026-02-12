package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"berkut-scc/api"
	"berkut-scc/config"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
	"berkut-scc/core/bootstrap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		logger.Fatalf("db init: %v", err)
	}
	defer db.Close()

	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		logger.Fatalf("migrations: %v", err)
	}

	if err := bootstrap.EnsureDefaultAdmin(context.Background(), db, cfg, logger); err != nil {
		logger.Fatalf("seed admin: %v", err)
	}

	srv := api.NewServer(cfg, db, logger)
	go func() {
		if err := srv.Start(); err != nil {
			logger.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Stop(ctx); err != nil {
		logger.Errorf("graceful shutdown: %v", err)
	}
}
