package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
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
	"github.com/gorilla/mux"
)

type ReportsHandler struct {
	cfg     *config.AppConfig
	docs    store.DocsStore
	reports store.ReportsStore
	users   store.UsersStore
	policy  *rbac.Policy
	svc     *docs.Service
	incidents store.IncidentsStore
	incidentsSvc *incidents.Service
	controls store.ControlsStore
	monitoring store.MonitoringStore
	tasksSvc *tasks.Service
	audits  store.AuditStore
	logger  *utils.Logger
}

func NewReportsHandler(cfg *config.AppConfig, ds store.DocsStore, rs store.ReportsStore, us store.UsersStore, policy *rbac.Policy, svc *docs.Service, incidents store.IncidentsStore, incidentsSvc *incidents.Service, controls store.ControlsStore, monitoring store.MonitoringStore, tasksSvc *tasks.Service, audits store.AuditStore, logger *utils.Logger) *ReportsHandler {
	return &ReportsHandler{
		cfg: cfg,
		docs: ds,
		reports: rs,
		users: us,
		policy: policy,
		svc: svc,
		incidents: incidents,
		incidentsSvc: incidentsSvc,
		controls: controls,
		monitoring: monitoring,
		tasksSvc: tasksSvc,
		audits: audits,
		logger: logger,
	}
}

func (h *ReportsHandler) currentUser(r *http.Request) (*store.User, []string, error) {
	val := r.Context().Value(auth.SessionContextKey)
	if val == nil {
		return nil, nil, errors.New("no session")
	}
	sess := val.(*store.SessionRecord)
	u, roles, err := h.users.FindByUsername(r.Context(), sess.Username)
	if err != nil || u == nil {
		return u, roles, err
	}
	groups, _ := h.users.UserGroups(r.Context(), u.ID)
	eff := auth.CalculateEffectiveAccess(u, roles, groups, h.policy)
	u.ClearanceLevel = eff.ClearanceLevel
	u.ClearanceTags = eff.ClearanceTags
	return u, eff.Roles, err
}

func (h *ReportsHandler) currentUserWithAccess(r *http.Request) (*store.User, []string, []store.Group, store.EffectiveAccess, error) {
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
	u.ClearanceLevel = eff.ClearanceLevel
	u.ClearanceTags = eff.ClearanceTags
	return u, eff.Roles, groups, eff, err
}

func (h *ReportsHandler) isReport(doc *store.Document) bool {
	return doc != nil && strings.ToLower(strings.TrimSpace(doc.DocType)) == "report"
}

func (h *ReportsHandler) log(ctx context.Context, username, action, details string) {
	if h.audits != nil {
		_ = h.audits.Log(ctx, username, action, details)
	}
}

func (h *ReportsHandler) loadReportForAccess(w http.ResponseWriter, r *http.Request, required string) (*store.Document, *store.ReportMeta, *store.User, []string, bool) {
	user, roles, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil, nil, nil, nil, false
	}
	id, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	doc, err := h.docs.GetDocument(r.Context(), id)
	if err != nil || doc == nil || !h.isReport(doc) {
		http.Error(w, "not found", http.StatusNotFound)
		return nil, nil, nil, nil, false
	}
	docACL, _ := h.docs.GetDocACL(r.Context(), doc.ID)
	if !h.svc.CheckACL(user, roles, doc, docACL, nil, required) {
		http.Error(w, "not found", http.StatusNotFound)
		return nil, nil, nil, nil, false
	}
	meta, _ := h.reports.GetReportMeta(r.Context(), doc.ID)
	if meta == nil {
		meta = &store.ReportMeta{DocID: doc.ID, Status: "draft"}
	}
	return doc, meta, user, roles, true
}

func hasPrivRole(roles []string) bool {
	for _, r := range roles {
		if r == "superadmin" || r == "admin" {
			return true
		}
	}
	return false
}

func parseDateStrict(val string) (*time.Time, error) {
	if strings.TrimSpace(val) == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", val)
	if err != nil {
		return nil, err
	}
	tt := t.UTC()
	return &tt, nil
}

func parseDateParam(val string) *time.Time {
	t, err := parseDateStrict(val)
	if err != nil {
		return nil
	}
	return t
}

func formatPeriod(start, end *time.Time) string {
	if start == nil && end == nil {
		return ""
	}
	if start != nil && end != nil {
		return fmt.Sprintf("%s - %s", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
	if start != nil {
		return start.Format("2006-01-02")
	}
	return end.Format("2006-01-02")
}

func applyTemplate(input string, vars map[string]string) string {
	out := input
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}

func normalizeReportStatus(val string) (string, bool) {
	v := strings.ToLower(strings.TrimSpace(val))
	if v == "" {
		return "draft", true
	}
	switch v {
	case "draft", "final", "archived":
		return v, true
	default:
		return "", false
	}
}

func buildBaseACLFor(user *store.User) []store.ACLRule {
	if user == nil {
		return nil
	}
	return []store.ACLRule{
		{SubjectType: "user", SubjectID: user.Username, Permission: "view"},
		{SubjectType: "user", SubjectID: user.Username, Permission: "edit"},
		{SubjectType: "user", SubjectID: user.Username, Permission: "manage"},
		{SubjectType: "user", SubjectID: user.Username, Permission: "export"},
	}
}

func buildACLFor(roleIDs []string, userIDs []int64) []store.ACLRule {
	var acl []store.ACLRule
	for _, r := range roleIDs {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		for _, p := range []string{"view", "edit"} {
			acl = append(acl, store.ACLRule{SubjectType: "role", SubjectID: r, Permission: p})
		}
	}
	for _, uid := range userIDs {
		if uid == 0 {
			continue
		}
		idStr := fmt.Sprintf("%d", uid)
		for _, p := range []string{"view", "edit"} {
			acl = append(acl, store.ACLRule{SubjectType: "user", SubjectID: idStr, Permission: p})
		}
	}
	return acl
}

func (h *ReportsHandler) defaultReportMarkdown(title string) string {
	return fmt.Sprintf("# %s\n\n## Summary\n\n## Scope\n\n## Findings\n\n## Decisions\n\n## Risks and Recommendations\n", strings.TrimSpace(title))
}
