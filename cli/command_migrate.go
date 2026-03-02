package cli

import (
	"context"
	"flag"
	"fmt"

	"berkut-scc/config"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func runMigrate(args []string) {
	cmd := flag.NewFlagSet("migrate", flag.ExitOnError)
	_ = cmd.Parse(args)

	cfg, _ := config.Load()
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		logger.Fatalf("db: %v", err)
	}
	defer db.Close()
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		logger.Fatalf("migrations: %v", err)
	}
	fmt.Println("migrations applied")
}

