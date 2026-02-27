package handlers

import "net/http"

const (
	softwareAuditCreate         = "software.create"
	softwareAuditUpdate         = "software.update"
	softwareAuditArchive        = "software.archive"
	softwareAuditRestore        = "software.restore"
	softwareAuditVersionCreate  = "software.version.create"
	softwareAuditVersionUpdate  = "software.version.update"
	softwareAuditVersionArchive = "software.version.archive"
	softwareAuditVersionRestore = "software.version.restore"
	softwareAuditInstallAdd     = "software.install.add"
	softwareAuditInstallUpdate  = "software.install.update"
	softwareAuditInstallArchive = "software.install.archive"
	softwareAuditInstallRestore = "software.install.restore"
	softwareAuditExportCSV      = "software.export.csv"
)

func (h *SoftwareHandler) audit(r *http.Request, action, details string) {
	if h == nil || h.audits == nil {
		return
	}
	_ = h.audits.Log(r.Context(), currentUsername(r), action, details)
}
