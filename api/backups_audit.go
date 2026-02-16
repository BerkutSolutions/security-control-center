package api

import (
	"net/http"
	"strings"

	corebackups "berkut-scc/core/backups"
)

func (s *Server) logBackupsDenied(r *http.Request, username string) {
	if s == nil || s.audits == nil || r == nil {
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/backups") {
		return
	}
	action := corebackups.AuditActionForRequest(r)
	corebackups.Log(s.audits, r.Context(), username, action, "denied", "")
}
