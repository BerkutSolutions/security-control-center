package rbac

var permissions = []Permission{
	"app.view", "dashboard.view",
	"accounts.view", "accounts.manage", "accounts.view_dashboard",
	"roles.view", "roles.manage",
	"groups.view", "groups.manage",
	"settings.general", "settings.advanced", "settings.tags", "settings.controls", "settings.incident_options", "settings.detection_sources",
	"docs.view", "docs.create", "docs.upload", "docs.edit", "docs.delete", "docs.manage",
	"docs.classification.set", "docs.export", "docs.versions.view", "docs.versions.restore",
	"docs.approval.start", "docs.approval.view", "docs.approval.approve",
	"folders.view", "folders.manage",
	"templates.view", "templates.manage",
	"controls.view", "controls.manage",
	"controls.checks.view", "controls.checks.manage",
	"controls.violations.view", "controls.violations.manage",
	"controls.frameworks.view", "controls.frameworks.manage",
	"monitoring.view", "monitoring.manage", "monitoring.events.view", "monitoring.settings.manage", "monitoring.certs.view", "monitoring.certs.manage", "monitoring.maintenance.view", "monitoring.maintenance.manage", "monitoring.notifications.view", "monitoring.notifications.manage", "monitoring.incidents.link",
	"tasks.view", "tasks.create", "tasks.edit", "tasks.assign", "tasks.move", "tasks.close", "tasks.archive", "tasks.comment", "tasks.block.create", "tasks.block.resolve", "tasks.block.view", "tasks.templates.view", "tasks.templates.manage", "tasks.recurring.view", "tasks.recurring.manage", "tasks.recurring.run", "tasks.manage",
	"findings.view", "findings.manage",
	"incidents.view", "incidents.create", "incidents.edit", "incidents.delete", "incidents.manage", "incidents.export",
	"reports.view", "reports.create", "reports.edit", "reports.delete", "reports.export", "reports.templates.view", "reports.templates.manage",
	"logs.view",
}

var roles = []Role{
	{Name: "superadmin", Permissions: permissions},
	{Name: "admin", Permissions: []Permission{"app.view", "dashboard.view", "accounts.view", "accounts.manage", "accounts.view_dashboard", "groups.view", "groups.manage", "settings.general", "settings.advanced", "settings.tags", "settings.controls", "settings.incident_options", "settings.detection_sources", "docs.view", "docs.create", "docs.upload", "docs.edit", "docs.delete", "docs.manage", "docs.classification.set", "docs.export", "docs.versions.view", "docs.versions.restore", "docs.approval.start", "docs.approval.view", "docs.approval.approve", "folders.view", "folders.manage", "templates.view", "templates.manage", "controls.view", "controls.manage", "controls.checks.view", "controls.checks.manage", "controls.violations.view", "controls.violations.manage", "controls.frameworks.view", "controls.frameworks.manage", "monitoring.view", "monitoring.manage", "monitoring.events.view", "monitoring.settings.manage", "monitoring.certs.view", "monitoring.certs.manage", "monitoring.maintenance.view", "monitoring.maintenance.manage", "monitoring.notifications.view", "monitoring.notifications.manage", "monitoring.incidents.link", "tasks.view", "tasks.create", "tasks.edit", "tasks.assign", "tasks.move", "tasks.close", "tasks.archive", "tasks.comment", "tasks.block.create", "tasks.block.resolve", "tasks.block.view", "tasks.templates.view", "tasks.templates.manage", "tasks.recurring.view", "tasks.recurring.manage", "tasks.recurring.run", "tasks.manage", "findings.view", "incidents.view", "incidents.create", "incidents.edit", "incidents.delete", "incidents.manage", "incidents.export", "reports.view", "reports.create", "reports.edit", "reports.delete", "reports.export", "reports.templates.view", "reports.templates.manage", "logs.view"}},
	{Name: "security_officer", Permissions: []Permission{"app.view", "dashboard.view", "docs.classification.set", "docs.manage", "docs.view", "docs.export", "docs.versions.view", "folders.manage", "incidents.view", "incidents.create", "incidents.edit", "logs.view"}},
	{Name: "doc_admin", Permissions: []Permission{"app.view", "dashboard.view", "docs.view", "docs.create", "docs.upload", "docs.edit", "docs.delete", "docs.manage", "docs.classification.set", "docs.export", "docs.versions.view", "docs.versions.restore", "docs.approval.start", "docs.approval.view", "docs.approval.approve", "folders.manage", "templates.manage", "incidents.view", "incidents.create", "incidents.edit", "logs.view"}},
	{Name: "doc_editor", Permissions: []Permission{"app.view", "dashboard.view", "docs.view", "docs.create", "docs.upload", "docs.edit", "docs.versions.view", "docs.approval.start", "docs.approval.view", "incidents.view"}},
	{Name: "doc_reviewer", Permissions: []Permission{"app.view", "dashboard.view", "docs.view", "docs.versions.view", "docs.approval.view", "docs.approval.approve"}},
	{Name: "doc_viewer", Permissions: []Permission{"app.view", "dashboard.view", "docs.view", "docs.versions.view"}},
	{Name: "auditor", Permissions: []Permission{"app.view", "dashboard.view", "docs.view", "docs.versions.view", "docs.export", "incidents.view", "reports.view", "reports.export", "logs.view"}},
	{Name: "manager", Permissions: []Permission{"app.view", "dashboard.view", "reports.view"}},
	{Name: "analyst", Permissions: []Permission{"app.view", "dashboard.view", "docs.view", "controls.view", "controls.checks.view", "controls.violations.view", "controls.frameworks.view", "tasks.view", "tasks.create", "tasks.edit", "tasks.comment", "findings.view", "incidents.view"}},
}

func DefaultRoles() []Role {
	out := make([]Role, len(roles))
	copy(out, roles)
	return out
}
