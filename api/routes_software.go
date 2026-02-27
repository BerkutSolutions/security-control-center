package api

import (
	"net/http"

	"berkut-scc/api/routegroups"
	"berkut-scc/core/rbac"
	"github.com/go-chi/chi/v5"
)

func (s *Server) registerSoftwareRoutes(apiRouter chi.Router, h routeHandlers) {
	routegroups.RegisterSoftware(apiRouter, routegroups.Guards{
		WithSession:       s.withSession,
		RequirePermission: func(p string) func(http.HandlerFunc) http.HandlerFunc { return s.requirePermission(rbac.Permission(p)) },
	}, h.software)
}
