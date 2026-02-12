package auth

import "time"

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Session struct {
	ID         string
	UserID     int64
	Username   string
	Roles      []string
	IP         string
	UserAgent  string
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
	CSRFToken  string
	Revoked    bool
	RevokedAt  *time.Time
}

type LoginResult struct {
	Session *Session `json:"session"`
	User    *UserDTO `json:"user"`
}

type UserDTO struct {
	ID                    int64      `json:"id"`
	Username              string     `json:"username"`
	Roles                 []string   `json:"roles"`
	Active                bool       `json:"active"`
	PasswordSet           bool       `json:"password_set"`
	RequirePasswordChange bool       `json:"require_password_change"`
	PasswordChangedAt     *time.Time `json:"password_changed_at,omitempty"`
	Permissions           []string   `json:"permissions,omitempty"`
	MenuPermissions       []string   `json:"menu_permissions,omitempty"`
}
