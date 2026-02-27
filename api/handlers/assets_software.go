package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"berkut-scc/core/auth"
	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
)

func (h *AssetsHandler) ListSoftware(w http.ResponseWriter, r *http.Request) {
	user, roles, ok := h.requireSoftwareView(w, r)
	if !ok {
		return
	}
	_ = roles
	assetID := parseInt64Default(pathParams(r)["id"], 0)
	if assetID <= 0 {
		http.Error(w, "assets.error.badRequest", http.StatusBadRequest)
		return
	}
	asset, err := h.store.GetAsset(r.Context(), assetID)
	if err != nil || asset == nil {
		http.Error(w, "assets.error.notFound", http.StatusNotFound)
		return
	}
	includeDeleted := h.policy != nil && h.policy.Allowed(roles, "software.manage") && parseBool(r.URL.Query().Get("include_deleted"))
	items, err := h.sw.ListAssetSoftware(r.Context(), assetID, includeDeleted)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.logAudit(r.Context(), user.Username, "assets.software.view", strconv.FormatInt(assetID, 10))
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type assetSoftwarePayload struct {
	ProductID   int64  `json:"product_id"`
	VersionID   *int64 `json:"version_id"`
	VersionText string `json:"version_text"`
	InstalledAt string `json:"installed_at"`
	Source      string `json:"source"`
	Notes       string `json:"notes"`
}

func (h *AssetsHandler) AddSoftware(w http.ResponseWriter, r *http.Request) {
	user, _, ok := h.requireSoftwareManage(w, r)
	if !ok {
		return
	}
	assetID := parseInt64Default(pathParams(r)["id"], 0)
	if assetID <= 0 {
		http.Error(w, "assets.error.badRequest", http.StatusBadRequest)
		return
	}
	asset, err := h.store.GetAsset(r.Context(), assetID)
	if err != nil || asset == nil || asset.DeletedAt != nil {
		http.Error(w, "assets.error.notFound", http.StatusNotFound)
		return
	}
	var payload assetSoftwarePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "software.install.error.badRequest", http.StatusBadRequest)
		return
	}
	if payload.ProductID <= 0 {
		http.Error(w, "software.install.error.productRequired", http.StatusBadRequest)
		return
	}
	p, err := h.sw.GetProduct(r.Context(), payload.ProductID)
	if err != nil || p == nil || p.DeletedAt != nil {
		http.Error(w, "software.install.error.productNotFound", http.StatusNotFound)
		return
	}
	installedAt, err := parseDateOrEmpty(payload.InstalledAt)
	if err != nil {
		http.Error(w, "software.install.error.dateInvalid", http.StatusBadRequest)
		return
	}
	versionText := strings.TrimSpace(payload.VersionText)
	if payload.VersionID == nil && versionText == "" {
		http.Error(w, "software.install.error.versionRequired", http.StatusBadRequest)
		return
	}
	if payload.VersionID != nil && *payload.VersionID > 0 {
		versions, _ := h.sw.ListVersions(r.Context(), payload.ProductID, true)
		found := false
		for _, v := range versions {
			if v.ID == *payload.VersionID && v.DeletedAt == nil {
				found = true
				break
			}
		}
		if !found {
			http.Error(w, "software.install.error.versionNotFound", http.StatusNotFound)
			return
		}
	}
	inst := &store.AssetSoftwareInstallation{
		AssetID:     assetID,
		ProductID:   payload.ProductID,
		VersionID:   payload.VersionID,
		VersionText: versionText,
		InstalledAt: installedAt,
		Source:      payload.Source,
		Notes:       payload.Notes,
		CreatedBy:   &user.ID,
		UpdatedBy:   &user.ID,
	}
	id, err := h.sw.AddAssetSoftware(r.Context(), inst)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.logAudit(r.Context(), user.Username, "assets.software.add", strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *AssetsHandler) UpdateSoftware(w http.ResponseWriter, r *http.Request) {
	user, _, ok := h.requireSoftwareManage(w, r)
	if !ok {
		return
	}
	instID := parseInt64Default(pathParams(r)["inst_id"], 0)
	if instID <= 0 {
		http.Error(w, "assets.error.badRequest", http.StatusBadRequest)
		return
	}
	var payload assetSoftwarePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "software.install.error.badRequest", http.StatusBadRequest)
		return
	}
	installedAt, err := parseDateOrEmpty(payload.InstalledAt)
	if err != nil {
		http.Error(w, "software.install.error.dateInvalid", http.StatusBadRequest)
		return
	}
	versionText := strings.TrimSpace(payload.VersionText)
	if payload.VersionID == nil && versionText == "" {
		http.Error(w, "software.install.error.versionRequired", http.StatusBadRequest)
		return
	}
	inst := &store.AssetSoftwareInstallation{
		ID:          instID,
		VersionID:   payload.VersionID,
		VersionText: versionText,
		InstalledAt: installedAt,
		Source:      payload.Source,
		Notes:       payload.Notes,
		UpdatedBy:   &user.ID,
	}
	if err := h.sw.UpdateAssetSoftware(r.Context(), inst); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "software.install.error.notFound", http.StatusNotFound)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	h.logAudit(r.Context(), user.Username, "assets.software.update", strconv.FormatInt(instID, 10))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AssetsHandler) ArchiveSoftware(w http.ResponseWriter, r *http.Request) {
	user, _, ok := h.requireSoftwareManage(w, r)
	if !ok {
		return
	}
	instID := parseInt64Default(pathParams(r)["inst_id"], 0)
	if instID <= 0 {
		http.Error(w, "assets.error.badRequest", http.StatusBadRequest)
		return
	}
	if err := h.sw.ArchiveAssetSoftware(r.Context(), instID, user.ID); err != nil {
		http.Error(w, "software.install.error.notFound", http.StatusNotFound)
		return
	}
	h.logAudit(r.Context(), user.Username, "assets.software.archive", strconv.FormatInt(instID, 10))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AssetsHandler) RestoreSoftware(w http.ResponseWriter, r *http.Request) {
	user, _, ok := h.requireSoftwareManage(w, r)
	if !ok {
		return
	}
	instID := parseInt64Default(pathParams(r)["inst_id"], 0)
	if instID <= 0 {
		http.Error(w, "assets.error.badRequest", http.StatusBadRequest)
		return
	}
	if err := h.sw.RestoreAssetSoftware(r.Context(), instID, user.ID); err != nil {
		http.Error(w, "software.install.error.notFound", http.StatusNotFound)
		return
	}
	h.logAudit(r.Context(), user.Username, "assets.software.restore", strconv.FormatInt(instID, 10))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AssetsHandler) requireSoftwareView(w http.ResponseWriter, r *http.Request) (*store.User, []string, bool) {
	return h.requireSoftwarePerm(w, r, "software.view")
}

func (h *AssetsHandler) requireSoftwareManage(w http.ResponseWriter, r *http.Request) (*store.User, []string, bool) {
	return h.requireSoftwarePerm(w, r, "software.manage")
}

func (h *AssetsHandler) requireSoftwarePerm(w http.ResponseWriter, r *http.Request, perm rbac.Permission) (*store.User, []string, bool) {
	val := r.Context().Value(auth.SessionContextKey)
	if val == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil, nil, false
	}
	sess := val.(*store.SessionRecord)
	if h.policy == nil || !h.policy.Allowed(sess.Roles, perm) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return nil, nil, false
	}
	u, _, err := h.users.FindByUsername(r.Context(), sess.Username)
	if err != nil || u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil, nil, false
	}
	groups, _ := h.users.UserGroups(r.Context(), u.ID)
	eff := auth.CalculateEffectiveAccess(u, sess.Roles, groups, h.policy)
	if !allowedByMenuPermissions(eff.MenuPermissions, "software") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return nil, nil, false
	}
	return u, sess.Roles, true
}
