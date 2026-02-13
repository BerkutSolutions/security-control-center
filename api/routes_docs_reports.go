package api

import (
	"net/http"

	"berkut-scc/api/routegroups"
	"berkut-scc/core/rbac"
	"github.com/go-chi/chi/v5"
)

func (s *Server) registerDocsRoutes(apiRouter chi.Router, h routeHandlers) {
	routegroups.RegisterDocs(apiRouter, routegroups.Guards{
		WithSession:       s.withSession,
		RequirePermission: func(p string) func(http.HandlerFunc) http.HandlerFunc { return s.requirePermission(rbac.Permission(p)) },
	}, h.docs, h.incidents)
}

func (s *Server) registerReportsRoutes(apiRouter chi.Router, h routeHandlers) {
	routegroups.RegisterReports(apiRouter, routegroups.Guards{
		WithSession:       s.withSession,
		RequirePermission: func(p string) func(http.HandlerFunc) http.HandlerFunc { return s.requirePermission(rbac.Permission(p)) },
	}, h.reports)
}
