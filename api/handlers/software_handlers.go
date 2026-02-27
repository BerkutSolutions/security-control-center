package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"berkut-scc/core/auth"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
)

type SoftwareHandler struct {
	store  store.SoftwareStore
	users  store.UsersStore
	assets store.AssetsStore
	audits store.AuditStore
	policy *rbac.Policy
}

func NewSoftwareHandler(ss store.SoftwareStore, us store.UsersStore, assets store.AssetsStore, audits store.AuditStore, policy *rbac.Policy) *SoftwareHandler {
	return &SoftwareHandler{store: ss, users: us, assets: assets, audits: audits, policy: policy}
}

type softwareProductPayload struct {
	Name        string   `json:"name"`
	Vendor      string   `json:"vendor"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Version     int      `json:"version"`
}

func (p softwareProductPayload) toProduct() (*store.SoftwareProduct, error) {
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return nil, errors.New("software.error.nameRequired")
	}
	if len(name) > 200 {
		return nil, errors.New("software.error.nameTooLong")
	}
	vendor := strings.TrimSpace(p.Vendor)
	if len(vendor) > 200 {
		return nil, errors.New("software.error.vendorTooLong")
	}
	return &store.SoftwareProduct{
		Name:        name,
		Vendor:      vendor,
		Description: strings.TrimSpace(p.Description),
		Tags:        p.Tags,
		Version:     p.Version,
	}, nil
}

func (h *SoftwareHandler) List(w http.ResponseWriter, r *http.Request) {
	user, roles, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	canManage := h.policy != nil && h.policy.Allowed(roles, "software.manage")
	q := r.URL.Query()
	filter := store.SoftwareFilter{
		Search: q.Get("q"),
		Vendor: q.Get("vendor"),
		Tag:    q.Get("tag"),
		Limit:  parseIntDefault(q.Get("limit"), 0),
		Offset: parseIntDefault(q.Get("offset"), 0),
	}
	if canManage && parseBool(q.Get("include_deleted")) {
		filter.IncludeDeleted = true
	}
	items, err := h.store.ListProducts(r.Context(), filter)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *SoftwareHandler) ListLite(w http.ResponseWriter, r *http.Request) {
	if _, _, err := h.currentUser(r); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	limit := parseIntDefault(r.URL.Query().Get("limit"), 100)
	items, err := h.store.ListProductsLite(r.Context(), r.URL.Query().Get("q"), limit)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *SoftwareHandler) Get(w http.ResponseWriter, r *http.Request) {
	if _, _, err := h.currentUser(r); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := parseInt64Default(pathParams(r)["id"], 0)
	if id <= 0 {
		http.Error(w, "software.error.badRequest", http.StatusBadRequest)
		return
	}
	item, err := h.store.GetProduct(r.Context(), id)
	if err != nil || item == nil {
		http.Error(w, "software.error.notFound", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *SoftwareHandler) Create(w http.ResponseWriter, r *http.Request) {
	user, _, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var payload softwareProductPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "software.error.badRequest", http.StatusBadRequest)
		return
	}
	p, err := payload.toProduct()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p.CreatedBy = &user.ID
	p.UpdatedBy = &user.ID
	id, err := h.store.CreateProduct(r.Context(), p)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.audit(r, softwareAuditCreate, strconv.FormatInt(id, 10))
	item, _ := h.store.GetProduct(r.Context(), id)
	writeJSON(w, http.StatusCreated, item)
}

func (h *SoftwareHandler) Update(w http.ResponseWriter, r *http.Request) {
	user, _, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := parseInt64Default(pathParams(r)["id"], 0)
	if id <= 0 {
		http.Error(w, "software.error.badRequest", http.StatusBadRequest)
		return
	}
	existing, err := h.store.GetProduct(r.Context(), id)
	if err != nil || existing == nil || existing.DeletedAt != nil {
		http.Error(w, "software.error.notFound", http.StatusNotFound)
		return
	}
	var payload softwareProductPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "software.error.badRequest", http.StatusBadRequest)
		return
	}
	next, err := payload.toProduct()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if next.Version <= 0 {
		http.Error(w, "software.error.versionRequired", http.StatusBadRequest)
		return
	}
	next.ID = id
	next.CreatedAt = existing.CreatedAt
	next.CreatedBy = existing.CreatedBy
	next.UpdatedBy = &user.ID
	if err := h.store.UpdateProduct(r.Context(), next); err != nil {
		if err == store.ErrConflict {
			http.Error(w, "software.error.conflict", http.StatusConflict)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.audit(r, softwareAuditUpdate, strconv.FormatInt(id, 10))
	item, _ := h.store.GetProduct(r.Context(), id)
	writeJSON(w, http.StatusOK, item)
}

func (h *SoftwareHandler) Archive(w http.ResponseWriter, r *http.Request) {
	user, _, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := parseInt64Default(pathParams(r)["id"], 0)
	if id <= 0 {
		http.Error(w, "software.error.badRequest", http.StatusBadRequest)
		return
	}
	if err := h.store.ArchiveProduct(r.Context(), id, user.ID); err != nil {
		if err == store.ErrConflict {
			http.Error(w, "software.error.conflict", http.StatusConflict)
			return
		}
		http.Error(w, "software.error.notFound", http.StatusNotFound)
		return
	}
	h.audit(r, softwareAuditArchive, strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *SoftwareHandler) Restore(w http.ResponseWriter, r *http.Request) {
	user, _, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := parseInt64Default(pathParams(r)["id"], 0)
	if id <= 0 {
		http.Error(w, "software.error.badRequest", http.StatusBadRequest)
		return
	}
	if err := h.store.RestoreProduct(r.Context(), id, user.ID); err != nil {
		if err == store.ErrConflict {
			http.Error(w, "software.error.conflict", http.StatusConflict)
			return
		}
		http.Error(w, "software.error.notFound", http.StatusNotFound)
		return
	}
	h.audit(r, softwareAuditRestore, strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *SoftwareHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	if _, _, err := h.currentUser(r); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	productID := parseInt64Default(pathParams(r)["id"], 0)
	if productID <= 0 {
		http.Error(w, "software.error.badRequest", http.StatusBadRequest)
		return
	}
	includeDeleted := parseBool(r.URL.Query().Get("include_deleted"))
	items, err := h.store.ListVersions(r.Context(), productID, includeDeleted)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type softwareVersionPayload struct {
	Version     string `json:"version"`
	ReleaseDate string `json:"release_date"`
	EOLDate     string `json:"eol_date"`
	Notes       string `json:"notes"`
}

func parseDateOrEmpty(raw string) (*time.Time, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return nil, err
	}
	tt := t.UTC()
	return &tt, nil
}

func (h *SoftwareHandler) CreateVersion(w http.ResponseWriter, r *http.Request) {
	user, _, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	productID := parseInt64Default(pathParams(r)["id"], 0)
	if productID <= 0 {
		http.Error(w, "software.error.badRequest", http.StatusBadRequest)
		return
	}
	var payload softwareVersionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "software.error.badRequest", http.StatusBadRequest)
		return
	}
	ver := strings.TrimSpace(payload.Version)
	if ver == "" {
		http.Error(w, "software.version.error.required", http.StatusBadRequest)
		return
	}
	release, err := parseDateOrEmpty(payload.ReleaseDate)
	if err != nil {
		http.Error(w, "software.version.error.dateInvalid", http.StatusBadRequest)
		return
	}
	eol, err := parseDateOrEmpty(payload.EOLDate)
	if err != nil {
		http.Error(w, "software.version.error.dateInvalid", http.StatusBadRequest)
		return
	}
	item := &store.SoftwareVersion{
		ProductID:   productID,
		Version:     ver,
		ReleaseDate: release,
		EOLDate:     eol,
		Notes:       strings.TrimSpace(payload.Notes),
		CreatedBy:   &user.ID,
		UpdatedBy:   &user.ID,
	}
	id, err := h.store.CreateVersion(r.Context(), item)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.audit(r, softwareAuditVersionCreate, strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *SoftwareHandler) UpdateVersion(w http.ResponseWriter, r *http.Request) {
	user, _, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	versionID := parseInt64Default(pathParams(r)["version_id"], 0)
	if versionID <= 0 {
		http.Error(w, "software.error.badRequest", http.StatusBadRequest)
		return
	}
	var payload softwareVersionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "software.error.badRequest", http.StatusBadRequest)
		return
	}
	ver := strings.TrimSpace(payload.Version)
	if ver == "" {
		http.Error(w, "software.version.error.required", http.StatusBadRequest)
		return
	}
	release, err := parseDateOrEmpty(payload.ReleaseDate)
	if err != nil {
		http.Error(w, "software.version.error.dateInvalid", http.StatusBadRequest)
		return
	}
	eol, err := parseDateOrEmpty(payload.EOLDate)
	if err != nil {
		http.Error(w, "software.version.error.dateInvalid", http.StatusBadRequest)
		return
	}
	item := &store.SoftwareVersion{
		ID:          versionID,
		Version:     ver,
		ReleaseDate: release,
		EOLDate:     eol,
		Notes:       strings.TrimSpace(payload.Notes),
		UpdatedBy:   &user.ID,
	}
	if err := h.store.UpdateVersion(r.Context(), item); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "software.version.error.notFound", http.StatusNotFound)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.audit(r, softwareAuditVersionUpdate, strconv.FormatInt(versionID, 10))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *SoftwareHandler) ArchiveVersion(w http.ResponseWriter, r *http.Request) {
	user, _, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	versionID := parseInt64Default(pathParams(r)["version_id"], 0)
	if versionID <= 0 {
		http.Error(w, "software.error.badRequest", http.StatusBadRequest)
		return
	}
	if err := h.store.ArchiveVersion(r.Context(), versionID, user.ID); err != nil {
		http.Error(w, "software.version.error.notFound", http.StatusNotFound)
		return
	}
	h.audit(r, softwareAuditVersionArchive, strconv.FormatInt(versionID, 10))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *SoftwareHandler) RestoreVersion(w http.ResponseWriter, r *http.Request) {
	user, _, err := h.currentUser(r)
	if err != nil || user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	versionID := parseInt64Default(pathParams(r)["version_id"], 0)
	if versionID <= 0 {
		http.Error(w, "software.error.badRequest", http.StatusBadRequest)
		return
	}
	if err := h.store.RestoreVersion(r.Context(), versionID, user.ID); err != nil {
		http.Error(w, "software.version.error.notFound", http.StatusNotFound)
		return
	}
	h.audit(r, softwareAuditVersionRestore, strconv.FormatInt(versionID, 10))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *SoftwareHandler) ListProductAssets(w http.ResponseWriter, r *http.Request) {
	if _, _, err := h.currentUser(r); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	productID := parseInt64Default(pathParams(r)["id"], 0)
	if productID <= 0 {
		http.Error(w, "software.error.badRequest", http.StatusBadRequest)
		return
	}
	includeDeleted := parseBool(r.URL.Query().Get("include_deleted"))
	items, err := h.store.ListProductAssets(r.Context(), productID, includeDeleted)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *SoftwareHandler) currentUser(r *http.Request) (*store.User, []string, error) {
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
