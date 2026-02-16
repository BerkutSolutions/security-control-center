package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"berkut-scc/core/auth"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
)

func parseID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("bad id")
	}
	return id, nil
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var out []string
	for _, part := range parts {
		val := strings.TrimSpace(part)
		if val == "" {
			continue
		}
		out = append(out, val)
	}
	return out
}

func sessionUserID(r *http.Request) int64 {
	if sr := r.Context().Value(auth.SessionContextKey); sr != nil {
		if sess, ok := sr.(*store.SessionRecord); ok {
			return sess.UserID
		}
	}
	return 0
}

func currentUsername(r *http.Request) string {
	if sr := r.Context().Value(auth.SessionContextKey); sr != nil {
		if sess, ok := sr.(*store.SessionRecord); ok {
			return sess.Username
		}
	}
	return ""
}

func hasPermission(r *http.Request, policy *rbac.Policy, perm rbac.Permission) bool {
	if policy == nil {
		return false
	}
	if sr := r.Context().Value(auth.SessionContextKey); sr != nil {
		if sess, ok := sr.(*store.SessionRecord); ok {
			return policy.Allowed(sess.Roles, perm)
		}
	}
	return false
}

func initialStatus(paused bool) string {
	if paused {
		return "paused"
	}
	return "down"
}
