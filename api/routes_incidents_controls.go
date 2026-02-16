package api

import (
	"net/http"

	"berkut-scc/api/routegroups"
	"berkut-scc/core/rbac"
	"github.com/go-chi/chi/v5"
)

func (s *Server) registerIncidentsRoutes(apiRouter chi.Router, h routeHandlers) {
	routegroups.RegisterIncidents(apiRouter, routegroups.Guards{
		WithSession:       s.withSession,
		RequirePermission: func(p string) func(http.HandlerFunc) http.HandlerFunc { return s.requirePermission(rbac.Permission(p)) },
	}, h.incidents)
}

func (s *Server) registerControlsRoutes(apiRouter chi.Router, h routeHandlers) {
	routegroups.RegisterControls(apiRouter, routegroups.Guards{
		WithSession:       s.withSession,
		RequirePermission: func(p string) func(http.HandlerFunc) http.HandlerFunc { return s.requirePermission(rbac.Permission(p)) },
	}, h.controls)
}
