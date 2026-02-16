package store

import "time"

type User struct {
	ID                   int64      `json:"id"`
	Username             string     `json:"username"`
	FullName             string     `json:"full_name"`
	Email                string     `json:"email"`
	Department           string     `json:"department"`
	Position             string     `json:"position"`
	ClearanceLevel       int        `json:"clearance_level"`
	ClearanceTags        []string   `json:"clearance_tags"`
	PasswordHash         string     `json:"-"`
	Salt                 string     `json:"-"`
	PasswordSet          bool       `json:"password_set"`
	RequirePasswordChange bool      `json:"require_password_change"`
	Active               bool       `json:"active"`
	DisabledAt           *time.Time `json:"disabled_at,omitempty"`
	LockedUntil          *time.Time `json:"locked_until,omitempty"`
	LockReason           string     `json:"lock_reason,omitempty"`
	LockStage            int        `json:"lock_stage"`
	FailedAttempts       int        `json:"failed_attempts"`
	TOTPEnabled          bool       `json:"totp_enabled"`
	TOTPSecret           string     `json:"-"`
	LastLoginAt          *time.Time `json:"last_login_at,omitempty"`
	LastFailedAt         *time.Time `json:"last_failed_at,omitempty"`
	PasswordChangedAt    *time.Time `json:"password_changed_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type UserWithRoles struct {
	User
	Roles []string `json:"roles"`
	Groups []Group `json:"groups"`
	EffectiveRoles []string `json:"effective_roles,omitempty"`
	EffectivePermissions []string `json:"effective_permissions,omitempty"`
	EffectiveClearanceLevel int `json:"effective_clearance_level,omitempty"`
	EffectiveClearanceTags []string `json:"effective_clearance_tags,omitempty"`
	EffectiveMenuPermissions []string `json:"effective_menu_permissions,omitempty"`
}

type SessionRecord struct {
	ID        string
	UserID    int64
	Username  string
	Roles     []string
	IP        string
	UserAgent string
	CSRFToken string
	CreatedAt time.Time
	LastSeenAt time.Time
	ExpiresAt time.Time
	Revoked    bool
	RevokedAt  *time.Time
	RevokedBy  string
}

type Group struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	ClearanceLevel   int       `json:"clearance_level"`
	ClearanceTags    []string  `json:"clearance_tags"`
	MenuPermissions  []string  `json:"menu_permissions"`
	IsSystem         bool      `json:"is_system"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Roles            []string  `json:"roles"`
	UserCount        int       `json:"user_count"`
}

type EffectiveAccess struct {
	Roles             []string `json:"effective_roles,omitempty"`
	Permissions       []string `json:"effective_permissions,omitempty"`
	ClearanceLevel    int      `json:"effective_clearance_level,omitempty"`
	ClearanceTags     []string `json:"effective_clearance_tags,omitempty"`
	MenuPermissions   []string `json:"effective_menu_permissions,omitempty"`
}

type Role struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Permissions []string          `json:"permissions"`
	BuiltIn     bool              `json:"built_in"`
	Template    bool              `json:"template"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}
