package tests

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func newTestStores(t *testing.T) (store.UsersStore, func()) {
	dir := t.TempDir()
	cfg := &config.AppConfig{DBPath: filepath.Join(dir, "policy.db"), Pepper: "pepper"}
	logger := utils.NewLogger()
	db, err := store.NewDB(cfg, logger)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.ApplyMigrations(context.Background(), db, logger); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return store.NewUsersStore(db), func() { db.Close() }
}

func TestPasswordHistoryLimit(t *testing.T) {
	users, cleanup := newTestStores(t)
	defer cleanup()
	ph := auth.MustHashPassword("p1", "pepper")
	u := &store.User{Username: "u1", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	uid, err := users.Create(context.Background(), u, []string{"admin"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	for i := 0; i < 12; i++ {
		p := auth.MustHashPassword("p"+string('a'+rune(i)), "pepper")
		if err := users.UpdatePassword(context.Background(), uid, p.Hash, p.Salt, false); err != nil {
			t.Fatalf("update %d: %v", i, err)
		}
	}
	history, err := users.PasswordHistory(context.Background(), uid, 20)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(history) != 10 {
		t.Fatalf("expected 10 history entries, got %d", len(history))
	}
}

func TestPasswordReuseDenied(t *testing.T) {
	users, cleanup := newTestStores(t)
	defer cleanup()
	ph := auth.MustHashPassword("p1", "pepper")
	u := &store.User{Username: "u2", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	uid, _ := users.Create(context.Background(), u, []string{"admin"})
	history, _ := users.PasswordHistory(context.Background(), uid, 10)
	if !isPasswordReusedHelper("p1", "pepper", u, history) {
		t.Fatalf("expected reuse detected")
	}
}

func TestAdminResetSetsMustChange(t *testing.T) {
	users, cleanup := newTestStores(t)
	defer cleanup()
	ph := auth.MustHashPassword("p1", "pepper")
	u := &store.User{Username: "admin1", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true}
	uid, _ := users.Create(context.Background(), u, []string{"admin"})
	ph2 := auth.MustHashPassword("p2", "pepper")
	if err := users.UpdatePassword(context.Background(), uid, ph2.Hash, ph2.Salt, true); err != nil {
		t.Fatalf("reset: %v", err)
	}
	u2, _, _ := users.Get(context.Background(), uid)
	if !u2.RequirePasswordChange {
		t.Fatalf("expected must change password")
	}
}

func TestUserChangeClearsMustChange(t *testing.T) {
	users, cleanup := newTestStores(t)
	defer cleanup()
	ph := auth.MustHashPassword("p1", "pepper")
	u := &store.User{Username: "user3", PasswordHash: ph.Hash, Salt: ph.Salt, PasswordSet: true, Active: true, RequirePasswordChange: true}
	uid, _ := users.Create(context.Background(), u, []string{"admin"})
	ph2 := auth.MustHashPassword("p2", "pepper")
	if err := users.UpdatePassword(context.Background(), uid, ph2.Hash, ph2.Salt, false); err != nil {
		t.Fatalf("change: %v", err)
	}
	u2, _, _ := users.Get(context.Background(), uid)
	if u2.RequirePasswordChange {
		t.Fatalf("expected must change cleared")
	}
	if u2.PasswordChangedAt == nil || time.Since(*u2.PasswordChangedAt) > time.Minute {
		t.Fatalf("password_changed_at not set")
	}
}

// Helper mirrors handler logic.
func isPasswordReusedHelper(password, pepper string, user *store.User, history []store.PasswordHistoryEntry) bool {
	check := func(hash, salt string) bool {
		ph, err := auth.ParsePasswordHash(hash, salt)
		if err != nil {
			return false
		}
		ok, _ := auth.VerifyPassword(password, pepper, ph)
		return ok
	}
	if user != nil && user.PasswordHash != "" && user.Salt != "" {
		if check(user.PasswordHash, user.Salt) {
			return true
		}
	}
	for _, h := range history {
		if check(h.Hash, h.Salt) {
			return true
		}
	}
	return false
}
