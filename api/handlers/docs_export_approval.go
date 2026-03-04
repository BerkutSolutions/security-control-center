package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"berkut-scc/core/store"
)

var exportApprovalRoleRank = map[string]int{
	"superadmin":       100,
	"admin":            90,
	"doc_admin":        80,
	"security_officer": 70,
	"doc_reviewer":     60,
	"doc_editor":       50,
	"auditor":          40,
	"analyst":          30,
	"manager":          20,
	"doc_viewer":       10,
}

func highestExportRoleRank(roles []string) int {
	rank := 0
	for _, role := range roles {
		val := exportApprovalRoleRank[strings.ToLower(strings.TrimSpace(role))]
		if val > rank {
			rank = val
		}
	}
	return rank
}

func (h *DocsHandler) validateExportApprovalPair(approver *store.User, approverRoles []string, requester *store.User, requesterRoles []string) (string, int) {
	if approver == nil || requester == nil {
		return "forbidden", http.StatusForbidden
	}
	if requester.ID == approver.ID {
		return "docs.export.selfApprovalForbidden", http.StatusBadRequest
	}
	if strings.EqualFold(strings.TrimSpace(requester.Username), "admin") && !hasRole(approverRoles, "superadmin") {
		return "docs.export.adminRequiresSuperadmin", http.StatusForbidden
	}
	approverRank := highestExportRoleRank(approverRoles)
	requesterRank := highestExportRoleRank(requesterRoles)
	if approverRank < requesterRank {
		return "docs.export.targetRankTooLow", http.StatusForbidden
	}
	return "", http.StatusOK
}

func (h *DocsHandler) canUserParticipateInExportApproval(user *store.User, roles []string, doc *store.Document, docACL, folderACL []store.ACLRule) bool {
	if user == nil || doc == nil {
		return false
	}
	// Export approver must be able to access the document. "export" is preferred,
	// but legacy ACL entries may contain only view/edit.
	return h.svc.CheckACL(user, roles, doc, docACL, folderACL, "export") ||
		h.svc.CheckACL(user, roles, doc, docACL, folderACL, "edit") ||
		h.svc.CheckACL(user, roles, doc, docACL, folderACL, "view")
}

func (h *DocsHandler) ListExportApprovalCandidates(w http.ResponseWriter, r *http.Request) {
	requester, requesterRoles, err := h.currentUser(r)
	if err != nil || requester == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id, _ := strconv.ParseInt(pathParams(r)["id"], 10, 64)
	doc, err := h.store.GetDocument(r.Context(), id)
	if err != nil || doc == nil || !h.isDocument(doc) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	docACL, _ := h.store.GetDocACL(r.Context(), doc.ID)
	var folderACL []store.ACLRule
	if doc.FolderID != nil {
		folderACL, _ = h.store.GetFolderACL(r.Context(), *doc.FolderID)
	}
	if !h.svc.CheckACL(requester, requesterRoles, doc, docACL, folderACL, "export") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	users, err := h.users.List(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	items := make([]map[string]any, 0, len(users))
	for i := range users {
		u := users[i]
		if !u.Active || u.ID == requester.ID {
			continue
		}
		if !h.canUserParticipateInExportApproval(&u.User, u.Roles, doc, docACL, folderACL) {
			continue
		}
		if key, _ := h.validateExportApprovalPair(&u.User, u.Roles, requester, requesterRoles); key != "" {
			continue
		}
		displayName := strings.TrimSpace(u.FullName)
		if displayName == "" {
			displayName = u.Username
		}
		items = append(items, map[string]any{
			"id":           u.ID,
			"username":     u.Username,
			"full_name":    u.FullName,
			"display_name": displayName,
			"roles":        u.Roles,
			"rank":         highestExportRoleRank(u.Roles),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		li := strings.ToLower(strings.TrimSpace(items[i]["display_name"].(string)))
		lj := strings.ToLower(strings.TrimSpace(items[j]["display_name"].(string)))
		if li == lj {
			return items[i]["username"].(string) < items[j]["username"].(string)
		}
		return li < lj
	})
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
