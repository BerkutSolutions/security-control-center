package api

import (
	"net/http"

	"berkut-scc/api/handlers"
	"github.com/go-chi/chi/v5"
)

func (s *Server) registerRoutes() {
	s.router.Use(s.recoverMiddleware)
	s.router.Use(s.loggingMiddleware)
	s.router.Use(s.securityHeadersMiddleware)

	staticHandler := s.staticHandler()
	s.router.Handle("/static/*", http.StripPrefix("/static/", staticHandler))

	h := s.newRouteHandlers()
	appShell := s.withSession(s.requirePermission("app.view")(handlers.ServeStatic("app.html")))

	s.registerShellRoutes(appShell, h)

	apiRouter := chi.NewRouter()
	apiRouter.Use(s.jsonMiddleware)
	apiRouter.Use(s.maintenanceModeMiddleware)

	s.registerCoreAPIRoutes(apiRouter, h)
	s.registerAccountsRoutes(apiRouter, h)
	s.registerDocsRoutes(apiRouter, h)
	s.registerReportsRoutes(apiRouter, h)
	s.registerIncidentsRoutes(apiRouter, h)
	s.registerControlsRoutes(apiRouter, h)
	s.registerFindingsRoutes(apiRouter, h)
	s.registerAssetsRoutes(apiRouter, h)
	s.registerSoftwareRoutes(apiRouter, h)
	s.registerMonitoringRoutes(apiRouter, h)
	s.registerBackupsRoutes(apiRouter)
	s.registerTasksRoutes(apiRouter)
	s.registerTemplatesAndApprovalsRoutes(apiRouter, h)
	s.registerLogsAndSettingsRoutes(apiRouter, h)
	s.registerPageRoutes(h)
	s.router.Mount("/api", apiRouter)
}
