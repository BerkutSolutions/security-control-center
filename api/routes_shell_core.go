package api

import (
	"net/http"

	"berkut-scc/api/handlers"
	"github.com/go-chi/chi/v5"
)

func (s *Server) registerShellRoutes(appShell http.HandlerFunc, h routeHandlers) {
	s.router.MethodFunc("GET", "/login", handlers.ServeStatic("login.html"))
	s.router.MethodFunc("GET", "/login/2fa", handlers.ServeStatic("login_2fa.html"))
	s.router.MethodFunc("GET", "/password-change", s.withSession(h.auth.PasswordChangePage))
	s.router.MethodFunc("GET", "/healthcheck", s.withSession(h.auth.HealthcheckPage))
	s.router.HandleFunc("/", s.redirectToEntry)
	s.router.MethodFunc("GET", "/app", handlers.ServeStatic("app.html"))
	s.registerShellTabRoutes(appShell)
	s.router.HandleFunc("/index.html", s.redirectToEntry)
	s.router.MethodFunc("GET", "/favicon.ico", handlers.ServeStatic("favicon.ico"))
}

func (s *Server) registerCoreAPIRoutes(apiRouter chi.Router, h routeHandlers) {
	apiRouter.MethodFunc("POST", "/auth/login", s.rateLimitMiddleware(h.auth.Login))
	apiRouter.MethodFunc("POST", "/auth/login/2fa", s.rateLimit2FAMiddleware(h.auth.Login2FA))
	apiRouter.MethodFunc("POST", "/auth/login/2fa/passkey/begin", s.rateLimit2FAMiddleware(h.auth.Login2FAPasskeyBegin))
	apiRouter.MethodFunc("POST", "/auth/login/2fa/passkey/finish", s.rateLimit2FAMiddleware(h.auth.Login2FAPasskeyFinish))
	apiRouter.MethodFunc("POST", "/auth/passkeys/login/begin", s.rateLimitMiddleware(h.auth.PasskeyLoginBegin))
	apiRouter.MethodFunc("POST", "/auth/passkeys/login/finish", s.rateLimitMiddleware(h.auth.PasskeyLoginFinish))
	apiRouter.MethodFunc("POST", "/auth/logout", s.withSession(h.auth.Logout))
	apiRouter.MethodFunc("GET", "/auth/me", s.withSession(h.auth.Me))
	apiRouter.MethodFunc("GET", "/auth/2fa/status", s.withSession(s.requirePermission("app.view")(h.auth.TwoFAStatus)))
	apiRouter.MethodFunc("POST", "/auth/2fa/setup", s.withSession(s.requirePermission("app.view")(h.auth.TwoFASetup)))
	apiRouter.MethodFunc("POST", "/auth/2fa/enable", s.withSession(s.requirePermission("app.view")(h.auth.TwoFAEnable)))
	apiRouter.MethodFunc("POST", "/auth/2fa/disable", s.withSession(s.requirePermission("app.view")(h.auth.TwoFADisable)))
	apiRouter.MethodFunc("GET", "/auth/passkeys", s.withSession(s.requirePermission("app.view")(h.auth.PasskeysList)))
	apiRouter.MethodFunc("POST", "/auth/passkeys/register/begin", s.withSession(s.requirePermission("app.view")(h.auth.PasskeyRegisterBegin)))
	apiRouter.MethodFunc("POST", "/auth/passkeys/register/finish", s.withSession(s.requirePermission("app.view")(h.auth.PasskeyRegisterFinish)))
	apiRouter.MethodFunc("PUT", "/auth/passkeys/{id}/rename", s.withSession(s.requirePermission("app.view")(h.auth.PasskeyRename)))
	apiRouter.MethodFunc("DELETE", "/auth/passkeys/{id}", s.withSession(s.requirePermission("app.view")(h.auth.PasskeyDelete)))
	apiRouter.MethodFunc("POST", "/auth/change-password", s.withSession(h.auth.ChangePassword))
	apiRouter.MethodFunc("GET", "/app/menu", s.withSession(h.auth.Menu))
	apiRouter.MethodFunc("POST", "/app/ping", s.withSession(h.auth.Ping))
	apiRouter.MethodFunc("POST", "/app/view", s.withSession(s.appView))
	apiRouter.MethodFunc("GET", "/app/meta", s.withSession(s.requirePermission("app.view")(h.runtime.Meta)))
	apiRouter.MethodFunc("GET", "/app/preflight", s.withSession(s.requirePermission("app.preflight.view")(h.preflight.Report)))
	apiRouter.MethodFunc("GET", "/app/compat", s.withSession(s.requirePermission("app.compat.view")(h.compat.Report)))
	apiRouter.MethodFunc("POST", "/app/jobs", s.withSession(s.requirePermission("app.compat.manage.partial")(h.jobs.Create)))
	apiRouter.MethodFunc("GET", "/app/jobs", s.withSession(s.requirePermission("app.compat.view")(h.jobs.List)))
	apiRouter.MethodFunc("GET", "/app/jobs/{id}", s.withSession(s.requirePermission("app.compat.view")(h.jobs.Get)))
	apiRouter.MethodFunc("POST", "/app/jobs/{id}/cancel", s.withSession(s.requirePermission("app.compat.manage.partial")(h.jobs.Cancel)))
	apiRouter.MethodFunc("GET", "/dashboard", s.withSession(s.requirePermission("dashboard.view")(h.dashboard.Data)))
	apiRouter.MethodFunc("POST", "/dashboard/layout", s.withSession(s.requirePermission("dashboard.view")(h.dashboard.SaveLayout)))
}

func (s *Server) registerPageRoutes(h routeHandlers) {
	s.router.Route("/api/page", func(pageRouter chi.Router) {
		pageRouter.MethodFunc("GET", "/dashboard", s.withSession(s.requirePermission("dashboard.view")(h.dashboard.Dashboard)))
		pageRouter.MethodFunc("GET", "/settings", s.withSession(s.requirePermission("app.view")(h.settings.Page)))
		pageRouter.MethodFunc("GET", "/accounts", s.withSession(s.requirePermission("accounts.view")(h.accounts.Page)))
		pageRouter.MethodFunc("GET", "/{name}", s.withSession(s.requirePermissionFromPath(handlers.RequiredPermission)(h.placeholder.Page)))
	})
}
