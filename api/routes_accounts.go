package api

import (
	"net/http"

	"berkut-scc/api/routegroups"
	"berkut-scc/core/rbac"
	"github.com/go-chi/chi/v5"
)

func (s *Server) registerAccountsRoutes(apiRouter chi.Router, h routeHandlers) {
	routegroups.RegisterAccounts(apiRouter, routegroups.Guards{
		WithSession:       s.withSession,
		RequirePermission: func(p string) func(http.HandlerFunc) http.HandlerFunc { return s.requirePermission(rbac.Permission(p)) },
		RequireAnyPermission: func(perms ...string) func(http.HandlerFunc) http.HandlerFunc {
			return s.requireAnyPermission(toPermissions(perms)...)
		},
	}, h.accounts)
}

func toPermissions(in []string) []rbac.Permission {
	out := make([]rbac.Permission, 0, len(in))
	for _, p := range in {
		out = append(out, rbac.Permission(p))
	}
	return out
}
