package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

type UsersStore interface {
	FindByUsername(ctx context.Context, username string) (*User, []string, error)
	Get(ctx context.Context, userID int64) (*User, []string, error)
	Create(ctx context.Context, user *User, roles []string) (int64, error)
	List(ctx context.Context) ([]UserWithRoles, error)
	ListFiltered(ctx context.Context, f UserFilter) ([]UserWithRoles, error)
	UserGroups(ctx context.Context, userID int64) ([]Group, error)
	UserDirectRoles(ctx context.Context, userID int64) ([]string, error)
	Update(ctx context.Context, user *User, roles []string) error
	SetActive(ctx context.Context, userID int64, active bool) error
	UpdatePassword(ctx context.Context, userID int64, hash, salt string, requireChange bool) error
	PasswordHistory(ctx context.Context, userID int64, limit int) ([]PasswordHistoryEntry, error)
	Delete(ctx context.Context, userID int64) error
}

type usersStore struct {
	db *sql.DB
}

func NewUsersStore(db *sql.DB) UsersStore {
	return &usersStore{db: db}
}

type UserFilter struct {
	Department string
	GroupID    int64
	Role       string
	Status     string
	HasPassword *bool
	PasswordStatus string
	ClearanceMin  int
	ClearanceMax  int
	Query         string
}

type PasswordHistoryEntry struct {
	Hash      string
	Salt      string
	CreatedAt time.Time
}

func (s *usersStore) FindByUsername(ctx context.Context, username string) (*User, []string, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, username, email, full_name, department, position, clearance_level, clearance_tags, password_hash, salt, password_set, require_password_change, active, disabled_at, locked_until, lock_reason, lock_stage, failed_attempts, totp_secret, totp_enabled, last_login_at, last_failed_at, password_changed_at, created_at, updated_at
		FROM users WHERE username=?`, username)
	return s.scanUser(ctx, row)
}

func (s *usersStore) Get(ctx context.Context, userID int64) (*User, []string, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, username, email, full_name, department, position, clearance_level, clearance_tags, password_hash, salt, password_set, require_password_change, active, disabled_at, locked_until, lock_reason, lock_stage, failed_attempts, totp_secret, totp_enabled, last_login_at, last_failed_at, password_changed_at, created_at, updated_at
		FROM users WHERE id=?`, userID)
	return s.scanUser(ctx, row)
}

func (s *usersStore) scanUser(ctx context.Context, row *sql.Row) (*User, []string, error) {
	u := User{}
	var disabled sql.NullTime
	var locked sql.NullTime
	var lastLogin sql.NullTime
	var lastFailed sql.NullTime
	var lastChanged sql.NullTime
	var tagsRaw string
	if err := row.Scan(
		&u.ID, &u.Username, &u.Email, &u.FullName, &u.Department, &u.Position, &u.ClearanceLevel, &tagsRaw,
		&u.PasswordHash, &u.Salt, &u.PasswordSet, &u.RequirePasswordChange, &u.Active, &disabled,
		&locked, &u.LockReason, &u.LockStage, &u.FailedAttempts, &u.TOTPSecret, &u.TOTPEnabled, &lastLogin, &lastFailed, &lastChanged, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	if tagsRaw != "" {
		_ = json.Unmarshal([]byte(tagsRaw), &u.ClearanceTags)
	}
	if disabled.Valid {
		u.DisabledAt = &disabled.Time
	}
	if locked.Valid {
		u.LockedUntil = &locked.Time
	}
	if lastLogin.Valid {
		u.LastLoginAt = &lastLogin.Time
	}
	if lastFailed.Valid {
		u.LastFailedAt = &lastFailed.Time
	}
	if lastChanged.Valid {
		u.PasswordChangedAt = &lastChanged.Time
	}
	roles, err := s.rolesForUser(ctx, u.ID)
	return &u, roles, err
}

func (s *usersStore) Create(ctx context.Context, user *User, roles []string) (int64, error) {
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO users(username, email, full_name, department, position, clearance_level, clearance_tags, password_hash, salt, password_set, require_password_change, active, disabled_at, locked_until, lock_reason, lock_stage, failed_attempts, totp_secret, totp_enabled, last_login_at, last_failed_at, password_changed_at, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		user.Username, user.Email, user.FullName, user.Department, user.Position, user.ClearanceLevel, tagsToJSON(user.ClearanceTags), user.PasswordHash, user.Salt, boolToInt(user.PasswordSet), boolToInt(user.RequirePasswordChange), boolToInt(user.Active), nullTime(user.DisabledAt), nullTime(user.LockedUntil), user.LockReason, user.LockStage, user.FailedAttempts, user.TOTPSecret, boolToInt(user.TOTPEnabled), nullTime(user.LastLoginAt), nullTime(user.LastFailedAt), nullTime(user.PasswordChangedAt), now, now)
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	userID, err := res.LastInsertId()
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	if err := s.assignRolesTx(ctx, tx, userID, roles); err != nil {
		tx.Rollback()
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return userID, nil
}

func (s *usersStore) List(ctx context.Context) ([]UserWithRoles, error) {
	return s.ListFiltered(ctx, UserFilter{})
}

func (s *usersStore) ListFiltered(ctx context.Context, f UserFilter) ([]UserWithRoles, error) {
	where := []string{}
	args := []any{}
	now := time.Now().UTC()

	if strings.TrimSpace(f.Department) != "" {
		where = append(where, "department=?")
		args = append(args, strings.TrimSpace(f.Department))
	}
	if f.GroupID > 0 {
		where = append(where, "id IN (SELECT user_id FROM user_groups WHERE group_id=?)")
		args = append(args, f.GroupID)
	}
	if strings.TrimSpace(f.Role) != "" {
		role := strings.ToLower(strings.TrimSpace(f.Role))
		where = append(where, `id IN (
			SELECT user_id FROM user_roles ur JOIN roles r ON ur.role_id=r.id WHERE r.name=?
			UNION
			SELECT ug.user_id FROM user_groups ug JOIN group_roles gr ON ug.group_id=gr.group_id JOIN roles r ON gr.role_id=r.id WHERE r.name=?
		)`)
		args = append(args, role, role)
	}
	if f.HasPassword != nil {
		where = append(where, "password_set=?")
		args = append(args, boolToInt(*f.HasPassword))
	}
	switch strings.ToLower(strings.TrimSpace(f.PasswordStatus)) {
	case "has_password":
		val := true
		where = append(where, "password_set=?")
		args = append(args, boolToInt(val))
	case "no_password":
		val := false
		where = append(where, "password_set=?")
		args = append(args, boolToInt(val))
	}
	if f.ClearanceMin > 0 {
		where = append(where, "clearance_level>=?")
		args = append(args, f.ClearanceMin)
	}
	if f.ClearanceMax > 0 {
		where = append(where, "clearance_level<=?")
		args = append(args, f.ClearanceMax)
	}
	if strings.TrimSpace(f.Status) != "" {
		switch strings.ToLower(strings.TrimSpace(f.Status)) {
		case "active":
			where = append(where, "(active=1 AND lock_stage<6 AND (locked_until IS NULL OR locked_until<=?))")
			args = append(args, now)
		case "disabled":
			where = append(where, "active=0")
		case "blocked", "locked":
			where = append(where, "( (locked_until IS NOT NULL AND locked_until> ?) OR lock_stage>=6 )")
			args = append(args, now)
		}
	}
	if strings.TrimSpace(f.Query) != "" {
		pattern := "%" + strings.ToLower(strings.TrimSpace(f.Query)) + "%"
		where = append(where, "(LOWER(username) LIKE ? OR LOWER(full_name) LIKE ? OR LOWER(email) LIKE ?)")
		args = append(args, pattern, pattern, pattern)
	}

	query := `
		SELECT id, username, email, full_name, department, position, clearance_level, clearance_tags, password_hash, salt, password_set, require_password_change, active, disabled_at, locked_until, lock_reason, lock_stage, failed_attempts, totp_secret, totp_enabled, last_login_at, last_failed_at, password_changed_at, created_at, updated_at
		FROM users`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY username"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	var users []UserWithRoles
	for rows.Next() {
		u := User{}
		var tagsRaw string
		var disabled, locked, lastLogin, lastFailed sql.NullTime
		if err := rows.Scan(
			&u.ID, &u.Username, &u.Email, &u.FullName, &u.Department, &u.Position, &u.ClearanceLevel, &tagsRaw,
			&u.PasswordHash, &u.Salt, &u.PasswordSet, &u.RequirePasswordChange, &u.Active, &disabled,
			&locked, &u.LockReason, &u.LockStage, &u.FailedAttempts, &u.TOTPSecret, &u.TOTPEnabled, &lastLogin, &lastFailed, &u.PasswordChangedAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		if tagsRaw != "" {
			_ = json.Unmarshal([]byte(tagsRaw), &u.ClearanceTags)
		}
		if disabled.Valid {
			u.DisabledAt = &disabled.Time
		}
		if locked.Valid {
			u.LockedUntil = &locked.Time
		}
		if lastLogin.Valid {
			u.LastLoginAt = &lastLogin.Time
		}
		if lastFailed.Valid {
			u.LastFailedAt = &lastFailed.Time
		}
		users = append(users, UserWithRoles{User: u})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range users {
		roles, err := s.rolesForUser(ctx, users[i].ID)
		if err != nil {
			return nil, err
		}
		groups, err := s.groupsForUser(ctx, users[i].ID)
		if err != nil {
			return nil, err
		}
		users[i].Roles = roles
		users[i].Groups = groups
	}
	return users, nil
}

func (s *usersStore) Update(ctx context.Context, user *User, roles []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE users
		SET email=?, full_name=?, department=?, position=?, clearance_level=?, clearance_tags=?, require_password_change=?, active=?, disabled_at=?, locked_until=?, lock_reason=?, lock_stage=?, failed_attempts=?, totp_secret=?, totp_enabled=?, last_login_at=?, last_failed_at=?, password_changed_at=?, updated_at=?
		WHERE id=?`,
		user.Email, user.FullName, user.Department, user.Position, user.ClearanceLevel, tagsToJSON(user.ClearanceTags), boolToInt(user.RequirePasswordChange), boolToInt(user.Active), nullTime(user.DisabledAt), nullTime(user.LockedUntil), user.LockReason, user.LockStage, user.FailedAttempts, user.TOTPSecret, boolToInt(user.TOTPEnabled), nullTime(user.LastLoginAt), nullTime(user.LastFailedAt), nullTime(user.PasswordChangedAt), time.Now().UTC(), user.ID)
	if err != nil {
		tx.Rollback()
		return err
	}
	if roles != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=?`, user.ID); err != nil {
			tx.Rollback()
			return err
		}
		if err := s.assignRolesTx(ctx, tx, user.ID, roles); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *usersStore) SetActive(ctx context.Context, userID int64, active bool) error {
	var disabled any
	if active {
		disabled = nil
	} else {
		disabled = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `UPDATE users SET active=?, disabled_at=?, updated_at=? WHERE id=?`, boolToInt(active), disabled, time.Now().UTC(), userID)
	return err
}

func (s *usersStore) UpdatePassword(ctx context.Context, userID int64, hash, salt string, requireChange bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash=?, salt=?, password_set=1, require_password_change=?, failed_attempts=0, lock_stage=0, locked_until=NULL, lock_reason='', password_changed_at=?, updated_at=? WHERE id=?`, hash, salt, boolToInt(requireChange), now, now, userID); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO password_history(user_id, password_hash, salt, created_at) VALUES(?,?,?,?)`, userID, hash, salt, now); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM password_history WHERE id NOT IN (SELECT id FROM password_history WHERE user_id=? ORDER BY created_at DESC LIMIT 10) AND user_id=?`, userID, userID); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *usersStore) Delete(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id=?`, userID)
	return err
}

func (s *usersStore) PasswordHistory(ctx context.Context, userID int64, limit int) ([]PasswordHistoryEntry, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.QueryContext(ctx, `SELECT password_hash, salt, created_at FROM password_history WHERE user_id=? ORDER BY created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []PasswordHistoryEntry
	for rows.Next() {
		var entry PasswordHistoryEntry
		if err := rows.Scan(&entry.Hash, &entry.Salt, &entry.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, entry)
	}
	return res, rows.Err()
}

func (s *usersStore) assignRolesTx(ctx context.Context, tx *sql.Tx, userID int64, roles []string) error {
	for _, r := range roles {
		roleID, err := ensureRoleTx(ctx, tx, r)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO user_roles(user_id, role_id) VALUES(?,?)`, userID, roleID); err != nil {
			return err
		}
	}
	return nil
}

func (s *usersStore) rolesForUser(ctx context.Context, userID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT r.name FROM roles r
		INNER JOIN (
			SELECT role_id FROM user_roles WHERE user_id=?
			UNION
			SELECT gr.role_id FROM group_roles gr INNER JOIN user_groups ug ON ug.group_id=gr.group_id WHERE ug.user_id=?
		) src ON src.role_id = r.id`, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		res = append(res, name)
	}
	return res, rows.Err()
}

func (s *usersStore) groupsForUser(ctx context.Context, userID int64) ([]Group, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.id, g.name, g.description, g.clearance_level, g.clearance_tags, g.menu_permissions, g.is_system, g.created_at, g.updated_at
		FROM groups g INNER JOIN user_groups ug ON ug.group_id=g.id WHERE ug.user_id=?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Group
	for rows.Next() {
		var g Group
		var tagsRaw, permsRaw string
		var isSystem int
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.ClearanceLevel, &tagsRaw, &permsRaw, &isSystem, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		if tagsRaw != "" {
			_ = json.Unmarshal([]byte(tagsRaw), &g.ClearanceTags)
		}
		if permsRaw != "" {
			_ = json.Unmarshal([]byte(permsRaw), &g.MenuPermissions)
		}
		g.IsSystem = isSystem == 1
		roles, err := s.groupRoles(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		g.Roles = roles
		res = append(res, g)
	}
	return res, rows.Err()
}

func (s *usersStore) UserGroups(ctx context.Context, userID int64) ([]Group, error) {
	return s.groupsForUser(ctx, userID)
}

func (s *usersStore) groupRoles(ctx context.Context, groupID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT r.name FROM roles r INNER JOIN group_roles gr ON gr.role_id=r.id WHERE gr.group_id=?`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		res = append(res, name)
	}
	return res, rows.Err()
}

func (s *usersStore) UserDirectRoles(ctx context.Context, userID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT r.name FROM roles r INNER JOIN user_roles ur ON ur.role_id=r.id WHERE ur.user_id=?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		res = append(res, name)
	}
	return res, rows.Err()
}

func ensureRoleTx(ctx context.Context, tx *sql.Tx, role string) (int64, error) {
	role = strings.ToLower(role)
	var id int64
	row := tx.QueryRowContext(ctx, `SELECT id FROM roles WHERE name=?`, role)
	if err := row.Scan(&id); err == nil {
		return id, nil
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO roles(name) VALUES(?)`, role)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

func tagsToJSON(tags []string) string {
	if tags == nil {
		tags = []string{}
	}
	b, _ := json.Marshal(tags)
	return string(b)
}
