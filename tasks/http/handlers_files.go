package taskshttp

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"berkut-scc/core/utils"
	"berkut-scc/tasks"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) ListFiles(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	task, ok := h.getTaskWithBoardAccess(w, r, user, roles, groups, "view")
	if !ok {
		return
	}
	items, err := h.svc.Store().ListTaskFiles(r.Context(), task.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	for i := range items {
		prepareTaskFile(task.ID, &items[i])
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) AddFile(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	task, ok := h.getTaskWithBoardAccess(w, r, user, roles, groups, "manage")
	if !ok {
		return
	}
	if task.ClosedAt != nil || task.IsArchived {
		respondError(w, http.StatusConflict, "tasks.closedReadOnly")
		return
	}
	if err := parseMultipartFormLimited(w, r, 25<<20); err != nil {
		return
	}
	if r.MultipartForm == nil || len(r.MultipartForm.File["files"]) == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	files, err := saveTaskFiles(task.ID, r.MultipartForm.File["files"])
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	var created []tasks.TaskFile
	for i := range files {
		files[i].TaskID = task.ID
		files[i].UploadedBy = &user.ID
		if _, err := h.svc.Store().AddTaskFile(r.Context(), &files[i]); err != nil {
			respondError(w, http.StatusInternalServerError, "server error")
			return
		}
		tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskFileAdd, fmt.Sprintf("%d|%d", task.ID, files[i].ID))
		prepareTaskFile(task.ID, &files[i])
		created = append(created, files[i])
	}
	respondJSON(w, http.StatusCreated, map[string]any{"items": created})
}

func (h *Handler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	task, ok := h.getTaskWithBoardAccess(w, r, user, roles, groups, "view")
	if !ok {
		return
	}
	fileID := parseInt64Default(chi.URLParam(r, "file_id"), 0)
	if fileID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	file, err := h.svc.Store().GetTaskFile(r.Context(), task.ID, fileID)
	if err != nil || file == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	path := filepath.Join(taskFilesDir(task.ID), file.StoredName)
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", attachmentDisposition(file.Name))
	http.ServeFile(w, r, path)
}

func (h *Handler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	task, ok := h.getTaskWithBoardAccess(w, r, user, roles, groups, "manage")
	if !ok {
		return
	}
	if task.ClosedAt != nil || task.IsArchived {
		respondError(w, http.StatusConflict, "tasks.closedReadOnly")
		return
	}
	fileID := parseInt64Default(chi.URLParam(r, "file_id"), 0)
	if fileID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	file, err := h.svc.Store().GetTaskFile(r.Context(), task.ID, fileID)
	if err != nil || file == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	path := filepath.Join(taskFilesDir(task.ID), file.StoredName)
	_ = os.Remove(path)
	if err := h.svc.Store().DeleteTaskFile(r.Context(), task.ID, fileID); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditTaskFileDelete, fmt.Sprintf("%d|%d", task.ID, fileID))
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func saveTaskFiles(taskID int64, files []*multipart.FileHeader) ([]tasks.TaskFile, error) {
	if len(files) == 0 {
		return nil, nil
	}
	baseDir := taskFilesDir(taskID)
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return nil, err
	}
	var res []tasks.TaskFile
	var created []string
	cleanup := func() {
		for _, p := range created {
			_ = os.Remove(p)
		}
	}
	for _, header := range files {
		if header == nil {
			continue
		}
		src, err := header.Open()
		if err != nil {
			cleanup()
			return nil, err
		}
		id, _ := utils.RandString(12)
		name := filepath.Base(header.Filename)
		if name == "" {
			name = id
		}
		stored := fmt.Sprintf("%s_%s", id, name)
		path := filepath.Join(baseDir, stored)
		dst, err := os.Create(path)
		if err != nil {
			_ = src.Close()
			cleanup()
			return nil, err
		}
		size, err := io.Copy(dst, src)
		_ = src.Close()
		_ = dst.Close()
		if err != nil {
			_ = os.Remove(path)
			cleanup()
			return nil, err
		}
		created = append(created, path)
		res = append(res, tasks.TaskFile{
			Name:        name,
			StoredName:  stored,
			ContentType: header.Header.Get("Content-Type"),
			Size:        size,
		})
	}
	return res, nil
}

func taskFilesDir(taskID int64) string {
	return filepath.Join("data", "tasks", "files", strconv.FormatInt(taskID, 10))
}

func prepareTaskFile(taskID int64, file *tasks.TaskFile) {
	if file == nil {
		return
	}
	file.URL = fmt.Sprintf("/api/tasks/%d/files/%d", taskID, file.ID)
	file.Path = ""
}
