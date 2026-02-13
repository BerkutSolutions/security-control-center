package api

import "net/http"

func (s *Server) registerShellTabRoutes(appShell http.HandlerFunc) {
	s.router.MethodFunc("GET", "/dashboard", appShell)
	s.router.MethodFunc("GET", "/tasks", appShell)
	s.router.MethodFunc("GET", "/tasks/task/{task_id}", s.withSession(s.requirePermission("app.view")(s.redirectLegacyTaskLink)))
	s.router.MethodFunc("GET", "/tasks/space/{space_id}/task/{task_id}", s.withSession(s.requirePermission("app.view")(s.taskSpaceTaskAppShell)))
	s.router.MethodFunc("GET", "/tasks/*", appShell)
	s.router.MethodFunc("GET", "/docs", appShell)
	s.router.MethodFunc("GET", "/docs/*", appShell)
	s.router.MethodFunc("GET", "/approvals", appShell)
	s.router.MethodFunc("GET", "/approvals/*", appShell)
	s.router.MethodFunc("GET", "/incidents", appShell)
	s.router.MethodFunc("GET", "/incidents/*", appShell)
	s.router.MethodFunc("GET", "/controls", appShell)
	s.router.MethodFunc("GET", "/monitoring", appShell)
	s.router.MethodFunc("GET", "/monitoring/*", appShell)
	s.router.MethodFunc("GET", "/reports", appShell)
	s.router.MethodFunc("GET", "/backups", appShell)
	s.router.MethodFunc("GET", "/backups/*", appShell)
	s.router.MethodFunc("GET", "/findings", appShell)
	s.router.MethodFunc("GET", "/accounts", appShell)
	s.router.MethodFunc("GET", "/accounts/*", appShell)
	s.router.MethodFunc("GET", "/settings", appShell)
	s.router.MethodFunc("GET", "/settings/*", appShell)
	s.router.MethodFunc("GET", "/logs", appShell)
}
