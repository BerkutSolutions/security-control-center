package handlers

import (
	"net/http"
)

const (
	findingAuditCreate  = "finding.create"
	findingAuditUpdate  = "finding.update"
	findingAuditArchive = "finding.archive"
	findingAuditRestore = "finding.restore"
	findingAuditLinkAdd = "finding.link.add"
	findingAuditLinkDel = "finding.link.remove"
)

func (h *FindingsHandler) audit(r *http.Request, action, details string) {
	if h == nil || h.audits == nil {
		return
	}
	_ = h.audits.Log(r.Context(), currentUsername(r), action, details)
}
