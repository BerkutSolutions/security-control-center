package taskshttp

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"berkut-scc/core/utils"
	"berkut-scc/tasks"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) ListComments(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	task, ok := h.getTaskWithBoardAccess(w, r, user, roles, groups, "view")
	if !ok {
		return
	}
	items, err := h.svc.Store().ListTaskComments(r.Context(), task.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	for i := range items {
		prepareCommentAttachments(task.ID, &items[i])
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) AddComment(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermComment) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	task, ok := h.getTaskWithBoardAccess(w, r, user, roles, groups, "view")
	if !ok {
		return
	}
	if task.ClosedAt != nil || task.IsArchived {
		respondError(w, http.StatusConflict, "tasks.closedReadOnly")
		return
	}
	var content string
	var files []*multipart.FileHeader
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := parseMultipartFormLimited(w, r, 25<<20); err != nil {
			return
		}
		content = strings.TrimSpace(r.FormValue("content"))
		if r.MultipartForm != nil {
			files = r.MultipartForm.File["files"]
		}
	} else {
		var payload struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			respondError(w, http.StatusBadRequest, "bad request")
			return
		}
		content = strings.TrimSpace(payload.Content)
	}
	if content == "" && len(files) == 0 {
		respondError(w, http.StatusBadRequest, "tasks.commentRequired")
		return
	}
	attachments, err := saveCommentFiles(task.ID, files)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	comment := &tasks.Comment{
		TaskID:      task.ID,
		AuthorID:    user.ID,
		Content:     content,
		Attachments: attachments,
	}
	if _, err := h.svc.Store().AddTaskComment(r.Context(), comment); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditCommentAdd, fmt.Sprintf("%d", task.ID))
	prepareCommentAttachments(task.ID, comment)
	respondJSON(w, http.StatusCreated, comment)
}

func (h *Handler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	task, ok := h.getTaskWithBoardAccess(w, r, user, roles, groups, "view")
	if !ok {
		return
	}
	if task.ClosedAt != nil || task.IsArchived {
		respondError(w, http.StatusConflict, "tasks.closedReadOnly")
		return
	}
	commentID := parseInt64Default(chi.URLParam(r, "comment_id"), 0)
	if commentID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	comment, err := h.svc.Store().GetTaskComment(r.Context(), commentID)
	if err != nil || comment == nil || comment.TaskID != task.ID {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	if !h.canManageComment(user.ID, roles, comment) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	var payload struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	content := strings.TrimSpace(payload.Content)
	if content == "" && len(comment.Attachments) == 0 {
		respondError(w, http.StatusBadRequest, "tasks.commentRequired")
		return
	}
	comment.Content = content
	if err := h.svc.Store().UpdateTaskComment(r.Context(), comment); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditCommentUpdate, fmt.Sprintf("%d|%d", task.ID, comment.ID))
	prepareCommentAttachments(task.ID, comment)
	respondJSON(w, http.StatusOK, comment)
}

func (h *Handler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	task, ok := h.getTaskWithBoardAccess(w, r, user, roles, groups, "view")
	if !ok {
		return
	}
	if task.ClosedAt != nil || task.IsArchived {
		respondError(w, http.StatusConflict, "tasks.closedReadOnly")
		return
	}
	commentID := parseInt64Default(chi.URLParam(r, "comment_id"), 0)
	if commentID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	comment, err := h.svc.Store().GetTaskComment(r.Context(), commentID)
	if err != nil || comment == nil || comment.TaskID != task.ID {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	if !h.canManageComment(user.ID, roles, comment) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	for _, att := range comment.Attachments {
		if att.Path != "" {
			_ = os.Remove(att.Path)
		}
	}
	if err := h.svc.Store().DeleteTaskComment(r.Context(), comment.ID); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditCommentDelete, fmt.Sprintf("%d|%d", task.ID, comment.ID))
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) DownloadCommentFile(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	task, ok := h.getTaskWithBoardAccess(w, r, user, roles, groups, "view")
	if !ok {
		return
	}
	commentID, _ := strconv.ParseInt(chi.URLParam(r, "comment_id"), 10, 64)
	fileID := chi.URLParam(r, "file_id")
	if commentID == 0 || fileID == "" {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	items, err := h.svc.Store().ListTaskComments(r.Context(), task.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	for _, c := range items {
		if c.ID != commentID {
			continue
		}
		for _, att := range c.Attachments {
			if att.ID != fileID {
				continue
			}
			if att.Path == "" {
				respondError(w, http.StatusNotFound, "not found")
				return
			}
			w.Header().Set("Content-Type", att.ContentType)
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", att.Name))
			http.ServeFile(w, r, att.Path)
			return
		}
	}
	respondError(w, http.StatusNotFound, "not found")
}

func (h *Handler) DeleteCommentFile(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	task, ok := h.getTaskWithBoardAccess(w, r, user, roles, groups, "view")
	if !ok {
		return
	}
	if task.ClosedAt != nil || task.IsArchived {
		respondError(w, http.StatusConflict, "tasks.closedReadOnly")
		return
	}
	commentID := parseInt64Default(chi.URLParam(r, "comment_id"), 0)
	fileID := chi.URLParam(r, "file_id")
	if commentID == 0 || fileID == "" {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	comment, err := h.svc.Store().GetTaskComment(r.Context(), commentID)
	if err != nil || comment == nil || comment.TaskID != task.ID {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	if !h.canManageComment(user.ID, roles, comment) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	updated := make([]tasks.CommentAttachment, 0, len(comment.Attachments))
	removed := false
	for _, att := range comment.Attachments {
		if att.ID != fileID {
			updated = append(updated, att)
			continue
		}
		removed = true
		if att.Path != "" {
			_ = os.Remove(att.Path)
		}
	}
	if !removed {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	comment.Attachments = updated
	if comment.Content == "" && len(comment.Attachments) == 0 {
		if err := h.svc.Store().DeleteTaskComment(r.Context(), comment.ID); err != nil {
			respondError(w, http.StatusInternalServerError, "server error")
			return
		}
		tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditCommentDelete, fmt.Sprintf("%d|%d", task.ID, comment.ID))
		respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		return
	}
	if err := h.svc.Store().UpdateTaskComment(r.Context(), comment); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditCommentFileDelete, fmt.Sprintf("%d|%d|%s", task.ID, comment.ID, fileID))
	prepareCommentAttachments(task.ID, comment)
	respondJSON(w, http.StatusOK, comment)
}

func saveCommentFiles(taskID int64, files []*multipart.FileHeader) ([]tasks.CommentAttachment, error) {
	if len(files) == 0 {
		return nil, nil
	}
	baseDir := filepath.Join("data", "tasks", "comments", fmt.Sprintf("%d", taskID))
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return nil, err
	}
	var res []tasks.CommentAttachment
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
		res = append(res, tasks.CommentAttachment{
			ID:          id,
			Name:        name,
			Size:        size,
			ContentType: header.Header.Get("Content-Type"),
			Path:        path,
		})
	}
	return res, nil
}

func prepareCommentAttachments(taskID int64, comment *tasks.Comment) {
	if comment == nil {
		return
	}
	for i := range comment.Attachments {
		comment.Attachments[i].URL = fmt.Sprintf("/api/tasks/%d/comments/%d/files/%s", taskID, comment.ID, comment.Attachments[i].ID)
		comment.Attachments[i].Path = ""
	}
}
