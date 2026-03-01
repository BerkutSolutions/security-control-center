package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"berkut-scc/core/auth"
	"berkut-scc/core/store"
)

func (h *AccountsHandler) ResetUser2FA(w http.ResponseWriter, r *http.Request) {
	lang := preferredLang(r)
	sr := r.Context().Value(auth.SessionContextKey).(*store.SessionRecord)
	if sr == nil || !hasRole(sr.Roles, "superadmin") {
		http.Error(w, localized(lang, "auth.2fa.superadminOnly"), http.StatusForbidden)
		return
	}
	raw := pathParams(r)["id"]
	id, _ := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if id <= 0 {
		http.Error(w, localized(lang, "common.badRequest"), http.StatusBadRequest)
		return
	}
	user, _, err := h.users.Get(r.Context(), id)
	if err != nil || user == nil {
		http.Error(w, localized(lang, "common.notFound"), http.StatusNotFound)
		return
	}
	if err := h.users.ClearTOTP(r.Context(), user.ID); err != nil {
		http.Error(w, localized(lang, "common.serverError"), http.StatusInternalServerError)
		return
	}
	if h.twoFA != nil {
		_ = h.twoFA.DeleteRecoveryCodes(r.Context(), user.ID)
		_ = h.twoFA.DeleteChallengesForUser(r.Context(), user.ID)
		_ = h.twoFA.DeleteTOTPSetup(r.Context(), user.ID)
	}
	_ = h.audits.Log(r.Context(), user.Username, "auth.2fa.disable", "admin_reset by="+sr.Username)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
