package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func Run() {
	createUserCmd := flag.NewFlagSet("create-user", flag.ExitOnError)
	username := createUserCmd.String("u", "", "username")
	password := createUserCmd.String("p", "", "password")
	roles := createUserCmd.String("r", "admin", "comma separated roles")

	if len(os.Args) < 2 {
		fmt.Println("commands: create-user")
		return
	}

	switch os.Args[1] {
	case "create-user":
		_ = createUserCmd.Parse(os.Args[2:])
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
		us := store.NewUsersStore(db)
		ph := auth.MustHashPassword(*password, cfg.Pepper)
		_, err = us.Create(context.Background(), *username, ph.Hash, ph.Salt, splitRoles(*roles))
		if err != nil {
			logger.Fatalf("create: %v", err)
		}
		fmt.Println("user created")
	default:
		fmt.Println("unknown command")
	}
}

func splitRoles(r string) []string {
	var res []string
	for _, part := range strings.Split(r, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			res = append(res, part)
		}
	}
	return res
}
