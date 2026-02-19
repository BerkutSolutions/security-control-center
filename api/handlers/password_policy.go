package handlers

import (
	"strings"

	"berkut-scc/core/auth"
	"berkut-scc/core/store"
)

func invalidUsernameMessage(lang string) string {
	if lang == "ru" {
		return "\u041d\u0435\u0432\u0435\u0440\u043d\u044b\u0439 \u043b\u043e\u0433\u0438\u043d (3-32 \u0441\u0438\u043c\u0432\u043e\u043b\u0430: \u043b\u0430\u0442\u0438\u043d\u0438\u0446\u0430, \u0446\u0438\u0444\u0440\u044b, . _ -)"
	}
	return "Invalid username (3-32 chars: letters, digits, . _ -)"
}

func passwordPolicyMessage(lang string, err error) string {
	if err == nil {
		if lang == "ru" {
			return "\u041d\u0435\u0432\u0435\u0440\u043d\u044b\u0439 \u043f\u0430\u0440\u043e\u043b\u044c"
		}
		return "Invalid password"
	}
	msg := strings.TrimSpace(err.Error())
	switch msg {
	case "password too short (min 12 chars)":
		if lang == "ru" {
			return "\u041f\u0430\u0440\u043e\u043b\u044c \u0441\u043b\u0438\u0448\u043a\u043e\u043c \u043a\u043e\u0440\u043e\u0442\u043a\u0438\u0439 (\u043c\u0438\u043d\u0438\u043c\u0443\u043c 12 \u0441\u0438\u043c\u0432\u043e\u043b\u043e\u0432)"
		}
		return "Password is too short (min 12 chars)"
	case "password too long (max 128 chars)":
		if lang == "ru" {
			return "\u041f\u0430\u0440\u043e\u043b\u044c \u0441\u043b\u0438\u0448\u043a\u043e\u043c \u0434\u043b\u0438\u043d\u043d\u044b\u0439 (\u043c\u0430\u043a\u0441\u0438\u043c\u0443\u043c 128 \u0441\u0438\u043c\u0432\u043e\u043b\u043e\u0432)"
		}
		return "Password is too long (max 128 chars)"
	case "password must not contain spaces":
		if lang == "ru" {
			return "\u041f\u0430\u0440\u043e\u043b\u044c \u043d\u0435 \u0434\u043e\u043b\u0436\u0435\u043d \u0441\u043e\u0434\u0435\u0440\u0436\u0430\u0442\u044c \u043f\u0440\u043e\u0431\u0435\u043b\u044b"
		}
		return "Password must not contain spaces"
	case "password must include at least one uppercase letter":
		if lang == "ru" {
			return "\u041f\u0430\u0440\u043e\u043b\u044c \u0434\u043e\u043b\u0436\u0435\u043d \u0441\u043e\u0434\u0435\u0440\u0436\u0430\u0442\u044c \u0445\u043e\u0442\u044f \u0431\u044b \u043e\u0434\u043d\u0443 \u0437\u0430\u0433\u043b\u0430\u0432\u043d\u0443\u044e \u0431\u0443\u043a\u0432\u0443 (A-Z)"
		}
		return "Password must include at least one uppercase letter (A-Z)"
	case "password must include at least one lowercase letter":
		if lang == "ru" {
			return "\u041f\u0430\u0440\u043e\u043b\u044c \u0434\u043e\u043b\u0436\u0435\u043d \u0441\u043e\u0434\u0435\u0440\u0436\u0430\u0442\u044c \u0445\u043e\u0442\u044f \u0431\u044b \u043e\u0434\u043d\u0443 \u0441\u0442\u0440\u043e\u0447\u043d\u0443\u044e \u0431\u0443\u043a\u0432\u0443 (a-z)"
		}
		return "Password must include at least one lowercase letter (a-z)"
	case "password must include at least one digit":
		if lang == "ru" {
			return "\u041f\u0430\u0440\u043e\u043b\u044c \u0434\u043e\u043b\u0436\u0435\u043d \u0441\u043e\u0434\u0435\u0440\u0436\u0430\u0442\u044c \u0445\u043e\u0442\u044f \u0431\u044b \u043e\u0434\u043d\u0443 \u0446\u0438\u0444\u0440\u0443 (0-9)"
		}
		return "Password must include at least one digit (0-9)"
	case "password must include at least one special character (!@#$%^&*_-+=)":
		if lang == "ru" {
			return "\u041f\u0430\u0440\u043e\u043b\u044c \u0434\u043e\u043b\u0436\u0435\u043d \u0441\u043e\u0434\u0435\u0440\u0436\u0430\u0442\u044c \u0445\u043e\u0442\u044f \u0431\u044b \u043e\u0434\u0438\u043d \u0441\u043f\u0435\u0446\u0441\u0438\u043c\u0432\u043e\u043b (!@#$%^&*_-+=)"
		}
		return "Password must include at least one special character (!@#$%^&*_-+=)"
	default:
		if lang == "ru" {
			return "\u041d\u0435\u0432\u0435\u0440\u043d\u044b\u0439 \u043f\u0430\u0440\u043e\u043b\u044c"
		}
		return "Invalid password"
	}
}

func (h *AccountsHandler) canAssignTagsEff(actorEff store.EffectiveAccess, tags []string) bool {
	if containsRole(actorEff.Roles, "superadmin") {
		return true
	}
	if !h.cfg.Security.TagsSubsetEnforced {
		return true
	}
	set := map[string]struct{}{}
	for _, t := range actorEff.ClearanceTags {
		set[strings.ToLower(t)] = struct{}{}
	}
	for _, t := range tags {
		if _, ok := set[strings.ToLower(t)]; !ok {
			return false
		}
	}
	return true
}

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
