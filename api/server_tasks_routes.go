package api

import (
	"net/http"
	"strconv"

	"berkut-scc/api/handlers"
	"github.com/go-chi/chi/v5"
)

func (s *Server) redirectLegacyTaskLink(w http.ResponseWriter, r *http.Request) {
	taskID := parsePathInt64(chi.URLParam(r, "task_id"))
	if taskID <= 0 {
		http.NotFound(w, r)
		return
	}
	task, err := s.tasksStore.GetTask(r.Context(), taskID)
	if err != nil || task == nil {
		http.NotFound(w, r)
		return
	}
	board, err := s.tasksStore.GetBoard(r.Context(), task.BoardID)
	if err != nil || board == nil || board.SpaceID <= 0 {
		http.Redirect(w, r, "/tasks", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/tasks/space/"+strconv.FormatInt(board.SpaceID, 10)+"/task/"+strconv.FormatInt(taskID, 10), http.StatusMovedPermanently)
}

func (s *Server) taskSpaceTaskAppShell(w http.ResponseWriter, r *http.Request) {
	spaceID := parsePathInt64(chi.URLParam(r, "space_id"))
	taskID := parsePathInt64(chi.URLParam(r, "task_id"))
	if spaceID <= 0 || taskID <= 0 {
		http.NotFound(w, r)
		return
	}
	task, err := s.tasksStore.GetTask(r.Context(), taskID)
	if err != nil || task == nil {
		http.NotFound(w, r)
		return
	}
	board, err := s.tasksStore.GetBoard(r.Context(), task.BoardID)
	if err != nil || board == nil || board.SpaceID != spaceID {
		http.NotFound(w, r)
		return
	}
	handlers.ServeStatic("app.html").ServeHTTP(w, r)
}

func parsePathInt64(raw string) int64 {
	val, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return val
}
