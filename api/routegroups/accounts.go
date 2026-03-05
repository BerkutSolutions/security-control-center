package routegroups

import (
	"berkut-scc/api/handlers"
	"github.com/go-chi/chi/v5"
)

func RegisterAccounts(apiRouter chi.Router, g Guards, h *handlers.AccountsHandler) {
	apiRouter.Route("/accounts", func(accounts chi.Router) {
		accounts.MethodFunc("GET", "/dashboard", g.SessionPerm("accounts.view_dashboard", h.Dashboard))
		accounts.MethodFunc("GET", "/users", g.SessionPerm("accounts.view", h.ListUsers))
		accounts.MethodFunc("POST", "/users", g.SessionPermStepup("accounts.manage", 900, h.CreateUser))
		accounts.MethodFunc("POST", "/users/bulk", g.SessionPermStepup("accounts.manage", 900, h.BulkUsers))
		accounts.MethodFunc("PUT", "/users/{id}", g.SessionPermStepup("accounts.manage", 900, h.UpdateUser))
		accounts.MethodFunc("DELETE", "/users/{id}", g.SessionPermStepup("accounts.manage", 900, h.Delete))
		accounts.MethodFunc("POST", "/users/{id}/reset-password", g.SessionPermStepup("accounts.manage", 900, h.ResetPassword))
		accounts.MethodFunc("POST", "/users/{id}/reset-2fa", g.SessionPermStepup("accounts.manage", 900, h.ResetUser2FA))
		accounts.MethodFunc("POST", "/users/{id}/lock", g.SessionPermStepup("accounts.manage", 900, h.LockUser))
		accounts.MethodFunc("POST", "/users/{id}/unlock", g.SessionPermStepup("accounts.manage", 900, h.UnlockUser))
		accounts.MethodFunc("GET", "/users/{id}/sessions", g.SessionAnyPerm([]string{"accounts.manage", "sessions.manage"}, h.ListSessions))
		accounts.MethodFunc("POST", "/users/{id}/sessions/kill_all", g.SessionAnyPerm([]string{"accounts.manage", "sessions.manage"}, h.KillAllSessions))
		accounts.MethodFunc("POST", "/sessions/{session_id}/kill", g.SessionAnyPerm([]string{"accounts.manage", "sessions.manage"}, h.KillSession))
		accounts.MethodFunc("GET", "/groups", g.SessionPerm("groups.view", h.ListGroups))
		accounts.MethodFunc("POST", "/groups", g.SessionPermStepup("groups.manage", 900, h.CreateGroup))
		accounts.MethodFunc("GET", "/groups/{id}", g.SessionPerm("groups.view", h.GetGroup))
		accounts.MethodFunc("PUT", "/groups/{id}", g.SessionPermStepup("groups.manage", 900, h.UpdateGroup))
		accounts.MethodFunc("DELETE", "/groups/{id}", g.SessionPermStepup("groups.manage", 900, h.DeleteGroup))
		accounts.MethodFunc("POST", "/groups/{id}/members", g.SessionPermStepup("groups.manage", 900, h.AddGroupMember))
		accounts.MethodFunc("DELETE", "/groups/{id}/members/{user_id}", g.SessionPermStepup("groups.manage", 900, h.RemoveGroupMember))
		accounts.MethodFunc("GET", "/users/{id}/groups", g.SessionPerm("groups.manage", h.ListUserGroups))
		accounts.MethodFunc("GET", "/roles", g.SessionPerm("roles.view", h.ListRoles))
		accounts.MethodFunc("POST", "/roles", g.SessionPermStepup("roles.manage", 900, h.CreateRole))
		accounts.MethodFunc("PUT", "/roles/{id}", g.SessionPermStepup("roles.manage", 900, h.UpdateRole))
		accounts.MethodFunc("DELETE", "/roles/{id}", g.SessionPermStepup("roles.manage", 900, h.DeleteRole))
		accounts.MethodFunc("GET", "/role-templates", g.SessionPerm("roles.view", h.ListRoleTemplates))
		accounts.MethodFunc("POST", "/roles/from-template", g.SessionPermStepup("roles.manage", 900, h.CreateRoleFromTemplate))
		accounts.MethodFunc("POST", "/import/upload", g.SessionPerm("accounts.manage", h.ImportUpload))
		accounts.MethodFunc("POST", "/import/commit", g.SessionPermStepup("accounts.manage", 900, h.ImportCommit))
		accounts.MethodFunc("POST", "/import", g.SessionPermStepup("accounts.manage", 900, h.ImportUsers))
	})
}
