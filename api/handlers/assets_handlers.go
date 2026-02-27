package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/auth"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
)

type AssetsHandler struct {
	store  store.AssetsStore
	sw     store.SoftwareStore
	users  store.UsersStore
	audits store.AuditStore
	policy *rbac.Policy
}

func NewAssetsHandler(as store.AssetsStore, sw store.SoftwareStore, us store.UsersStore, audits store.AuditStore, policy *rbac.Policy) *AssetsHandler {
	return &AssetsHandler{store: as, sw: sw, users: us, audits: audits, policy: policy}
}

var validAssetTypes = map[string]struct{}{
	"host":        {},
	"service":     {},
	"application": {},
	"network":     {},
	"other":       {},
}

var validAssetCriticality = map[string]struct{}{
	"low":      {},
	"medium":   {},
	"high":     {},
	"critical": {},
}

var validAssetEnv = map[string]struct{}{
	"prod":  {},
	"stage": {},
	"dev":   {},
	"test":  {},
	"other": {},
}

var validAssetStatus = map[string]struct{}{
	"active":         {},
	"decommissioned": {},
}

func (h *AssetsHandler) List(w http.ResponseWriter, r *http.Request) {
	user, roles, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	canManage := h.policy != nil && h.policy.Allowed(roles, "assets.manage")
	q := r.URL.Query()
	filter := store.AssetFilter{
		Search:      q.Get("q"),
		Type:        q.Get("type"),
		Criticality: q.Get("criticality"),
		Env:         q.Get("env"),
		Status:      q.Get("status"),
		Tag:         q.Get("tag"),
		Limit:       parseIntDefault(q.Get("limit"), 0),
		Offset:      parseIntDefault(q.Get("offset"), 0),
	}
	if canManage && (q.Get("include_deleted") == "1" || strings.ToLower(q.Get("include_deleted")) == "true") {
		filter.IncludeDeleted = true
	}
	items, err := h.store.ListAssets(r.Context(), filter)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *AssetsHandler) ListLite(w http.ResponseWriter, r *http.Request) {
	if _, _, err := h.currentUser(r); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	limit := parseIntDefault(r.URL.Query().Get("limit"), 100)
	items, err := h.store.ListAssetsLite(r.Context(), r.URL.Query().Get("q"), limit)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *AssetsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if _, _, err := h.currentUser(r); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := parseInt64Default(pathParams(r)["id"], 0)
	if id <= 0 {
		http.Error(w, "assets.error.badRequest", http.StatusBadRequest)
		return
	}
	item, err := h.store.GetAsset(r.Context(), id)
	if err != nil || item == nil {
		http.Error(w, "assets.error.notFound", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *AssetsHandler) Create(w http.ResponseWriter, r *http.Request) {
	user, _, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var payload assetPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "assets.error.badRequest", http.StatusBadRequest)
		return
	}
	a, err := payload.toAsset()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.CreatedBy = &user.ID
	a.UpdatedBy = &user.ID
	if _, err := h.store.CreateAsset(r.Context(), a); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.logAudit(r.Context(), user.Username, "assets.create", strconv.FormatInt(a.ID, 10))
	writeJSON(w, http.StatusCreated, a)
}

func (h *AssetsHandler) Update(w http.ResponseWriter, r *http.Request) {
	user, _, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := parseInt64Default(pathParams(r)["id"], 0)
	if id <= 0 {
		http.Error(w, "assets.error.badRequest", http.StatusBadRequest)
		return
	}
	existing, err := h.store.GetAsset(r.Context(), id)
	if err != nil || existing == nil {
		http.Error(w, "assets.error.notFound", http.StatusNotFound)
		return
	}
	if existing.DeletedAt != nil {
		http.Error(w, "assets.error.notFound", http.StatusNotFound)
		return
	}
	var payload assetPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "assets.error.badRequest", http.StatusBadRequest)
		return
	}
	next, err := payload.toAsset()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	next.ID = existing.ID
	next.CreatedAt = existing.CreatedAt
	next.CreatedBy = existing.CreatedBy
	next.UpdatedBy = &user.ID

	if err := h.store.UpdateAsset(r.Context(), next); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "assets.error.notFound", http.StatusNotFound)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.logAudit(r.Context(), user.Username, "assets.update", strconv.FormatInt(next.ID, 10))
	writeJSON(w, http.StatusOK, next)
}

func (h *AssetsHandler) Archive(w http.ResponseWriter, r *http.Request) {
	user, _, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := parseInt64Default(pathParams(r)["id"], 0)
	if id <= 0 {
		http.Error(w, "assets.error.badRequest", http.StatusBadRequest)
		return
	}
	if err := h.store.ArchiveAsset(r.Context(), id, user.ID); err != nil {
		http.Error(w, "assets.error.notFound", http.StatusNotFound)
		return
	}
	h.logAudit(r.Context(), user.Username, "assets.archive", strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AssetsHandler) Restore(w http.ResponseWriter, r *http.Request) {
	user, _, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := parseInt64Default(pathParams(r)["id"], 0)
	if id <= 0 {
		http.Error(w, "assets.error.badRequest", http.StatusBadRequest)
		return
	}
	if err := h.store.RestoreAsset(r.Context(), id, user.ID); err != nil {
		http.Error(w, "assets.error.notFound", http.StatusNotFound)
		return
	}
	h.logAudit(r.Context(), user.Username, "assets.restore", strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type assetPayload struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	Description    string   `json:"description"`
	CommissionedAt string   `json:"commissioned_at"`
	IPAddresses    []string `json:"ip_addresses"`
	Criticality    string   `json:"criticality"`
	Owner          string   `json:"owner"`
	Administrator  string   `json:"administrator"`
	Env            string   `json:"env"`
	Status         string   `json:"status"`
	Tags           []string `json:"tags"`
}

func (p assetPayload) toAsset() (*store.Asset, error) {
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return nil, errors.New("assets.error.required")
	}
	if len(name) > 200 {
		return nil, errors.New("assets.error.nameTooLong")
	}
	typ := strings.ToLower(strings.TrimSpace(p.Type))
	if typ == "" {
		typ = "other"
	}
	if _, ok := validAssetTypes[typ]; !ok {
		return nil, errors.New("assets.error.typeInvalid")
	}
	crit := strings.ToLower(strings.TrimSpace(p.Criticality))
	if crit == "" {
		crit = "medium"
	}
	if _, ok := validAssetCriticality[crit]; !ok {
		return nil, errors.New("assets.error.criticalityInvalid")
	}
	env := strings.ToLower(strings.TrimSpace(p.Env))
	if env == "" {
		env = "other"
	}
	if _, ok := validAssetEnv[env]; !ok {
		return nil, errors.New("assets.error.envInvalid")
	}
	status := strings.ToLower(strings.TrimSpace(p.Status))
	if status == "" {
		status = "active"
	}
	if _, ok := validAssetStatus[status]; !ok {
		return nil, errors.New("assets.error.statusInvalid")
	}
	if err := validateIPList(p.IPAddresses); err != nil {
		return nil, errors.New("assets.error.ipInvalid")
	}
	var commissioned *time.Time
	if v := strings.TrimSpace(p.CommissionedAt); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return nil, errors.New("assets.error.dateInvalid")
		}
		tt := t.UTC()
		commissioned = &tt
	}
	return &store.Asset{
		Name:           name,
		Type:           typ,
		Description:    strings.TrimSpace(p.Description),
		CommissionedAt: commissioned,
		IPAddresses:    p.IPAddresses,
		Criticality:    crit,
		Owner:          strings.TrimSpace(p.Owner),
		Administrator:  strings.TrimSpace(p.Administrator),
		Env:            env,
		Status:         status,
		Tags:           p.Tags,
	}, nil
}

func (h *AssetsHandler) currentUser(r *http.Request) (*store.User, []string, error) {
	val := r.Context().Value(auth.SessionContextKey)
	if val == nil {
		return nil, nil, errors.New("no session")
	}
	sess := val.(*store.SessionRecord)
	u, _, err := h.users.FindByUsername(r.Context(), sess.Username)
	if err != nil {
		return nil, nil, err
	}
	return u, sess.Roles, nil
}

func (h *AssetsHandler) logAudit(ctx context.Context, username, action, details string) {
	if h.audits == nil {
		return
	}
	_ = h.audits.Log(ctx, username, action, details)
}

func validateIPList(ips []string) error {
	for _, raw := range ips {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if net.ParseIP(v) == nil {
			return errors.New("invalid ip")
		}
	}
	return nil
}
