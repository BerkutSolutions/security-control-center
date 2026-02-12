package tasks

import (
	"encoding/json"
	"time"
)

const (
	PriorityLow      = "low"
	PriorityMedium   = "medium"
	PriorityHigh     = "high"
	PriorityCritical = "critical"
)

type Board struct {
	ID             int64     `json:"id"`
	SpaceID        int64     `json:"space_id"`
	OrganizationID string    `json:"organization_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Position       int       `json:"position"`
	DefaultTemplateID *int64 `json:"default_template_id,omitempty"`
	CreatedBy      *int64    `json:"created_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	IsActive       bool      `json:"is_active"`
}

type Space struct {
	ID             int64     `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Layout         string    `json:"layout"`
	CreatedBy      *int64    `json:"created_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	IsActive       bool      `json:"is_active"`
}

type Column struct {
	ID        int64     `json:"id"`
	BoardID   int64     `json:"board_id"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	IsFinal   bool      `json:"is_final"`
	WIPLimit  *int      `json:"wip_limit,omitempty"`
	DefaultTemplateID *int64 `json:"default_template_id,omitempty"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SubColumn struct {
	ID        int64     `json:"id"`
	ColumnID  int64     `json:"column_id"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Task struct {
	ID          int64      `json:"id"`
	BoardID     int64      `json:"board_id"`
	ColumnID    int64      `json:"column_id"`
	SubColumnID *int64     `json:"subcolumn_id,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Result      string     `json:"result"`
	ExternalLink string    `json:"external_link"`
	BusinessCustomer string `json:"business_customer"`
	SizeEstimate *int      `json:"size_estimate,omitempty"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	TemplateID  *int64     `json:"template_id,omitempty"`
	RecurringRuleID *int64 `json:"recurring_rule_id,omitempty"`
	Checklist   []TaskChecklistItem `json:"checklist,omitempty"`
	CreatedBy   *int64     `json:"created_by,omitempty"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
	IsArchived  bool       `json:"is_archived"`
	Position    int        `json:"position"`
}

type TaskArchiveEntry struct {
	Task
	ArchivedAt       time.Time  `json:"archived_at"`
	ArchivedBy       *int64     `json:"archived_by,omitempty"`
	ArchivedBoardID  int64      `json:"archived_board_id"`
	ArchivedColumnID int64      `json:"archived_column_id"`
	ArchivedSubColumnID *int64  `json:"archived_subcolumn_id,omitempty"`
	OriginalPosition int        `json:"original_position"`
	RestoredAt       *time.Time `json:"restored_at,omitempty"`
}

type TaskChecklistItem struct {
	Text   string     `json:"text"`
	Done   bool       `json:"done"`
	DoneBy *int64     `json:"done_by,omitempty"`
	DoneAt *time.Time `json:"done_at,omitempty"`
}

type TaskTemplateLink struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
}

type TaskTemplate struct {
	ID                  int64              `json:"id"`
	BoardID             int64              `json:"board_id"`
	ColumnID            int64              `json:"column_id"`
	TitleTemplate       string             `json:"title_template"`
	DescriptionTemplate string             `json:"description_template"`
	Priority            string             `json:"priority"`
	DefaultAssignees    []int64            `json:"default_assignees,omitempty"`
	DefaultDueDays      int                `json:"default_due_days"`
	ChecklistTemplate   []TaskChecklistItem `json:"checklist_template,omitempty"`
	LinksTemplate       []TaskTemplateLink `json:"links_template,omitempty"`
	IsActive            bool               `json:"is_active"`
	CreatedBy           *int64             `json:"created_by,omitempty"`
	CreatedAt           time.Time          `json:"created_at"`
	UpdatedAt           time.Time          `json:"updated_at"`
}

type TaskTemplateFilter struct {
	BoardID        int64
	IncludeInactive bool
}

type TaskRecurringRule struct {
	ID            int64           `json:"id"`
	TemplateID    int64           `json:"template_id"`
	ScheduleType  string          `json:"schedule_type"`
	ScheduleConfig json.RawMessage `json:"schedule_config,omitempty"`
	TimeOfDay     string          `json:"time_of_day"`
	NextRunAt     *time.Time      `json:"next_run_at,omitempty"`
	LastRunAt     *time.Time      `json:"last_run_at,omitempty"`
	IsActive      bool            `json:"is_active"`
	CreatedBy     *int64          `json:"created_by,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type TaskRecurringRuleFilter struct {
	TemplateID int64
	IncludeInactive bool
}

type TaskRecurringInstance struct {
	ID           int64      `json:"id"`
	RuleID       int64      `json:"rule_id"`
	TemplateID   int64      `json:"template_id"`
	TaskID       *int64     `json:"task_id,omitempty"`
	ScheduledFor time.Time  `json:"scheduled_for"`
	CreatedAt    time.Time  `json:"created_at"`
}

type TaskBlock struct {
	ID            int64      `json:"id"`
	TaskID        int64      `json:"task_id"`
	BlockType     string     `json:"block_type"`
	Reason        *string    `json:"reason,omitempty"`
	BlockerTaskID *int64     `json:"blocker_task_id,omitempty"`
	CreatedBy     *int64     `json:"created_by,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	ResolvedBy    *int64     `json:"resolved_by,omitempty"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
	IsActive      bool       `json:"is_active"`
}

type TaskBlockInfo struct {
	BlockType     string  `json:"block_type"`
	Reason        *string `json:"reason,omitempty"`
	BlockerTaskID *int64  `json:"blocker_task_id,omitempty"`
}

type Assignment struct {
	TaskID     int64     `json:"task_id"`
	UserID     int64     `json:"user_id"`
	AssignedAt time.Time `json:"assigned_at"`
	AssignedBy *int64    `json:"assigned_by,omitempty"`
}

type Comment struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	AuthorID  int64     `json:"author_id"`
	Content   string    `json:"content"`
	Attachments []CommentAttachment `json:"attachments,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type CommentAttachment struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
	Path        string `json:"path,omitempty"`
	URL         string `json:"url,omitempty"`
}

type TaskFile struct {
	ID          int64     `json:"id"`
	TaskID      int64     `json:"task_id"`
	Name        string    `json:"name"`
	StoredName  string    `json:"-"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	UploadedBy  *int64    `json:"uploaded_by,omitempty"`
	UploadedAt  time.Time `json:"uploaded_at"`
	Path        string    `json:"path,omitempty"`
	URL         string    `json:"url,omitempty"`
}

type Link struct {
	ID         int64     `json:"id"`
	SourceType string    `json:"source_type"`
	SourceID   string    `json:"source_id"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type BoardFilter struct {
	SpaceID         int64
	IncludeInactive bool
}

type TaskFilter struct {
	SpaceID         int64
	BoardID         int64
	ColumnID        int64
	SubColumnID     int64
	AssignedUserID  int64
	MineUserID      int64
	Status          string
	IncludeArchived bool
	Search          string
	Limit           int
	Offset          int
}

type TaskDTO struct {
	Task
	AssignedTo     []int64        `json:"assigned_to,omitempty"`
	IsBlocked      bool           `json:"is_blocked"`
	Blocks         []TaskBlockInfo `json:"blocks,omitempty"`
	BlockedByTasks []int64        `json:"blocked_by_tasks,omitempty"`
	Tags           []string       `json:"tags,omitempty"`
}

type ACLRule struct {
	SubjectType string `json:"subject_type"`
	SubjectID   string `json:"subject_id"`
	Permission  string `json:"permission"`
}

type SpaceFilter struct {
	IncludeInactive bool
}
