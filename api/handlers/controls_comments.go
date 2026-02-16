package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

func (h *ControlsHandler) ListControlComments(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.view")
	if !ok {
		return
	}
	if _, err := h.userFromSession(r.Context(), sess); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	controlID := parseInt64Default(pathParams(r)["id"], 0)
	if controlID == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	control, err := h.store.GetControl(r.Context(), controlID)
	if err != nil || control == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	items, err := h.store.ListControlComments(r.Context(), controlID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	for i := range items {
		prepareControlCommentAttachments(controlID, &items[i])
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *ControlsHandler) AddControlComment(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.manage")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	controlID := parseInt64Default(pathParams(r)["id"], 0)
	if controlID == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	control, err := h.store.GetControl(r.Context(), controlID)
	if err != nil || control == nil {
		http.Error(w, "not found", http.StatusNotFound)
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
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		content = strings.TrimSpace(payload.Content)
	}
	if content == "" && len(files) == 0 {
		http.Error(w, "controls.comments.required", http.StatusBadRequest)
		return
	}
	attachments, err := saveControlCommentFiles(controlID, files)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	comment := &store.ControlComment{
		ControlID:   controlID,
		AuthorID:    user.ID,
		Content:     content,
		Attachments: attachments,
	}
	if _, err := h.store.AddControlComment(r.Context(), comment); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.logAudit(r.Context(), user.Username, "control.comment.add", fmt.Sprintf("%d", control.ID))
	prepareControlCommentAttachments(controlID, comment)
	writeJSON(w, http.StatusCreated, comment)
}

func (h *ControlsHandler) UpdateControlComment(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.view")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	controlID := parseInt64Default(pathParams(r)["id"], 0)
	commentID := parseInt64Default(pathParams(r)["comment_id"], 0)
	if controlID == 0 || commentID == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	control, err := h.store.GetControl(r.Context(), controlID)
	if err != nil || control == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	comment, err := h.store.GetControlComment(r.Context(), commentID)
	if err != nil || comment == nil || comment.ControlID != controlID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !canManageControlComment(user.ID, sess.Roles, h.policy, comment) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var payload struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	content := strings.TrimSpace(payload.Content)
	if content == "" && len(comment.Attachments) == 0 {
		http.Error(w, "controls.comments.required", http.StatusBadRequest)
		return
	}
	comment.Content = content
	if err := h.store.UpdateControlComment(r.Context(), comment); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.logAudit(r.Context(), user.Username, "control.comment.update", fmt.Sprintf("%d|%d", control.ID, comment.ID))
	prepareControlCommentAttachments(controlID, comment)
	writeJSON(w, http.StatusOK, comment)
}

func (h *ControlsHandler) DeleteControlComment(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.view")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	controlID := parseInt64Default(pathParams(r)["id"], 0)
	commentID := parseInt64Default(pathParams(r)["comment_id"], 0)
	if controlID == 0 || commentID == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	control, err := h.store.GetControl(r.Context(), controlID)
	if err != nil || control == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	comment, err := h.store.GetControlComment(r.Context(), commentID)
	if err != nil || comment == nil || comment.ControlID != controlID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !canManageControlComment(user.ID, sess.Roles, h.policy, comment) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	for _, att := range comment.Attachments {
		if att.Path != "" {
			_ = os.Remove(att.Path)
		}
	}
	if err := h.store.DeleteControlComment(r.Context(), comment.ID); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.logAudit(r.Context(), user.Username, "control.comment.delete", fmt.Sprintf("%d|%d", control.ID, comment.ID))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *ControlsHandler) DownloadControlCommentFile(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.view")
	if !ok {
		return
	}
	if _, err := h.userFromSession(r.Context(), sess); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	controlID := parseInt64Default(pathParams(r)["id"], 0)
	commentID := parseInt64Default(pathParams(r)["comment_id"], 0)
	fileID := pathParams(r)["file_id"]
	if controlID == 0 || commentID == 0 || fileID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	control, err := h.store.GetControl(r.Context(), controlID)
	if err != nil || control == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	items, err := h.store.ListControlComments(r.Context(), controlID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
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
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", att.ContentType)
			w.Header().Set("Content-Disposition", attachmentDisposition(att.Name))
			http.ServeFile(w, r, att.Path)
			return
		}
	}
	http.Error(w, "not found", http.StatusNotFound)
}

func (h *ControlsHandler) DeleteControlCommentFile(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.requirePermission(w, r, "controls.view")
	if !ok {
		return
	}
	user, err := h.userFromSession(r.Context(), sess)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	controlID := parseInt64Default(pathParams(r)["id"], 0)
	commentID := parseInt64Default(pathParams(r)["comment_id"], 0)
	fileID := pathParams(r)["file_id"]
	if controlID == 0 || commentID == 0 || fileID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	control, err := h.store.GetControl(r.Context(), controlID)
	if err != nil || control == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	comment, err := h.store.GetControlComment(r.Context(), commentID)
	if err != nil || comment == nil || comment.ControlID != controlID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !canManageControlComment(user.ID, sess.Roles, h.policy, comment) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	updated := make([]store.ControlCommentAttachment, 0, len(comment.Attachments))
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
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	comment.Attachments = updated
	if comment.Content == "" && len(comment.Attachments) == 0 {
		if err := h.store.DeleteControlComment(r.Context(), comment.ID); err != nil {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		h.logAudit(r.Context(), user.Username, "control.comment.delete", fmt.Sprintf("%d|%d", control.ID, comment.ID))
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		return
	}
	if err := h.store.UpdateControlComment(r.Context(), comment); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.logAudit(r.Context(), user.Username, "control.comment.file.delete", fmt.Sprintf("%d|%d|%s", control.ID, comment.ID, fileID))
	prepareControlCommentAttachments(controlID, comment)
	writeJSON(w, http.StatusOK, comment)
}

func saveControlCommentFiles(controlID int64, files []*multipart.FileHeader) ([]store.ControlCommentAttachment, error) {
	if len(files) == 0 {
		return nil, nil
	}
	baseDir := filepath.Join("data", "controls", "comments", fmt.Sprintf("%d", controlID))
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return nil, err
	}
	var res []store.ControlCommentAttachment
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
		res = append(res, store.ControlCommentAttachment{
			ID:          id,
			Name:        name,
			Size:        size,
			ContentType: header.Header.Get("Content-Type"),
			Path:        path,
		})
	}
	return res, nil
}

func prepareControlCommentAttachments(controlID int64, comment *store.ControlComment) {
	if comment == nil {
		return
	}
	for i := range comment.Attachments {
		comment.Attachments[i].URL = fmt.Sprintf("/api/controls/%d/comments/%d/files/%s", controlID, comment.ID, comment.Attachments[i].ID)
		comment.Attachments[i].Path = ""
	}
}

func canManageControlComment(userID int64, roles []string, policy *rbac.Policy, comment *store.ControlComment) bool {
	if comment == nil {
		return false
	}
	if comment.AuthorID == userID {
		return true
	}
	if isAdminRole(roles) {
		return true
	}
	if policy == nil {
		return false
	}
	return policy.Allowed(roles, rbac.Permission("controls.manage"))
}

func isAdminRole(roles []string) bool {
	return hasRole(roles, "admin") || hasRole(roles, "superadmin")
}
