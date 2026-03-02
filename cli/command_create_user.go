package cli

import (
	"context"
	"flag"
	"fmt"

	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func runCreateUser(args []string) {
	cmd := flag.NewFlagSet("create-user", flag.ExitOnError)
	username := cmd.String("u", "", "username")
	password := cmd.String("p", "", "password")
	roles := cmd.String("r", "admin", "comma separated roles")
	_ = cmd.Parse(args)

	cfg, _ := config.Load()
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		logger.Fatalf("db: %v", err)
	}
	defer db.Close()

	us := store.NewUsersStore(db)
	ph := auth.MustHashPassword(*password, cfg.Pepper)
	user := &store.User{
		Username:              *username,
		PasswordHash:          ph.Hash,
		Salt:                  ph.Salt,
		PasswordSet:           true,
		RequirePasswordChange: true,
		Active:                true,
	}
	if _, err := us.Create(context.Background(), user, splitRoles(*roles)); err != nil {
		logger.Fatalf("create: %v", err)
	}
	fmt.Println("user created")
}

