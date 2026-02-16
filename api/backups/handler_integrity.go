package backups

import (
	"context"
	"net/http"
	"strconv"

	corebackups "berkut-scc/core/backups"
)

type integrityProvider interface {
	IntegrityStatus(ctx context.Context) (*corebackups.BackupIntegrityStatus, error)
	StartIntegrityVerification(ctx context.Context, requestedBy string) (*corebackups.RestoreRun, error)
}

func (h *Handler) GetIntegrityStatus(w http.ResponseWriter, r *http.Request) {
	session := currentSession(r)
	provider, ok := h.svc.(integrityProvider)
	if !ok {
		writeError(w, http.StatusNotImplemented, corebackups.ErrorCodeInternal, "common.serverError")
		return
	}
	status, err := provider.IntegrityStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, corebackups.ErrorCodeInternal, "common.serverError")
		return
	}
	corebackups.Log(h.audits, r.Context(), session.Username, corebackups.AuditReadBackup, "success", "resource=integrity")
	writeJSON(w, http.StatusOK, map[string]any{"item": status})
}

func (h *Handler) RunIntegrityTest(w http.ResponseWriter, r *http.Request) {
	session := currentSession(r)
	provider, ok := h.svc.(integrityProvider)
	if !ok {
		writeError(w, http.StatusNotImplemented, corebackups.ErrorCodeInternal, "common.serverError")
		return
	}
	item, err := provider.StartIntegrityVerification(r.Context(), session.Username)
	if err != nil {
		if de, ok := corebackups.AsDomainError(err); ok {
			writeError(w, restoreErrorHTTPStatus(de.Code), de.Code, de.I18NKey)
			return
		}
		writeError(w, http.StatusInternalServerError, corebackups.ErrorCodeInternal, "common.serverError")
		return
	}
	corebackups.Log(h.audits, r.Context(), session.Username, corebackups.AuditRestoreDryRun, "queued", "mode=integrity restore_id="+strconv.FormatInt(item.ID, 10))
	writeJSON(w, http.StatusAccepted, map[string]any{"restore_id": item.ID, "item": item})
}
