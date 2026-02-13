package backups

import "berkut-scc/core/rbac"

const (
	PermRead       rbac.Permission = "backups.read"
	PermCreate     rbac.Permission = "backups.create"
	PermImport     rbac.Permission = "backups.import"
	PermPlanUpdate rbac.Permission = "backups.plan.update"
	PermDelete     rbac.Permission = "backups.delete"
	PermDownload   rbac.Permission = "backups.download"
	PermRestore    rbac.Permission = "backups.restore"
)
