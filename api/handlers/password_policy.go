package handlers

import (
	"berkut-scc/core/auth"
	"berkut-scc/core/store"
)

func isPasswordReused(password, pepper string, user *store.User, history []store.PasswordHistoryEntry) bool {
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
