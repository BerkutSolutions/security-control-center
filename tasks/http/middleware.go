package taskshttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/docs"
	"berkut-scc/core/incidents"
	"berkut-scc/core/rbac"
	cstore "berkut-scc/core/store"
	"berkut-scc/tasks"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	cfg            *config.AppConfig
	svc            *tasks.Service
	users          cstore.UsersStore
	docsStore      cstore.DocsStore
	docsSvc        *docs.Service
	incidentsStore cstore.IncidentsStore
	incidentsSvc   *incidents.Service
	controlsStore  cstore.ControlsStore
	entityLinks    cstore.EntityLinksStore
	policy         *rbac.Policy
	audits         cstore.AuditStore
}

func NewHandler(cfg *config.AppConfig, svc *tasks.Service, users cstore.UsersStore, docsStore cstore.DocsStore, docsSvc *docs.Service, incidentsStore cstore.IncidentsStore, incidentsSvc *incidents.Service, controlsStore cstore.ControlsStore, entityLinks cstore.EntityLinksStore, policy *rbac.Policy, audits cstore.AuditStore) *Handler {
	return &Handler{
		cfg:            cfg,
		svc:            svc,
		users:          users,
		docsStore:      docsStore,
		docsSvc:        docsSvc,
		incidentsStore: incidentsStore,
		incidentsSvc:   incidentsSvc,
		controlsStore:  controlsStore,
		entityLinks:    entityLinks,
		policy:         policy,
		audits:         audits,
	}
}

func (h *Handler) currentUser(r *http.Request) (*cstore.User, []string, []cstore.Group, cstore.EffectiveAccess, error) {
	val := r.Context().Value(auth.SessionContextKey)
	if val == nil {
		return nil, nil, nil, cstore.EffectiveAccess{}, errors.New("no session")
	}
	sess := val.(*cstore.SessionRecord)
	u, roles, err := h.users.FindByUsername(r.Context(), sess.Username)
	if err != nil || u == nil {
		return u, roles, nil, cstore.EffectiveAccess{}, err
	}
	groups, _ := h.users.UserGroups(r.Context(), u.ID)
	eff := auth.CalculateEffectiveAccess(u, roles, groups, h.policy)
	return u, eff.Roles, groups, eff, err
}

func (h *Handler) getTaskWithBoardAccess(w http.ResponseWriter, r *http.Request, user *cstore.User, roles []string, groups []cstore.Group, required string) (*tasks.Task, bool) {
	taskID := parseInt64Default(chi.URLParam(r, "id"), 0)
	if taskID == 0 {
		respondError(w, http.StatusBadRequest, "bad request")
		return nil, false
	}
	task, err := h.svc.Store().GetTask(r.Context(), taskID)
	if err != nil || task == nil {
		respondError(w, http.StatusNotFound, "tasks.notFound")
		return nil, false
	}
	board, _ := h.svc.Store().GetBoard(r.Context(), task.BoardID)
	spaceACL := []tasks.ACLRule{}
	if board != nil && board.SpaceID > 0 {
		spaceACL, _ = h.svc.Store().GetSpaceACL(r.Context(), board.SpaceID)
	}
	boardACL, _ := h.svc.Store().GetBoardACL(r.Context(), task.BoardID)
	if !boardAllowed(user, roles, groups, spaceACL, boardACL, required) {
		respondError(w, http.StatusNotFound, "tasks.notFound")
		return nil, false
	}
	return task, true
}

func (h *Handler) resolveUserIDs(ctx context.Context, tokens []string) ([]int64, error) {
	seen := map[int64]struct{}{}
	var out []int64
	for _, raw := range tokens {
		token := strings.TrimSpace(raw)
		if token == "" {
			continue
		}
		u, err := h.lookupUserByToken(ctx, token)
		if err != nil || u == nil {
			return nil, errors.New("user not found")
		}
		if _, ok := seen[u.ID]; ok {
			continue
		}
		seen[u.ID] = struct{}{}
		out = append(out, u.ID)
	}
	return out, nil
}

func (h *Handler) lookupUserByToken(ctx context.Context, token string) (*cstore.User, error) {
	t := strings.TrimSpace(token)
	if t == "" {
		return nil, errors.New("empty token")
	}
	if id, err := strconv.ParseInt(t, 10, 64); err == nil {
		u, _, err := h.users.Get(ctx, id)
		return u, err
	}
	u, _, err := h.users.FindByUsername(ctx, strings.ToLower(t))
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, errors.New("not found")
	}
	return u, nil
}

func (h *Handler) canViewByClassification(eff cstore.EffectiveAccess, level int, tags []string) bool {
	if h.cfg != nil && h.cfg.Security.TagsSubsetEnforced {
		return docs.HasClearance(docs.ClassificationLevel(eff.ClearanceLevel), eff.ClearanceTags, docs.ClassificationLevel(level), tags)
	}
	return eff.ClearanceLevel >= level
}

func boardAllowed(user *cstore.User, roles []string, groups []cstore.Group, spaceACL []tasks.ACLRule, boardACL []tasks.ACLRule, required string) bool {
	if len(spaceACL) > 0 {
		return aclAllowed(user, roles, groups, spaceACL, required)
	}
	return aclAllowed(user, roles, groups, boardACL, required)
}

func aclAllowed(user *cstore.User, roles []string, groups []cstore.Group, acl []tasks.ACLRule, required string) bool {
	if user == nil {
		return false
	}
	req := strings.ToLower(strings.TrimSpace(required))
	if req == "" {
		return false
	}
	allowed := map[string]struct{}{req: {}}
	switch req {
	case "view":
		allowed["manage"] = struct{}{}
	}
	for _, rule := range acl {
		if _, ok := allowed[strings.ToLower(rule.Permission)]; !ok {
			continue
		}
		switch strings.ToLower(rule.SubjectType) {
		case "all":
			return true
		case "user":
			if rule.SubjectID == user.Username || rule.SubjectID == fmt.Sprintf("%d", user.ID) {
				return true
			}
		case "role":
			for _, r := range roles {
				if r == rule.SubjectID {
					return true
				}
			}
		case "department":
			if strings.EqualFold(user.Department, rule.SubjectID) {
				return true
			}
		case "group":
			for _, g := range groups {
				if strings.EqualFold(g.Name, rule.SubjectID) || fmt.Sprintf("%d", g.ID) == rule.SubjectID {
					return true
				}
			}
		}
	}
	return false
}

func hasRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

func isAdminRole(roles []string) bool {
	return hasRole(roles, "admin") || hasRole(roles, "superadmin")
}

func (h *Handler) canManageComment(userID int64, roles []string, comment *tasks.Comment) bool {
	if comment == nil {
		return false
	}
	if comment.AuthorID == userID {
		return true
	}
	if isAdminRole(roles) {
		return true
	}
	return tasks.Allowed(h.policy, roles, tasks.PermManage)
}

func respondJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func respondError(w http.ResponseWriter, status int, msg string) {
	http.Error(w, msg, status)
}

func parseMultipartFormLimited(w http.ResponseWriter, r *http.Request, maxBytes int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+1)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			respondError(w, http.StatusRequestEntityTooLarge, "payload too large")
			return err
		}
		respondError(w, http.StatusBadRequest, "bad request")
		return err
	}
	return nil
}

func parseIntDefault(val string, def int) int {
	if val == "" {
		return def
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil {
		return def
	}
	return parsed
}

func parseInt64Default(val string, def int64) int64 {
	if val == "" {
		return def
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
	if err != nil {
		return def
	}
	return parsed
}
