package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"berkut-scc/core/auth"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
)

type appViewPayload struct {
	Page string `json:"page"`
	Tab  string `json:"tab"`
	Path string `json:"path"`
}

func (s *Server) appView(w http.ResponseWriter, r *http.Request) {
	var payload appViewPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	page := strings.ToLower(strings.TrimSpace(payload.Page))
	tab := strings.ToLower(strings.TrimSpace(payload.Tab))
	path := strings.TrimSpace(payload.Path)
	if page == "" || len(page) > 64 || len(tab) > 64 || len(path) > 256 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	user := "-"
	var session *store.SessionRecord
	if v := r.Context().Value(auth.SessionContextKey); v != nil {
		if sr, ok := v.(*store.SessionRecord); ok && sr != nil && sr.Username != "" {
			user = sr.Username
			session = sr
		}
	}

	if !s.isViewAllowed(session, page, tab) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	action := s.viewAction(page, tab)
	if action == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if s.audits != nil && user != "-" {
		_ = s.audits.Log(r.Context(), user, action, "")
	}
	if s.logger != nil {
		s.logger.Printf("VIEW page=%s tab=%s path=%s user=%s ts=%s", page, tab, path, user, time.Now().UTC().Format(time.RFC3339))
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) isViewAllowed(sr *store.SessionRecord, page, tab string) bool {
	if sr == nil || s.policy == nil {
		return false
	}
	roles := sr.Roles
	switch page {
	case "registry":
		if !s.policy.Allowed(roles, rbac.Permission("controls.view")) {
			return false
		}
		switch tab {
		case "assets":
			return s.policy.Allowed(roles, rbac.Permission("assets.view"))
		case "software":
			return s.policy.Allowed(roles, rbac.Permission("software.view"))
		case "findings":
			return s.policy.Allowed(roles, rbac.Permission("findings.view"))
		default:
			return true
		}
	default:
		return false
	}
}

func (s *Server) viewAction(page, tab string) string {
	switch page {
	case "registry":
		switch tab {
		case "overview", "controls", "checks", "violations", "frameworks", "assets", "software", "findings":
			return "registry.tab." + tab + ".view"
		default:
			return ""
		}
	default:
		return ""
	}
}
