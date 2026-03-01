package routegroups

import (
	"berkut-scc/api/handlers"
	"github.com/go-chi/chi/v5"
)

func RegisterAccounts(apiRouter chi.Router, g Guards, h *handlers.AccountsHandler) {
	apiRouter.Route("/accounts", func(accounts chi.Router) {
		accounts.MethodFunc("GET", "/dashboard", g.SessionPerm("accounts.view_dashboard", h.Dashboard))
		accounts.MethodFunc("GET", "/users", g.SessionPerm("accounts.view", h.ListUsers))
		accounts.MethodFunc("POST", "/users", g.SessionPerm("accounts.manage", h.CreateUser))
		accounts.MethodFunc("POST", "/users/bulk", g.SessionPerm("accounts.manage", h.BulkUsers))
		accounts.MethodFunc("PUT", "/users/{id}", g.SessionPerm("accounts.manage", h.UpdateUser))
		accounts.MethodFunc("POST", "/users/{id}/reset-password", g.SessionPerm("accounts.manage", h.ResetPassword))
		accounts.MethodFunc("POST", "/users/{id}/reset-2fa", g.SessionPerm("accounts.manage", h.ResetUser2FA))
		accounts.MethodFunc("POST", "/users/{id}/lock", g.SessionPerm("accounts.manage", h.LockUser))
		accounts.MethodFunc("POST", "/users/{id}/unlock", g.SessionPerm("accounts.manage", h.UnlockUser))
		accounts.MethodFunc("GET", "/users/{id}/sessions", g.SessionAnyPerm([]string{"accounts.manage", "sessions.manage"}, h.ListSessions))
		accounts.MethodFunc("POST", "/users/{id}/sessions/kill_all", g.SessionAnyPerm([]string{"accounts.manage", "sessions.manage"}, h.KillAllSessions))
		accounts.MethodFunc("POST", "/sessions/{session_id}/kill", g.SessionAnyPerm([]string{"accounts.manage", "sessions.manage"}, h.KillSession))
		accounts.MethodFunc("GET", "/groups", g.SessionPerm("groups.view", h.ListGroups))
		accounts.MethodFunc("POST", "/groups", g.SessionPerm("groups.manage", h.CreateGroup))
		accounts.MethodFunc("GET", "/groups/{id}", g.SessionPerm("groups.view", h.GetGroup))
		accounts.MethodFunc("PUT", "/groups/{id}", g.SessionPerm("groups.manage", h.UpdateGroup))
		accounts.MethodFunc("DELETE", "/groups/{id}", g.SessionPerm("groups.manage", h.DeleteGroup))
		accounts.MethodFunc("POST", "/groups/{id}/members", g.SessionPerm("groups.manage", h.AddGroupMember))
		accounts.MethodFunc("DELETE", "/groups/{id}/members/{user_id}", g.SessionPerm("groups.manage", h.RemoveGroupMember))
		accounts.MethodFunc("GET", "/users/{id}/groups", g.SessionPerm("groups.manage", h.ListUserGroups))
		accounts.MethodFunc("GET", "/roles", g.SessionPerm("roles.view", h.ListRoles))
		accounts.MethodFunc("POST", "/roles", g.SessionPerm("roles.manage", h.CreateRole))
		accounts.MethodFunc("PUT", "/roles/{id}", g.SessionPerm("roles.manage", h.UpdateRole))
		accounts.MethodFunc("DELETE", "/roles/{id}", g.SessionPerm("roles.manage", h.DeleteRole))
		accounts.MethodFunc("GET", "/role-templates", g.SessionPerm("roles.view", h.ListRoleTemplates))
		accounts.MethodFunc("POST", "/roles/from-template", g.SessionPerm("roles.manage", h.CreateRoleFromTemplate))
		accounts.MethodFunc("POST", "/import/upload", g.SessionPerm("accounts.manage", h.ImportUpload))
		accounts.MethodFunc("POST", "/import/commit", g.SessionPerm("accounts.manage", h.ImportCommit))
		accounts.MethodFunc("POST", "/import", g.SessionPerm("accounts.manage", h.ImportUsers))
	})
}
