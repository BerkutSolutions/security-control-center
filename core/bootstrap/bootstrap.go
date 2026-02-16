package bootstrap

import (
	"context"
	"database/sql"
	"strings"

	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/docs"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

// EnsureDefaultAdmin ensures admin user exists.
func EnsureDefaultAdmin(ctx context.Context, db *sql.DB, cfg *config.AppConfig, logger *utils.Logger) error {
	us := store.NewUsersStore(db)
	return EnsureDefaultAdminWithStore(ctx, us, cfg, logger)
}

// EnsureDefaultAdminWithStore ensures admin user exists using the provided store (useful outside main bootstrap).
func EnsureDefaultAdminWithStore(ctx context.Context, us store.UsersStore, cfg *config.AppConfig, logger *utils.Logger) error {
	existing, roles, err := us.FindByUsername(ctx, "admin")
	if err != nil {
		return err
	}
	if existing != nil {
		desiredLevel := int(docs.ClassificationSpecialImportance)
		desiredTags := docs.TagList
		needsClearance := existing.ClearanceLevel < desiredLevel || !hasAllTags(existing.ClearanceTags, desiredTags)
		rolesUpdated, nextRoles := ensureRole(roles, "superadmin")
		if needsClearance {
			existing.ClearanceLevel = desiredLevel
			existing.ClearanceTags = desiredTags
		}
		if needsClearance || rolesUpdated {
			var roleUpdate []string
			if rolesUpdated {
				roleUpdate = nextRoles
			}
			if err := us.Update(ctx, existing, roleUpdate); err != nil && logger != nil {
				logger.Printf("default admin update failed: %v", err)
			}
		}
		return nil
	}
	ph := auth.MustHashPassword("admin", cfg.Pepper)
	u := &store.User{
		Username:        "admin",
		FullName:        "Default Administrator",
		Department:      "Security",
		Position:        "Administrator",
		ClearanceLevel:  int(docs.ClassificationSpecialImportance),
		ClearanceTags:   docs.TagList,
		PasswordHash:    ph.Hash,
		Salt:            ph.Salt,
		PasswordSet:     false,
		Active:          true,
	}
	_, err = us.Create(ctx, u, []string{"superadmin"})
	if err == nil && logger != nil {
		logger.Printf("default admin created; password must be changed")
	}
	return err
}

func hasAllTags(current []string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(current))
	for _, tag := range current {
		set[strings.ToUpper(strings.TrimSpace(tag))] = struct{}{}
	}
	for _, tag := range required {
		if _, ok := set[strings.ToUpper(strings.TrimSpace(tag))]; !ok {
			return false
		}
	}
	return true
}

func ensureRole(current []string, role string) (bool, []string) {
	for _, r := range current {
		if strings.EqualFold(strings.TrimSpace(r), role) {
			return false, current
		}
	}
	next := append([]string{}, current...)
	next = append(next, role)
	return true, next
}
