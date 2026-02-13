package api

import (
	"net/http"

	backupapi "berkut-scc/api/backups"
	"github.com/go-chi/chi/v5"
)

func (s *Server) registerBackupsRoutes(apiRouter chi.Router) {
	handler := backupapi.NewHandler(s.backupsSvc, s.audits)
	backupsRouter := backupapi.RegisterRoutes(backupapi.RouteDeps{
		WithSession:       s.withSession,
		RequirePermission: s.requirePermission,
		Handler:           handler,
	})
	apiRouter.Handle("/backups", http.StripPrefix("/api", backupsRouter))
	apiRouter.Handle("/backups/*", http.StripPrefix("/api", backupsRouter))
}
