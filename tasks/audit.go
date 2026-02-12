package tasks

import (
	"context"

	"berkut-scc/core/store"
)

const (
	AuditSpaceCreate   = "task.space.create"
	AuditSpaceUpdate   = "task.space.update"
	AuditSpaceDelete   = "task.space.delete"
	AuditBoardCreate   = "task.board.create"
	AuditBoardUpdate   = "task.board.update"
	AuditBoardTemplateDefault = "task.board.template_default"
	AuditBoardDelete   = "task.board.delete"
	AuditBoardMove     = "task.board.move"
	AuditColumnCreate  = "task.column.create"
	AuditColumnUpdate  = "task.column.update"
	AuditColumnTemplateDefault = "task.column.template_default"
	AuditColumnDelete  = "task.column.delete"
	AuditColumnMove    = "task.column.move"
	AuditColumnArchiveTasks = "task.column.archive_tasks"
	AuditSubColumnCreate = "task.subcolumn.create"
	AuditSubColumnUpdate = "task.subcolumn.update"
	AuditSubColumnDelete = "task.subcolumn.delete"
	AuditSubColumnMove   = "task.subcolumn.move"
	AuditTaskCreate    = "task.create"
	AuditTaskUpdate    = "task.update"
	AuditTaskAssign    = "task.assign"
	AuditTaskMove      = "task.move"
	AuditTaskRelocate  = "task.relocate"
	AuditTaskClone     = "task.clone"
	AuditTaskClose     = "task.close"
	AuditTaskArchive   = "task.archive"
	AuditTaskRestore   = "task.restore"
	AuditTaskDelete    = "task.delete"
	AuditCommentAdd    = "task.comment.add"
	AuditCommentUpdate = "task.comment.update"
	AuditCommentDelete = "task.comment.delete"
	AuditCommentFileDelete = "task.comment.file.delete"
	AuditLinkAdd       = "task.link.add"
	AuditLinkRemove    = "task.link.remove"
	AuditLinkPairAdd   = "task.link.pair.add"
	AuditLinkPairRemove = "task.link.pair.remove"
	AuditTaskBlockCreateText      = "task.block.create_text"
	AuditTaskBlockCreateTask      = "task.block.create_task"
	AuditTaskBlockResolveManual   = "task.block.resolve_manual"
	AuditTaskBlockResolveAuto     = "task.block.resolve_auto"
	AuditTaskFileAdd   = "task.file.add"
	AuditTaskFileDelete = "task.file.delete"
	AuditTaskFieldClear = "task.field.clear"
	AuditTaskMoveDeniedBlockedFinal = "task.move.denied_blocked_final"
	AuditTaskCloseDeniedBlocked   = "task.close.denied_blocked"
	AuditTemplateCreate           = "task.template.create"
	AuditTemplateUpdate           = "task.template.update"
	AuditTemplateDelete           = "task.template.delete"
	AuditRecurringCreate          = "task.recurring.create"
	AuditRecurringUpdate          = "task.recurring.update"
	AuditRecurringToggle          = "task.recurring.toggle"
	AuditRecurringRunNow          = "task.recurring.run_now"
	AuditTaskRecurringCreate      = "task.recurring.task_create"
)

func Log(audits store.AuditStore, ctx context.Context, username, action, details string) {
	if audits == nil {
		return
	}
	_ = audits.Log(ctx, username, action, details)
}
