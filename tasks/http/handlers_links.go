package taskshttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	cstore "berkut-scc/core/store"
	"berkut-scc/tasks"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) ListLinks(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	task, ok := h.getTaskWithBoardAccess(w, r, user, roles, groups, "view")
	if !ok {
		return
	}
	items, err := h.svc.Store().ListEntityLinks(r.Context(), "task", fmt.Sprintf("%d", task.ID))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) ListControlLinks(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	task, ok := h.getTaskWithBoardAccess(w, r, user, roles, groups, "view")
	if !ok {
		return
	}
	links, err := h.entityLinks.ListByTarget(r.Context(), "task", fmt.Sprintf("%d", task.ID))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	type controlLink struct {
		ID           int64  `json:"id"`
		ControlID    int64  `json:"control_id"`
		Code         string `json:"code"`
		Title        string `json:"title"`
		RelationType string `json:"relation_type"`
	}
	var items []controlLink
	for _, link := range links {
		if link.SourceType != "control" {
			continue
		}
		ctrlID, err := strconv.ParseInt(link.SourceID, 10, 64)
		if err != nil || ctrlID == 0 {
			continue
		}
		ctrl, err := h.controlsStore.GetControl(r.Context(), ctrlID)
		if err != nil || ctrl == nil {
			continue
		}
		items = append(items, controlLink{
			ID:           link.ID,
			ControlID:    ctrl.ID,
			Code:         ctrl.Code,
			Title:        ctrl.Title,
			RelationType: link.RelationType,
		})
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) AddLink(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, eff, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermEdit) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	task, ok := h.getTaskWithBoardAccess(w, r, user, roles, groups, "manage")
	if !ok {
		return
	}
	var payload struct {
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	targetType := strings.ToLower(strings.TrimSpace(payload.TargetType))
	targetID := strings.TrimSpace(payload.TargetID)
	if targetType == "" || targetID == "" {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	switch targetType {
	case "doc":
		docID, err := strconv.ParseInt(targetID, 10, 64)
		if err != nil || docID == 0 {
			respondError(w, http.StatusBadRequest, "bad request")
			return
		}
		doc, err := h.docsStore.GetDocument(r.Context(), docID)
		if err != nil || doc == nil {
			respondError(w, http.StatusNotFound, "not found")
			return
		}
		docACL, _ := h.docsStore.GetDocACL(r.Context(), doc.ID)
		var folderACL []cstore.ACLRule
		if doc.FolderID != nil {
			folderACL, _ = h.docsStore.GetFolderACL(r.Context(), *doc.FolderID)
		}
		if !h.docsSvc.CheckACL(user, roles, doc, docACL, folderACL, "view") || !h.canViewByClassification(eff, doc.ClassificationLevel, doc.ClassificationTags) {
			respondError(w, http.StatusForbidden, "forbidden")
			return
		}
	case "incident":
		incID, err := strconv.ParseInt(targetID, 10, 64)
		if err != nil || incID == 0 {
			respondError(w, http.StatusBadRequest, "bad request")
			return
		}
		inc, err := h.incidentsStore.GetIncident(r.Context(), incID)
		if err != nil || inc == nil || inc.DeletedAt != nil {
			respondError(w, http.StatusNotFound, "not found")
			return
		}
		incACL, _ := h.incidentsStore.GetIncidentACL(r.Context(), inc.ID)
		if !h.incidentsSvc.CheckACL(user, roles, incACL, "view") || !h.canViewByClassification(eff, inc.ClassificationLevel, inc.ClassificationTags) {
			respondError(w, http.StatusForbidden, "forbidden")
			return
		}
	case "task", "task_parent", "task_child":
		refID, err := strconv.ParseInt(targetID, 10, 64)
		if err != nil || refID == 0 {
			respondError(w, http.StatusBadRequest, "bad request")
			return
		}
		if refID == task.ID {
			respondError(w, http.StatusBadRequest, "bad request")
			return
		}
		ref, err := h.svc.Store().GetTask(r.Context(), refID)
		if err != nil || ref == nil {
			respondError(w, http.StatusNotFound, "not found")
			return
		}
		refBoard, _ := h.svc.Store().GetBoard(r.Context(), ref.BoardID)
		spaceACL := []tasks.ACLRule{}
		if refBoard != nil && refBoard.SpaceID > 0 {
			spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), refBoard.SpaceID)
		}
		refACL, _ := h.svc.Store().GetBoardACL(r.Context(), ref.BoardID)
		if !boardAllowed(user, roles, groups, spaceACL, refACL, "view") {
			respondError(w, http.StatusForbidden, "forbidden")
			return
		}
	case "control":
		// Controls module is not enforced at this stage.
	default:
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	link := &tasks.Link{
		SourceType: "task",
		SourceID:   fmt.Sprintf("%d", task.ID),
		TargetType: targetType,
		TargetID:   targetID,
	}
	if _, err := h.svc.Store().AddEntityLink(r.Context(), link); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditLinkAdd, fmt.Sprintf("%d|%s|%s", task.ID, targetType, targetID))
	if targetType == "task_parent" || targetType == "task_child" {
		pairType := "task_parent"
		if targetType == "task_parent" {
			pairType = "task_child"
		}
		pair := &tasks.Link{
			SourceType: "task",
			SourceID:   targetID,
			TargetType: pairType,
			TargetID:   fmt.Sprintf("%d", task.ID),
		}
		if _, err := h.svc.Store().AddEntityLink(r.Context(), pair); err == nil {
			tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditLinkPairAdd, fmt.Sprintf("%s|%s|%d", targetID, pairType, task.ID))
		}
	}
	respondJSON(w, http.StatusCreated, link)
}

func (h *Handler) DeleteLink(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, _, err := h.currentUser(r)
	if err != nil || user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if !tasks.Allowed(h.policy, roles, tasks.PermEdit) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	task, ok := h.getTaskWithBoardAccess(w, r, user, roles, groups, "manage")
	if !ok {
		return
	}
	linkID := parseInt64Default(chi.URLParam(r, "link_id"), 0)
	if linkID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return
	}
	links, err := h.svc.Store().ListEntityLinks(r.Context(), "task", fmt.Sprintf("%d", task.ID))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	var target *tasks.Link
	for i := range links {
		if links[i].ID == linkID {
			target = &links[i]
			break
		}
	}
	if target == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	if err := h.svc.Store().DeleteEntityLink(r.Context(), linkID); err != nil {
		respondError(w, http.StatusInternalServerError, "server error")
		return
	}
	tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditLinkRemove, fmt.Sprintf("%d|%s|%s", task.ID, target.TargetType, target.TargetID))
	if target.TargetType == "task_parent" || target.TargetType == "task_child" {
		pairType := "task_parent"
		if target.TargetType == "task_parent" {
			pairType = "task_child"
		}
		pairLinks, err := h.svc.Store().ListEntityLinks(r.Context(), "task", target.TargetID)
		if err == nil {
			for _, pl := range pairLinks {
				if pl.TargetType == pairType && pl.TargetID == fmt.Sprintf("%d", task.ID) {
					_ = h.svc.Store().DeleteEntityLink(r.Context(), pl.ID)
					tasks.Log(h.audits, r.Context(), user.Username, tasks.AuditLinkPairRemove, fmt.Sprintf("%s|%s|%d", target.TargetID, pairType, task.ID))
					break
				}
			}
		}
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
