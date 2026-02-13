package main

import (
	"context"
	"log"

	"berkut-scc/config"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		logger.Fatalf("db: %v", err)
	}
	defer db.Close()
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		logger.Fatalf("migrations: %v", err)
	}
	logger.Printf("migrations applied")
}
