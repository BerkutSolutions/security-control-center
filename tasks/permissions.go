package tasks

import "berkut-scc/core/rbac"

const (
	PermView            rbac.Permission = "tasks.view"
	PermCreate          rbac.Permission = "tasks.create"
	PermEdit            rbac.Permission = "tasks.edit"
	PermAssign          rbac.Permission = "tasks.assign"
	PermMove            rbac.Permission = "tasks.move"
	PermClose           rbac.Permission = "tasks.close"
	PermArchive         rbac.Permission = "tasks.archive"
	PermComment         rbac.Permission = "tasks.comment"
	PermBlockCreate     rbac.Permission = "tasks.block.create"
	PermBlockResolve    rbac.Permission = "tasks.block.resolve"
	PermBlockView       rbac.Permission = "tasks.block.view"
	PermTemplatesView   rbac.Permission = "tasks.templates.view"
	PermTemplatesManage rbac.Permission = "tasks.templates.manage"
	PermRecurringView   rbac.Permission = "tasks.recurring.view"
	PermRecurringManage rbac.Permission = "tasks.recurring.manage"
	PermRecurringRun    rbac.Permission = "tasks.recurring.run"
	PermManage          rbac.Permission = "tasks.manage"
)

func Allowed(policy *rbac.Policy, roles []string, perm rbac.Permission) bool {
	if policy == nil {
		return false
	}
	return policy.Allowed(roles, perm)
}
