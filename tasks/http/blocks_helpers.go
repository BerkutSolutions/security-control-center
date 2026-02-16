package taskshttp

import (
	"context"

	"berkut-scc/tasks"
)

func (h *Handler) canViewBlockDetails(userID int64, roles []string, task *tasks.Task, assignments []tasks.Assignment) bool {
	if tasks.Allowed(h.policy, roles, tasks.PermBlockView) {
		return true
	}
	if task != nil && task.CreatedBy != nil && *task.CreatedBy == userID {
		return true
	}
	for _, a := range assignments {
		if a.UserID == userID {
			return true
		}
	}
	return false
}

func (h *Handler) activeBlocksForTask(ctx context.Context, taskID int64) ([]tasks.TaskBlock, error) {
	blocksByTask, err := h.svc.Store().ListActiveTaskBlocksForTasks(ctx, []int64{taskID})
	if err != nil {
		return nil, err
	}
	return blocksByTask[taskID], nil
}

func taskBlockInfo(blocks []tasks.TaskBlock, allowDetails bool) ([]tasks.TaskBlockInfo, []int64) {
	if len(blocks) == 0 {
		return nil, nil
	}
	info := make([]tasks.TaskBlockInfo, 0, len(blocks))
	var blockedBy []int64
	for _, block := range blocks {
		item := tasks.TaskBlockInfo{
			BlockType: block.BlockType,
		}
		if allowDetails {
			item.Reason = block.Reason
			item.BlockerTaskID = block.BlockerTaskID
			if block.BlockerTaskID != nil {
				blockedBy = append(blockedBy, *block.BlockerTaskID)
			}
		}
		info = append(info, item)
	}
	return info, blockedBy
}
