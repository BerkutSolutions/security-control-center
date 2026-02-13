package taskshttp

import (
	"net/http"

	"berkut-scc/core/rbac"
	"berkut-scc/tasks"
	"github.com/go-chi/chi/v5"
)

type RouteDeps struct {
	WithSession       func(http.HandlerFunc) http.HandlerFunc
	RequirePermission func(rbac.Permission) func(http.HandlerFunc) http.HandlerFunc
	Handler           *Handler
}

func RegisterRoutes(deps RouteDeps) http.Handler {
	r := chi.NewRouter()
	h := deps.Handler
	withSession := deps.WithSession
	require := deps.RequirePermission

	r.Get("/tasks/spaces", withSession(require(tasks.PermView)(h.ListSpaces)))
	r.Get("/tasks/spaces/summary", withSession(require(tasks.PermView)(h.ListSpaceSummary)))
	r.Post("/tasks/spaces", withSession(require(tasks.PermManage)(h.CreateSpace)))
	r.Put("/tasks/spaces/{id}", withSession(require(tasks.PermManage)(h.UpdateSpace)))
	r.Delete("/tasks/spaces/{id}", withSession(require(tasks.PermManage)(h.DeleteSpace)))
	r.Get("/tasks/spaces/{id}/layout", withSession(require(tasks.PermView)(h.GetBoardLayout)))
	r.Post("/tasks/spaces/{id}/layout", withSession(require(tasks.PermView)(h.SaveBoardLayout)))

	r.Get("/tasks/boards", withSession(require(tasks.PermView)(h.ListBoards)))
	r.Post("/tasks/boards", withSession(require(tasks.PermManage)(h.CreateBoard)))
	r.Put("/tasks/boards/{id}", withSession(require(tasks.PermManage)(h.UpdateBoard)))
	r.Delete("/tasks/boards/{id}", withSession(require(tasks.PermManage)(h.DeleteBoard)))
	r.Post("/tasks/boards/{id}/move", withSession(require(tasks.PermManage)(h.MoveBoard)))
	r.Get("/tasks/boards/{board_id}/columns", withSession(require(tasks.PermView)(h.ListColumns)))
	r.Post("/tasks/boards/{board_id}/columns", withSession(require(tasks.PermManage)(h.CreateColumn)))
	r.Get("/tasks/boards/{board_id}/subcolumns", withSession(require(tasks.PermView)(h.ListSubColumnsByBoard)))
	r.Put("/tasks/columns/{id}", withSession(require(tasks.PermManage)(h.UpdateColumn)))
	r.Delete("/tasks/columns/{id}", withSession(require(tasks.PermManage)(h.DeleteColumn)))
	r.Post("/tasks/columns/{id}/move", withSession(require(tasks.PermManage)(h.MoveColumn)))
	r.Post("/tasks/columns/{id}/archive", withSession(require(tasks.PermArchive)(h.ArchiveColumnTasks)))
	r.Get("/tasks/columns/{column_id}/subcolumns", withSession(require(tasks.PermView)(h.ListSubColumns)))
	r.Post("/tasks/columns/{column_id}/subcolumns", withSession(require(tasks.PermManage)(h.CreateSubColumn)))
	r.Put("/tasks/subcolumns/{id}", withSession(require(tasks.PermManage)(h.UpdateSubColumn)))
	r.Delete("/tasks/subcolumns/{id}", withSession(require(tasks.PermManage)(h.DeleteSubColumn)))
	r.Post("/tasks/subcolumns/{id}/move", withSession(require(tasks.PermManage)(h.MoveSubColumn)))
	r.Get("/tasks/tags", withSession(require(tasks.PermView)(h.ListTags)))
	r.Get("/tasks", withSession(require(tasks.PermView)(h.ListTasks)))
	r.Post("/tasks", withSession(require(tasks.PermCreate)(h.CreateTask)))
	r.Get("/tasks/templates", withSession(require(tasks.PermTemplatesView)(h.ListTemplates)))
	r.Post("/tasks/templates", withSession(require(tasks.PermTemplatesManage)(h.CreateTemplate)))
	r.Put("/tasks/templates/{id}", withSession(require(tasks.PermTemplatesManage)(h.UpdateTemplate)))
	r.Delete("/tasks/templates/{id}", withSession(require(tasks.PermTemplatesManage)(h.DeleteTemplate)))
	r.Post("/tasks/templates/{id}/create-task", withSession(require(tasks.PermCreate)(h.CreateTaskFromTemplate)))
	r.Get("/tasks/recurring", withSession(require(tasks.PermRecurringView)(h.ListRecurringRules)))
	r.Post("/tasks/recurring", withSession(require(tasks.PermRecurringManage)(h.CreateRecurringRule)))
	r.Put("/tasks/recurring/{id}", withSession(require(tasks.PermRecurringManage)(h.UpdateRecurringRule)))
	r.Post("/tasks/recurring/{id}/toggle", withSession(require(tasks.PermRecurringManage)(h.ToggleRecurringRule)))
	r.Post("/tasks/recurring/{id}/run-now", withSession(require(tasks.PermRecurringRun)(h.RunRecurringRuleNow)))
	r.Get("/tasks/archive", withSession(require(tasks.PermArchive)(h.ListArchivedTasks)))
	r.Post("/tasks/archive/{id}/restore", withSession(require(tasks.PermArchive)(h.RestoreTask)))
	r.Get("/tasks/{id}", withSession(require(tasks.PermView)(h.GetTask)))
	r.Put("/tasks/{id}", withSession(require(tasks.PermEdit)(h.UpdateTask)))
	r.Delete("/tasks/{id}", withSession(require(tasks.PermArchive)(h.DeleteTask)))
	r.Post("/tasks/{id}/move", withSession(require(tasks.PermMove)(h.MoveTask)))
	r.Post("/tasks/{id}/relocate", withSession(require(tasks.PermMove)(h.RelocateTask)))
	r.Post("/tasks/{id}/clone", withSession(require(tasks.PermCreate)(h.CloneTask)))
	r.Post("/tasks/{id}/close", withSession(require(tasks.PermClose)(h.CloseTask)))
	r.Post("/tasks/{id}/archive", withSession(require(tasks.PermArchive)(h.ArchiveTask)))
	r.Get("/tasks/{id}/comments", withSession(require(tasks.PermView)(h.ListComments)))
	r.Post("/tasks/{id}/comments", withSession(require(tasks.PermComment)(h.AddComment)))
	r.Put("/tasks/{id}/comments/{comment_id}", withSession(require(tasks.PermComment)(h.UpdateComment)))
	r.Delete("/tasks/{id}/comments/{comment_id}", withSession(require(tasks.PermComment)(h.DeleteComment)))
	r.Get("/tasks/{id}/comments/{comment_id}/files/{file_id}", withSession(require(tasks.PermView)(h.DownloadCommentFile)))
	r.Delete("/tasks/{id}/comments/{comment_id}/files/{file_id}", withSession(require(tasks.PermComment)(h.DeleteCommentFile)))
	r.Get("/tasks/{id}/files", withSession(require(tasks.PermView)(h.ListFiles)))
	r.Post("/tasks/{id}/files", withSession(require(tasks.PermEdit)(h.AddFile)))
	r.Get("/tasks/{id}/files/{file_id}", withSession(require(tasks.PermView)(h.DownloadFile)))
	r.Delete("/tasks/{id}/files/{file_id}", withSession(require(tasks.PermEdit)(h.DeleteFile)))
	r.Get("/tasks/{id}/links", withSession(require(tasks.PermView)(h.ListLinks)))
	r.Post("/tasks/{id}/links", withSession(require(tasks.PermEdit)(h.AddLink)))
	r.Delete("/tasks/{id}/links/{link_id}", withSession(require(tasks.PermEdit)(h.DeleteLink)))
	r.Get("/tasks/{id}/control-links", withSession(require(tasks.PermView)(h.ListControlLinks)))
	r.Get("/tasks/{id}/blocks", withSession(require(tasks.PermView)(h.ListBlocks)))
	r.Post("/tasks/{id}/blocks/text", withSession(require(tasks.PermBlockCreate)(h.AddTextBlock)))
	r.Post("/tasks/{id}/blocks/task", withSession(require(tasks.PermBlockCreate)(h.AddTaskBlock)))
	r.Post("/tasks/{id}/blocks/{block_id}/resolve", withSession(require(tasks.PermBlockResolve)(h.ResolveBlock)))
	return r
}
