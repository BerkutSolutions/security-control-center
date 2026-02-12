package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/docs"
	"berkut-scc/core/incidents"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
	"berkut-scc/tasks"
	"berkut-scc/gui"
)

type DashboardHandler struct {
	cfg            *config.AppConfig
	dash           store.DashboardStore
	users          store.UsersStore
	docsStore      store.DocsStore
	incidentsStore store.IncidentsStore
	docsSvc        *docs.Service
	incidentsSvc   *incidents.Service
	tasksStore     tasks.Store
	audits         store.AuditStore
	policy         *rbac.Policy
	logger         *utils.Logger
}

type DashboardLayout struct {
	Order    []string                         `json:"order"`
	Hidden   []string                         `json:"hidden"`
	Settings map[string]map[string]interface{} `json:"settings,omitempty"`
}

func NewDashboardHandler(cfg *config.AppConfig, dash store.DashboardStore, users store.UsersStore, docsStore store.DocsStore, incidentsStore store.IncidentsStore, docsSvc *docs.Service, incidentsSvc *incidents.Service, tasksStore tasks.Store, audits store.AuditStore, policy *rbac.Policy, logger *utils.Logger) *DashboardHandler {
	return &DashboardHandler{
		cfg:            cfg,
		dash:           dash,
		users:          users,
		docsStore:      docsStore,
		incidentsStore: incidentsStore,
		docsSvc:        docsSvc,
		incidentsSvc:   incidentsSvc,
		tasksStore:     tasksStore,
		audits:         audits,
		policy:         policy,
		logger:         logger,
	}
}

func (h *DashboardHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	data, err := gui.StaticFiles.ReadFile("static/dashboard.html")
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	http.ServeContent(w, r, "dashboard.html", time.Now(), bytes.NewReader(data))
}

func (h *DashboardHandler) Data(w http.ResponseWriter, r *http.Request) {
	user, roles, groups, eff, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	perms := permissionsMap(eff.Permissions)
	allowedFrames := h.allowedFrames(perms)

	defaultLayout := defaultDashboardLayout(eff.Roles, perms, allowedFrames)
	layout, err := h.loadLayout(r.Context(), user.ID, eff.Roles, perms, allowedFrames)
	if err != nil && h.logger != nil {
		h.logger.Errorf("dashboard layout load: %v", err)
	}
	summary, todo, docsBlock, incidentsBlock, tasksBlock := h.collectData(r.Context(), user, roles, groups, eff, perms)

	writeJSON(w, http.StatusOK, map[string]any{
		"layout":           layout,
		"default_layout":   defaultLayout,
		"frames":           allowedFrames,
		"summary":          summary,
		"todo":             todo,
		"documents":        docsBlock,
		"incidents":        incidentsBlock,
		"tasks":            tasksBlock,
		"tasks_available":  perms["tasks.view"],
	})
}

func (h *DashboardHandler) SaveLayout(w http.ResponseWriter, r *http.Request) {
	user, _, _, eff, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var payload struct {
		Layout DashboardLayout `json:"layout"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	perms := permissionsMap(eff.Permissions)
	allowedFrames := h.allowedFrames(perms)
	layout := sanitizeLayout(payload.Layout, allowedFrames)
	layout = ensureDefaultSettings(layout)
	raw, err := json.Marshal(layout)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := h.dash.SaveLayout(r.Context(), user.ID, string(raw)); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.dashLog(r.Context(), user.Username, "dashboard.layout.save", "")
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *DashboardHandler) collectData(ctx context.Context, user *store.User, roles []string, groups []store.Group, eff store.EffectiveAccess, perms map[string]bool) (map[string]any, map[string]any, map[string]any, map[string]any, map[string]any) {
	summary := map[string]any{
		"docs_on_approval": nil,
		"tasks_overdue":    nil,
		"incidents_open":   nil,
		"accounts_blocked": nil,
	}
	todo := map[string]any{
		"approvals_pending": nil,
		"tasks_assigned":    nil,
		"docs_returned":     nil,
		"incidents_assigned": nil,
	}
	documents := map[string]any{
		"on_approval":    nil,
		"approved_30d":   nil,
		"returned":       nil,
	}
	incidents := map[string]any{
		"open":          nil,
		"critical":      nil,
		"new_last_7d":   nil,
		"closed":        nil,
	}
	tasksBlock := map[string]any{
		"total":        nil,
		"mine":         nil,
		"overdue":      nil,
		"blocked":      nil,
		"completed_30d": nil,
	}

	var docsOnApproval int
	if perms["docs.view"] {
		docsOnApproval = h.countDocsByStatus(ctx, user, roles, eff, docs.StatusReview, func(_ store.Document) bool { return true })
		summary["docs_on_approval"] = docsOnApproval
		documents["on_approval"] = docsOnApproval
		approved30 := h.countDocsByStatus(ctx, user, roles, eff, docs.StatusApproved, func(d store.Document) bool {
			return d.UpdatedAt.After(time.Now().UTC().AddDate(0, 0, -30))
		})
		documents["approved_30d"] = approved30
		returnedAll := h.countDocsByStatus(ctx, user, roles, eff, docs.StatusReturned, func(_ store.Document) bool { return true })
		documents["returned"] = returnedAll
		returnedMine := h.countDocsByStatus(ctx, user, roles, eff, docs.StatusReturned, func(d store.Document) bool {
			return d.CreatedBy == user.ID
		})
		todo["docs_returned"] = returnedMine
	}

	if perms["docs.approval.view"] || perms["docs.approval.approve"] {
		pending := h.countPendingApprovals(ctx, user, roles, eff)
		todo["approvals_pending"] = pending
	}

	if perms["incidents.view"] {
		openCount, criticalCount, newCount, closedCount := h.countIncidents(ctx, user, roles, eff)
		summary["incidents_open"] = openCount
		incidents["open"] = openCount
		incidents["critical"] = criticalCount
		incidents["new_last_7d"] = newCount
		incidents["closed"] = closedCount
		statusCounts := h.countIncidentStatuses(ctx, user, roles, eff)
		for _, status := range incidentStatusList() {
			incidents["status_"+status] = statusCounts[status]
		}
		todo["incidents_assigned"] = h.countAssignedIncidents(ctx, user, roles, eff)
	}

	if perms["accounts.view_dashboard"] {
		blocked := h.countBlockedAccounts(ctx)
		summary["accounts_blocked"] = blocked
	}

	if perms["tasks.view"] && h.tasksStore != nil {
		taskStats := h.countTasks(ctx, user, roles, groups)
		summary["tasks_overdue"] = taskStats.overdue
		todo["tasks_assigned"] = taskStats.mine
		tasksBlock["total"] = taskStats.total
		tasksBlock["mine"] = taskStats.mine
		tasksBlock["overdue"] = taskStats.overdue
		tasksBlock["blocked"] = taskStats.blocked
		tasksBlock["completed_30d"] = taskStats.completed30d
	}

	return summary, todo, documents, incidents, tasksBlock
}

func (h *DashboardHandler) countDocsByStatus(ctx context.Context, user *store.User, roles []string, eff store.EffectiveAccess, status string, extraFilter func(store.Document) bool) int {
	docsList, err := h.docsStore.ListDocuments(ctx, store.DocumentFilter{Status: status})
	if err != nil {
		return 0
	}
	count := 0
	for _, d := range docsList {
		docACL, _ := h.docsStore.GetDocACL(ctx, d.ID)
		var folderACL []store.ACLRule
		if d.FolderID != nil {
			folderACL, _ = h.docsStore.GetFolderACL(ctx, *d.FolderID)
		}
		if !h.docsSvc.CheckACL(user, roles, &d, docACL, folderACL, "view") {
			continue
		}
		if !h.canViewByClassification(eff, d.ClassificationLevel, d.ClassificationTags) {
			continue
		}
		if d.Status == docs.StatusReview {
			ap, parts, _ := h.docsStore.GetActiveApproval(ctx, d.ID)
			if ap != nil && !isApprovalParticipant(parts, user.ID) && !hasRole(roles, "doc_admin") && !hasRole(roles, "admin") {
				continue
			}
		}
		if extraFilter != nil && !extraFilter(d) {
			continue
		}
		count++
	}
	return count
}

func (h *DashboardHandler) countPendingApprovals(ctx context.Context, user *store.User, roles []string, eff store.EffectiveAccess) int {
	items, err := h.docsStore.ListApprovals(ctx, store.ApprovalFilter{UserID: user.ID, Status: docs.StatusReview})
	if err != nil {
		return 0
	}
	count := 0
	for _, item := range items {
		ap, parts, err := h.docsStore.GetApproval(ctx, item.ID)
		if err != nil || ap == nil {
			continue
		}
		isPending := false
		for _, p := range parts {
			if p.UserID == user.ID && p.Role == "approver" && p.Decision == nil && p.Stage == ap.CurrentStage {
				isPending = true
				break
			}
		}
		if !isPending {
			continue
		}
		doc, err := h.docsStore.GetDocument(ctx, ap.DocID)
		if err != nil || doc == nil {
			continue
		}
		if !h.canViewByClassification(eff, doc.ClassificationLevel, doc.ClassificationTags) {
			continue
		}
		count++
	}
	return count
}

func (h *DashboardHandler) countIncidents(ctx context.Context, user *store.User, roles []string, eff store.EffectiveAccess) (int, int, int, int) {
	items, err := h.incidentsStore.ListIncidents(ctx, store.IncidentFilter{IncludeDeleted: false})
	if err != nil {
		return 0, 0, 0, 0
	}
	openCount := 0
	critical := 0
	newCount := 0
	closedCount := 0
	cutoff := time.Now().UTC().AddDate(0, 0, -7)
	for _, inc := range items {
		acl, _ := h.incidentsStore.GetIncidentACL(ctx, inc.ID)
		if !h.incidentsSvc.CheckACL(user, roles, acl, "view") {
			continue
		}
		if !h.canViewByClassification(eff, inc.ClassificationLevel, inc.ClassificationTags) {
			continue
		}
		if strings.ToLower(inc.Status) != "closed" {
			openCount++
		} else {
			closedCount++
		}
		if inc.Severity == "critical" {
			critical++
		}
		if inc.CreatedAt.After(cutoff) {
			newCount++
		}
	}
	return openCount, critical, newCount, closedCount
}

func incidentStatusList() []string {
	return []string{
		"draft",
		"open",
		"in_progress",
		"contained",
		"resolved",
		"waiting",
		"waiting_info",
		"approval",
		"closed",
	}
}

func (h *DashboardHandler) countIncidentStatuses(ctx context.Context, user *store.User, roles []string, eff store.EffectiveAccess) map[string]int {
	items, err := h.incidentsStore.ListIncidents(ctx, store.IncidentFilter{IncludeDeleted: false})
	if err != nil {
		return map[string]int{}
	}
	statuses := map[string]int{}
	for _, s := range incidentStatusList() {
		statuses[s] = 0
	}
	for _, inc := range items {
		acl, _ := h.incidentsStore.GetIncidentACL(ctx, inc.ID)
		if !h.incidentsSvc.CheckACL(user, roles, acl, "view") {
			continue
		}
		if !h.canViewByClassification(eff, inc.ClassificationLevel, inc.ClassificationTags) {
			continue
		}
		st := strings.ToLower(strings.TrimSpace(inc.Status))
		if _, ok := statuses[st]; ok {
			statuses[st]++
		}
	}
	return statuses
}

func (h *DashboardHandler) countAssignedIncidents(ctx context.Context, user *store.User, roles []string, eff store.EffectiveAccess) int {
	if user == nil {
		return 0
	}
	items, err := h.incidentsStore.ListIncidents(ctx, store.IncidentFilter{
		IncludeDeleted: false,
		AssignedUserID: user.ID,
	})
	if err != nil {
		return 0
	}
	count := 0
	for _, inc := range items {
		acl, _ := h.incidentsStore.GetIncidentACL(ctx, inc.ID)
		if !h.incidentsSvc.CheckACL(user, roles, acl, "view") {
			continue
		}
		if !h.canViewByClassification(eff, inc.ClassificationLevel, inc.ClassificationTags) {
			continue
		}
		if strings.ToLower(inc.Status) == "closed" {
			continue
		}
		count++
	}
	return count
}

func (h *DashboardHandler) countBlockedAccounts(ctx context.Context) int {
	users, err := h.users.List(ctx)
	if err != nil {
		return 0
	}
	now := time.Now().UTC()
	blocked := 0
	for _, u := range users {
		if u.LockedUntil != nil && u.LockedUntil.After(now) {
			blocked++
		}
	}
	return blocked
}

type taskStats struct {
	total        int
	mine         int
	overdue      int
	blocked      int
	completed30d int
}

func (h *DashboardHandler) countTasks(ctx context.Context, user *store.User, roles []string, groups []store.Group) taskStats {
	stats := taskStats{}
	if user == nil || h.tasksStore == nil {
		return stats
	}
	items, err := h.tasksStore.ListTasks(ctx, tasks.TaskFilter{IncludeArchived: false})
	if err != nil {
		return stats
	}
	boardIDs := map[int64]struct{}{}
	for _, t := range items {
		boardIDs[t.BoardID] = struct{}{}
	}
	boardInfo := map[int64]*tasks.Board{}
	boardACL := map[int64][]tasks.ACLRule{}
	spaceACL := map[int64][]tasks.ACLRule{}
	for id := range boardIDs {
		board, _ := h.tasksStore.GetBoard(ctx, id)
		boardInfo[id] = board
		acl, _ := h.tasksStore.GetBoardACL(ctx, id)
		boardACL[id] = acl
		if board != nil && board.SpaceID > 0 {
			if _, ok := spaceACL[board.SpaceID]; !ok {
				acl, _ := h.tasksStore.GetSpaceACL(ctx, board.SpaceID)
				spaceACL[board.SpaceID] = acl
			}
		}
	}
	var visible []tasks.Task
	for _, t := range items {
		board := boardInfo[t.BoardID]
		if board == nil || !board.IsActive {
			continue
		}
		spaceID := board.SpaceID
		if !taskBoardAllowed(user, roles, groups, spaceACL[spaceID], boardACL[t.BoardID], "view") {
			continue
		}
		visible = append(visible, t)
	}
	if len(visible) == 0 {
		return stats
	}
	taskIDs := make([]int64, 0, len(visible))
	for _, t := range visible {
		taskIDs = append(taskIDs, t.ID)
	}
	assignments, _ := h.tasksStore.ListTaskAssignmentsForTasks(ctx, taskIDs)
	blocksByTask, _ := h.tasksStore.ListActiveTaskBlocksForTasks(ctx, taskIDs)
	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, -30)
	for _, t := range visible {
		stats.total++
		if t.ClosedAt != nil && t.ClosedAt.After(cutoff) {
			stats.completed30d++
		}
		if t.ClosedAt == nil && t.DueDate != nil && t.DueDate.Before(now) {
			stats.overdue++
		}
		if t.ClosedAt == nil && len(blocksByTask[t.ID]) > 0 {
			stats.blocked++
		}
		if t.ClosedAt == nil && assignedTo(assignments[t.ID], user.ID) {
			stats.mine++
		}
	}
	return stats
}

func assignedTo(assignments []tasks.Assignment, userID int64) bool {
	for _, a := range assignments {
		if a.UserID == userID {
			return true
		}
	}
	return false
}

func (h *DashboardHandler) currentUser(r *http.Request) (*store.User, []string, []store.Group, store.EffectiveAccess, error) {
	val := r.Context().Value(auth.SessionContextKey)
	if val == nil {
		return nil, nil, nil, store.EffectiveAccess{}, errors.New("no session")
	}
	sess := val.(*store.SessionRecord)
	u, roles, err := h.users.FindByUsername(r.Context(), sess.Username)
	if err != nil || u == nil {
		return u, roles, nil, store.EffectiveAccess{}, err
	}
	groups, _ := h.users.UserGroups(r.Context(), u.ID)
	eff := auth.CalculateEffectiveAccess(u, roles, groups, h.policy)
	return u, eff.Roles, groups, eff, err
}

func (h *DashboardHandler) canViewByClassification(eff store.EffectiveAccess, level int, tags []string) bool {
	if h.cfg != nil && h.cfg.Security.TagsSubsetEnforced {
		return docs.HasClearance(docs.ClassificationLevel(eff.ClearanceLevel), eff.ClearanceTags, docs.ClassificationLevel(level), tags)
	}
	return eff.ClearanceLevel >= level
}

func (h *DashboardHandler) allowedFrames(perms map[string]bool) []map[string]string {
	frames := []struct {
		ID    string
		Title string
		Perm  string
	}{
		{ID: "summary", Title: "dashboard.frame.summary", Perm: ""},
		{ID: "tasks", Title: "dashboard.frame.tasks", Perm: "tasks.view"},
		{ID: "todo", Title: "dashboard.frame.todo", Perm: "docs.view"},
		{ID: "incidents", Title: "dashboard.frame.incidents", Perm: "incidents.view"},
		{ID: "documents", Title: "dashboard.frame.documents", Perm: "docs.view"},
		{ID: "incident_chart", Title: "dashboard.frame.incidentChart", Perm: "incidents.view"},
		{ID: "activity", Title: "dashboard.frame.activity", Perm: "logs.view"},
	}
	var allowed []map[string]string
	for _, f := range frames {
		if f.Perm != "" && !perms[f.Perm] {
			continue
		}
		allowed = append(allowed, map[string]string{"id": f.ID, "title": f.Title})
	}
	return allowed
}

func permissionsMap(perms []string) map[string]bool {
	res := map[string]bool{}
	for _, p := range perms {
		res[p] = true
	}
	return res
}

func defaultDashboardLayout(roles []string, perms map[string]bool, frames []map[string]string) DashboardLayout {
	roleSet := map[string]bool{}
	for _, r := range roles {
		roleSet[r] = true
	}
	var order []string
	switch {
	case roleSet["admin"]:
		order = []string{"summary", "tasks", "todo", "incidents", "documents"}
	case roleSet["security_officer"]:
		order = []string{"summary", "tasks", "todo", "incidents", "documents"}
	case roleSet["analyst"]:
		order = []string{"summary", "tasks", "todo", "incidents", "documents"}
	case roleSet["doc_admin"]:
		order = []string{"summary", "tasks", "todo", "documents", "incidents"}
	case roleSet["doc_reviewer"]:
		order = []string{"summary", "tasks", "todo", "documents"}
	case roleSet["doc_editor"]:
		order = []string{"summary", "tasks", "todo", "documents", "incidents"}
	case roleSet["doc_viewer"]:
		order = []string{"summary", "tasks", "documents"}
	case roleSet["auditor"]:
		order = []string{"summary", "tasks", "incidents", "documents"}
	case roleSet["manager"]:
		order = []string{"summary", "tasks", "documents", "incidents"}
	default:
		order = []string{"summary", "tasks", "documents", "incidents", "todo"}
	}
	allowed := map[string]bool{}
	for _, f := range frames {
		allowed[f["id"]] = true
	}
	var filtered []string
	for _, id := range order {
		if allowed[id] {
			filtered = append(filtered, id)
		}
	}
	hidden := []string{}
	if allowed["incident_chart"] {
		hidden = append(hidden, "incident_chart")
	}
	if allowed["activity"] {
		hidden = append(hidden, "activity")
	}
	return ensureDefaultSettings(sanitizeLayout(DashboardLayout{Order: filtered, Hidden: hidden}, frames))
}

func sanitizeLayout(layout DashboardLayout, frames []map[string]string) DashboardLayout {
	allowed := map[string]bool{}
	for _, f := range frames {
		allowed[f["id"]] = true
	}
	order := []string{}
	seen := map[string]bool{}
	for _, id := range layout.Order {
		id = strings.TrimSpace(id)
		if id == "" || !allowed[id] || seen[id] {
			continue
		}
		seen[id] = true
		order = append(order, id)
	}
	for id := range allowed {
		if !seen[id] {
			order = append(order, id)
		}
	}
	hidden := []string{}
	hiddenSeen := map[string]bool{}
	for _, id := range layout.Hidden {
		if !allowed[id] || hiddenSeen[id] {
			continue
		}
		hiddenSeen[id] = true
		hidden = append(hidden, id)
	}
	settings := map[string]map[string]interface{}{}
	for key, value := range layout.Settings {
		if !allowed[key] || value == nil {
			continue
		}
		settings[key] = value
	}
	return DashboardLayout{Order: order, Hidden: hidden, Settings: settings}
}

func ensureDefaultSettings(layout DashboardLayout) DashboardLayout {
	if layout.Settings == nil {
		layout.Settings = map[string]map[string]interface{}{}
	}
	events, ok := layout.Settings["events"]
	if !ok || events == nil {
		return layout
	}
	limit := 20
	if raw, ok := events["limit"]; ok {
		switch v := raw.(type) {
		case float64:
			limit = int(v)
		case int:
			limit = v
		}
	}
	if limit < 5 {
		limit = 5
	}
	if limit > 100 {
		limit = 100
	}
	onlyImportant := boolFromAny(events["only_important"])
	onlyMine := boolFromAny(events["only_mine"])
	layout.Settings["events"] = map[string]interface{}{"limit": limit, "only_important": onlyImportant, "only_mine": onlyMine}
	return layout
}

func boolFromAny(val interface{}) bool {
	switch v := val.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case int:
		return v != 0
	case string:
		return strings.ToLower(strings.TrimSpace(v)) == "true"
	default:
		return false
	}
}

func (h *DashboardHandler) dashLog(ctx context.Context, username, action, details string) {
	if h.audits != nil {
		_ = h.audits.Log(ctx, username, action, details)
	}
}

func (h *DashboardHandler) loadLayout(ctx context.Context, userID int64, roles []string, perms map[string]bool, allowedFrames []map[string]string) (DashboardLayout, error) {
	defaultLayout := defaultDashboardLayout(roles, perms, allowedFrames)
	if h.dash == nil {
		return defaultLayout, nil
	}
	raw, err := h.dash.GetLayout(ctx, userID)
	if err != nil {
		return defaultLayout, err
	}
	if strings.TrimSpace(raw) == "" {
		return defaultLayout, nil
	}
	var stored DashboardLayout
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return defaultLayout, nil
	}
	return ensureDefaultSettings(sanitizeLayout(stored, allowedFrames)), nil
}
