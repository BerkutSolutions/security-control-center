package taskshttp

import (
	"net/http"

	"berkut-scc/core/rbac"
	"berkut-scc/tasks"
	"github.com/gorilla/mux"
)

type RouteDeps struct {
	Router            *mux.Router
	WithSession       func(http.HandlerFunc) http.HandlerFunc
	RequirePermission func(rbac.Permission) func(http.HandlerFunc) http.HandlerFunc
	Handler           *Handler
}

func RegisterRoutes(deps RouteDeps) {
	r := deps.Router.PathPrefix("/tasks").Subrouter()
	h := deps.Handler
	withSession := deps.WithSession
	require := deps.RequirePermission

	r.HandleFunc("/spaces", withSession(require(tasks.PermView)(h.ListSpaces))).Methods("GET")
	r.HandleFunc("/spaces/summary", withSession(require(tasks.PermView)(h.ListSpaceSummary))).Methods("GET")
	r.HandleFunc("/spaces", withSession(require(tasks.PermManage)(h.CreateSpace))).Methods("POST")
	r.HandleFunc("/spaces/{id}", withSession(require(tasks.PermManage)(h.UpdateSpace))).Methods("PUT")
	r.HandleFunc("/spaces/{id}", withSession(require(tasks.PermManage)(h.DeleteSpace))).Methods("DELETE")
	r.HandleFunc("/spaces/{id}/layout", withSession(require(tasks.PermView)(h.GetBoardLayout))).Methods("GET")
	r.HandleFunc("/spaces/{id}/layout", withSession(require(tasks.PermView)(h.SaveBoardLayout))).Methods("POST")

	r.HandleFunc("/boards", withSession(require(tasks.PermView)(h.ListBoards))).Methods("GET")
	r.HandleFunc("/boards", withSession(require(tasks.PermManage)(h.CreateBoard))).Methods("POST")
	r.HandleFunc("/boards/{id}", withSession(require(tasks.PermManage)(h.UpdateBoard))).Methods("PUT")
	r.HandleFunc("/boards/{id}", withSession(require(tasks.PermManage)(h.DeleteBoard))).Methods("DELETE")
	r.HandleFunc("/boards/{id}/move", withSession(require(tasks.PermManage)(h.MoveBoard))).Methods("POST")
	r.HandleFunc("/boards/{board_id}/columns", withSession(require(tasks.PermView)(h.ListColumns))).Methods("GET")
	r.HandleFunc("/boards/{board_id}/columns", withSession(require(tasks.PermManage)(h.CreateColumn))).Methods("POST")
	r.HandleFunc("/boards/{board_id}/subcolumns", withSession(require(tasks.PermView)(h.ListSubColumnsByBoard))).Methods("GET")
	r.HandleFunc("/columns/{id}", withSession(require(tasks.PermManage)(h.UpdateColumn))).Methods("PUT")
	r.HandleFunc("/columns/{id}", withSession(require(tasks.PermManage)(h.DeleteColumn))).Methods("DELETE")
	r.HandleFunc("/columns/{id}/move", withSession(require(tasks.PermManage)(h.MoveColumn))).Methods("POST")
	r.HandleFunc("/columns/{id}/archive", withSession(require(tasks.PermArchive)(h.ArchiveColumnTasks))).Methods("POST")
	r.HandleFunc("/columns/{column_id}/subcolumns", withSession(require(tasks.PermView)(h.ListSubColumns))).Methods("GET")
	r.HandleFunc("/columns/{column_id}/subcolumns", withSession(require(tasks.PermManage)(h.CreateSubColumn))).Methods("POST")
	r.HandleFunc("/subcolumns/{id}", withSession(require(tasks.PermManage)(h.UpdateSubColumn))).Methods("PUT")
	r.HandleFunc("/subcolumns/{id}", withSession(require(tasks.PermManage)(h.DeleteSubColumn))).Methods("DELETE")
	r.HandleFunc("/subcolumns/{id}/move", withSession(require(tasks.PermManage)(h.MoveSubColumn))).Methods("POST")
	r.HandleFunc("/tags", withSession(require(tasks.PermView)(h.ListTags))).Methods("GET")
	r.HandleFunc("", withSession(require(tasks.PermView)(h.ListTasks))).Methods("GET")
	r.HandleFunc("", withSession(require(tasks.PermCreate)(h.CreateTask))).Methods("POST")
	r.HandleFunc("/templates", withSession(require(tasks.PermTemplatesView)(h.ListTemplates))).Methods("GET")
	r.HandleFunc("/templates", withSession(require(tasks.PermTemplatesManage)(h.CreateTemplate))).Methods("POST")
	r.HandleFunc("/templates/{id}", withSession(require(tasks.PermTemplatesManage)(h.UpdateTemplate))).Methods("PUT")
	r.HandleFunc("/templates/{id}", withSession(require(tasks.PermTemplatesManage)(h.DeleteTemplate))).Methods("DELETE")
	r.HandleFunc("/templates/{id}/create-task", withSession(require(tasks.PermCreate)(h.CreateTaskFromTemplate))).Methods("POST")
	r.HandleFunc("/recurring", withSession(require(tasks.PermRecurringView)(h.ListRecurringRules))).Methods("GET")
	r.HandleFunc("/recurring", withSession(require(tasks.PermRecurringManage)(h.CreateRecurringRule))).Methods("POST")
	r.HandleFunc("/recurring/{id}", withSession(require(tasks.PermRecurringManage)(h.UpdateRecurringRule))).Methods("PUT")
	r.HandleFunc("/recurring/{id}/toggle", withSession(require(tasks.PermRecurringManage)(h.ToggleRecurringRule))).Methods("POST")
	r.HandleFunc("/recurring/{id}/run-now", withSession(require(tasks.PermRecurringRun)(h.RunRecurringRuleNow))).Methods("POST")
	r.HandleFunc("/archive", withSession(require(tasks.PermArchive)(h.ListArchivedTasks))).Methods("GET")
	r.HandleFunc("/archive/{id}/restore", withSession(require(tasks.PermArchive)(h.RestoreTask))).Methods("POST")
	r.HandleFunc("/{id}", withSession(require(tasks.PermView)(h.GetTask))).Methods("GET")
	r.HandleFunc("/{id}", withSession(require(tasks.PermEdit)(h.UpdateTask))).Methods("PUT")
	r.HandleFunc("/{id}", withSession(require(tasks.PermArchive)(h.DeleteTask))).Methods("DELETE")
	r.HandleFunc("/{id}/move", withSession(require(tasks.PermMove)(h.MoveTask))).Methods("POST")
	r.HandleFunc("/{id}/relocate", withSession(require(tasks.PermMove)(h.RelocateTask))).Methods("POST")
	r.HandleFunc("/{id}/clone", withSession(require(tasks.PermCreate)(h.CloneTask))).Methods("POST")
	r.HandleFunc("/{id}/close", withSession(require(tasks.PermClose)(h.CloseTask))).Methods("POST")
	r.HandleFunc("/{id}/archive", withSession(require(tasks.PermArchive)(h.ArchiveTask))).Methods("POST")
	r.HandleFunc("/{id}/comments", withSession(require(tasks.PermView)(h.ListComments))).Methods("GET")
	r.HandleFunc("/{id}/comments", withSession(require(tasks.PermComment)(h.AddComment))).Methods("POST")
	r.HandleFunc("/{id}/comments/{comment_id}", withSession(require(tasks.PermComment)(h.UpdateComment))).Methods("PUT")
	r.HandleFunc("/{id}/comments/{comment_id}", withSession(require(tasks.PermComment)(h.DeleteComment))).Methods("DELETE")
	r.HandleFunc("/{id}/comments/{comment_id}/files/{file_id}", withSession(require(tasks.PermView)(h.DownloadCommentFile))).Methods("GET")
	r.HandleFunc("/{id}/comments/{comment_id}/files/{file_id}", withSession(require(tasks.PermComment)(h.DeleteCommentFile))).Methods("DELETE")
	r.HandleFunc("/{id}/files", withSession(require(tasks.PermView)(h.ListFiles))).Methods("GET")
	r.HandleFunc("/{id}/files", withSession(require(tasks.PermEdit)(h.AddFile))).Methods("POST")
	r.HandleFunc("/{id}/files/{file_id}", withSession(require(tasks.PermView)(h.DownloadFile))).Methods("GET")
	r.HandleFunc("/{id}/files/{file_id}", withSession(require(tasks.PermEdit)(h.DeleteFile))).Methods("DELETE")
	r.HandleFunc("/{id}/links", withSession(require(tasks.PermView)(h.ListLinks))).Methods("GET")
	r.HandleFunc("/{id}/links", withSession(require(tasks.PermEdit)(h.AddLink))).Methods("POST")
	r.HandleFunc("/{id}/links/{link_id}", withSession(require(tasks.PermEdit)(h.DeleteLink))).Methods("DELETE")
	r.HandleFunc("/{id}/control-links", withSession(require(tasks.PermView)(h.ListControlLinks))).Methods("GET")
	r.HandleFunc("/{id}/blocks", withSession(require(tasks.PermView)(h.ListBlocks))).Methods("GET")
	r.HandleFunc("/{id}/blocks/text", withSession(require(tasks.PermBlockCreate)(h.AddTextBlock))).Methods("POST")
	r.HandleFunc("/{id}/blocks/task", withSession(require(tasks.PermBlockCreate)(h.AddTaskBlock))).Methods("POST")
	r.HandleFunc("/{id}/blocks/{block_id}/resolve", withSession(require(tasks.PermBlockResolve)(h.ResolveBlock))).Methods("POST")
}
