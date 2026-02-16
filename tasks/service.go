package tasks

import (
	"context"
	"time"
)

type Store interface {
	CreateSpace(ctx context.Context, space *Space, acl []ACLRule) (int64, error)
	UpdateSpace(ctx context.Context, space *Space) error
	DeleteSpace(ctx context.Context, spaceID int64) error
	GetSpace(ctx context.Context, spaceID int64) (*Space, error)
	ListSpaces(ctx context.Context, filter SpaceFilter) ([]Space, error)
	SetSpaceACL(ctx context.Context, spaceID int64, acl []ACLRule) error
	GetSpaceACL(ctx context.Context, spaceID int64) ([]ACLRule, error)

	CreateBoard(ctx context.Context, board *Board, acl []ACLRule) (int64, error)
	UpdateBoard(ctx context.Context, board *Board) error
	MoveBoard(ctx context.Context, boardID int64, position int) (*Board, error)
	DeleteBoard(ctx context.Context, boardID int64) error
	GetBoard(ctx context.Context, boardID int64) (*Board, error)
	ListBoards(ctx context.Context, filter BoardFilter) ([]Board, error)
	SetBoardACL(ctx context.Context, boardID int64, acl []ACLRule) error
	GetBoardACL(ctx context.Context, boardID int64) ([]ACLRule, error)
	NextBoardPosition(ctx context.Context, spaceID int64) (int, error)
	GetBoardLayout(ctx context.Context, userID, spaceID int64) (string, error)
	SaveBoardLayout(ctx context.Context, userID, spaceID int64, layoutJSON string) error

	CreateColumn(ctx context.Context, column *Column) (int64, error)
	UpdateColumn(ctx context.Context, column *Column) error
	MoveColumn(ctx context.Context, columnID int64, position int) (*Column, error)
	DeleteColumn(ctx context.Context, columnID int64) error
	GetColumn(ctx context.Context, columnID int64) (*Column, error)
	ListColumns(ctx context.Context, boardID int64, includeInactive bool) ([]Column, error)
	NextColumnPosition(ctx context.Context, boardID int64) (int, error)

	CreateSubColumn(ctx context.Context, subcolumn *SubColumn) (int64, error)
	UpdateSubColumn(ctx context.Context, subcolumn *SubColumn) error
	MoveSubColumn(ctx context.Context, subcolumnID int64, position int) (*SubColumn, error)
	DeleteSubColumn(ctx context.Context, subcolumnID int64) error
	GetSubColumn(ctx context.Context, subcolumnID int64) (*SubColumn, error)
	ListSubColumns(ctx context.Context, columnID int64, includeInactive bool) ([]SubColumn, error)
	ListSubColumnsByBoard(ctx context.Context, boardID int64, includeInactive bool) ([]SubColumn, error)
	NextSubColumnPosition(ctx context.Context, columnID int64) (int, error)
	CountTasksInSubColumn(ctx context.Context, subcolumnID int64) (int, error)

	CreateTask(ctx context.Context, task *Task, assignments []int64) (int64, error)
	CreateTaskWithLinks(ctx context.Context, task *Task, assignments []int64, links []Link) (int64, error)
	UpdateTask(ctx context.Context, task *Task) error
	MoveTask(ctx context.Context, taskID int64, columnID int64, subcolumnID *int64, position int) (*Task, error)
	RelocateTask(ctx context.Context, taskID int64, boardID int64, columnID int64, subcolumnID *int64, position int) (*Task, error)
	CloseTask(ctx context.Context, taskID int64, userID int64) (*Task, error)
	ArchiveTask(ctx context.Context, taskID int64, userID int64) (*Task, error)
	ArchiveTasksByColumn(ctx context.Context, columnID int64, userID int64) (int, error)
	ListArchivedTasks(ctx context.Context, filter TaskFilter) ([]TaskArchiveEntry, error)
	RestoreTask(ctx context.Context, taskID int64, boardID int64, columnID int64, subcolumnID *int64, userID int64) (*Task, error)
	DeleteTask(ctx context.Context, taskID int64) error
	GetTask(ctx context.Context, taskID int64) (*Task, error)
	ListTasks(ctx context.Context, filter TaskFilter) ([]Task, error)
	CountTasksByBoard(ctx context.Context, boardIDs []int64) (map[int64]int, error)

	CreateTaskTemplate(ctx context.Context, tpl *TaskTemplate) (int64, error)
	UpdateTaskTemplate(ctx context.Context, tpl *TaskTemplate) error
	DeleteTaskTemplate(ctx context.Context, id int64) error
	GetTaskTemplate(ctx context.Context, id int64) (*TaskTemplate, error)
	ListTaskTemplates(ctx context.Context, filter TaskTemplateFilter) ([]TaskTemplate, error)

	CreateTaskRecurringRule(ctx context.Context, rule *TaskRecurringRule) (int64, error)
	UpdateTaskRecurringRule(ctx context.Context, rule *TaskRecurringRule) error
	GetTaskRecurringRule(ctx context.Context, id int64) (*TaskRecurringRule, error)
	ListTaskRecurringRules(ctx context.Context, filter TaskRecurringRuleFilter) ([]TaskRecurringRule, error)
	ListDueRecurringRules(ctx context.Context, now time.Time, limit int) ([]TaskRecurringRule, error)
	UpdateRecurringRuleRun(ctx context.Context, ruleID int64, lastRunAt, nextRunAt time.Time) error
	CreateRecurringInstanceTask(ctx context.Context, rule *TaskRecurringRule, template *TaskTemplate, scheduledFor time.Time) (*Task, bool, error)

	SetTaskAssignments(ctx context.Context, taskID int64, userIDs []int64, assignedBy int64) error
	ListTaskAssignments(ctx context.Context, taskID int64) ([]Assignment, error)
	ListTaskAssignmentsForTasks(ctx context.Context, taskIDs []int64) (map[int64][]Assignment, error)

	AddTaskComment(ctx context.Context, comment *Comment) (int64, error)
	ListTaskComments(ctx context.Context, taskID int64) ([]Comment, error)
	GetTaskComment(ctx context.Context, commentID int64) (*Comment, error)
	UpdateTaskComment(ctx context.Context, comment *Comment) error
	DeleteTaskComment(ctx context.Context, commentID int64) error

	ListTaskFiles(ctx context.Context, taskID int64) ([]TaskFile, error)
	AddTaskFile(ctx context.Context, file *TaskFile) (int64, error)
	GetTaskFile(ctx context.Context, taskID, fileID int64) (*TaskFile, error)
	DeleteTaskFile(ctx context.Context, taskID, fileID int64) error

	CreateTaskBlock(ctx context.Context, block *TaskBlock) (int64, error)
	ResolveTaskBlock(ctx context.Context, taskID int64, blockID int64, resolvedBy int64) (*TaskBlock, error)
	ListTaskBlocks(ctx context.Context, taskID int64) ([]TaskBlock, error)
	ListActiveTaskBlocksForTasks(ctx context.Context, taskIDs []int64) (map[int64][]TaskBlock, error)
	ListActiveBlocksByBlocker(ctx context.Context, blockerTaskID int64) ([]TaskBlock, error)
	ResolveTaskBlocksByBlocker(ctx context.Context, blockerTaskID int64, resolvedBy int64) ([]TaskBlock, error)

	ListEntityLinks(ctx context.Context, sourceType, sourceID string) ([]Link, error)
	AddEntityLink(ctx context.Context, link *Link) (int64, error)
	DeleteEntityLink(ctx context.Context, linkID int64) error

	ListTaskTags(ctx context.Context) ([]string, error)
	ListTaskTagsForTasks(ctx context.Context, taskIDs []int64) (map[int64][]string, error)
	SetTaskTags(ctx context.Context, taskID int64, tags []string) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Store() Store {
	return s.store
}
