package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type GroupsStore interface {
	List(ctx context.Context) ([]Group, error)
	Get(ctx context.Context, id int64) (*Group, []int64, []string, error)
	Create(ctx context.Context, g *Group, roles []string, userIDs []int64) (int64, error)
	Update(ctx context.Context, g *Group, roles []string, userIDs []int64) error
	Delete(ctx context.Context, id int64) error
	SetUserGroups(ctx context.Context, userID int64, groupIDs []int64) error
	AddMember(ctx context.Context, groupID, userID int64) error
	RemoveMember(ctx context.Context, groupID, userID int64) error
	Members(ctx context.Context, groupID int64) ([]int64, error)
}

type groupsStore struct {
	db *sql.DB
}

func NewGroupsStore(db *sql.DB) GroupsStore {
	return &groupsStore{db: db}
}

func (s *groupsStore) List(ctx context.Context) ([]Group, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT g.id, g.name, g.description, g.clearance_level, g.clearance_tags, g.menu_permissions, g.is_system, g.created_at, g.updated_at,
		(SELECT COUNT(1) FROM user_groups ug WHERE ug.group_id=g.id) as user_count
		FROM groups g ORDER BY g.name`)
	if err != nil {
		return nil, err
	}

	var res []Group
	for rows.Next() {
		var g Group
		var tagsRaw, permsRaw string
		var isSystem int
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.ClearanceLevel, &tagsRaw, &permsRaw, &isSystem, &g.CreatedAt, &g.UpdatedAt, &g.UserCount); err != nil {
			return nil, err
		}
		if tagsRaw != "" {
			_ = json.Unmarshal([]byte(tagsRaw), &g.ClearanceTags)
		}
		if permsRaw != "" {
			_ = json.Unmarshal([]byte(permsRaw), &g.MenuPermissions)
		}
		g.IsSystem = isSystem == 1
		res = append(res, g)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range res {
		roles, err := s.rolesForGroup(ctx, res[i].ID)
		if err != nil {
			return nil, err
		}
		res[i].Roles = roles
	}
	return res, nil
}

func (s *groupsStore) Get(ctx context.Context, id int64) (*Group, []int64, []string, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, description, clearance_level, clearance_tags, menu_permissions, is_system, created_at, updated_at FROM groups WHERE id=?`, id)
	var g Group
	var tagsRaw, permsRaw string
	var isSystem int
	if err := row.Scan(&g.ID, &g.Name, &g.Description, &g.ClearanceLevel, &tagsRaw, &permsRaw, &isSystem, &g.CreatedAt, &g.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil, sql.ErrNoRows
		}
		return nil, nil, nil, err
	}
	if tagsRaw != "" {
		_ = json.Unmarshal([]byte(tagsRaw), &g.ClearanceTags)
	}
	if permsRaw != "" {
		_ = json.Unmarshal([]byte(permsRaw), &g.MenuPermissions)
	}
	g.IsSystem = isSystem == 1
	users, err := s.usersForGroup(ctx, id)
	if err != nil {
		return nil, nil, nil, err
	}
	roles, err := s.rolesForGroup(ctx, id)
	if err != nil {
		return nil, nil, nil, err
	}
	g.UserCount = len(users)
	g.Roles = roles
	return &g, users, roles, nil
}

func (s *groupsStore) Create(ctx context.Context, g *Group, roles []string, userIDs []int64) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	res, err := tx.ExecContext(ctx, `INSERT INTO groups(name, description, clearance_level, clearance_tags, menu_permissions, is_system, created_at, updated_at) VALUES(?,?,?,?,?,?,?,?)`,
		g.Name, g.Description, g.ClearanceLevel, tagsToJSON(g.ClearanceTags), permsToJSON(g.MenuPermissions), boolToInt(g.IsSystem), now, now)
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	if err := s.replaceGroupRolesTx(ctx, tx, id, roles); err != nil {
		tx.Rollback()
		return 0, err
	}
	if err := s.replaceGroupUsersTx(ctx, tx, id, userIDs); err != nil {
		tx.Rollback()
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *groupsStore) Update(ctx context.Context, g *Group, roles []string, userIDs []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE groups SET name=?, description=?, clearance_level=?, clearance_tags=?, menu_permissions=?, is_system=?, updated_at=? WHERE id=?`,
		g.Name, g.Description, g.ClearanceLevel, tagsToJSON(g.ClearanceTags), permsToJSON(g.MenuPermissions), boolToInt(g.IsSystem), time.Now().UTC(), g.ID)
	if err != nil {
		tx.Rollback()
		return err
	}
	if roles != nil {
		if err := s.replaceGroupRolesTx(ctx, tx, g.ID, roles); err != nil {
			tx.Rollback()
			return err
		}
	}
	if userIDs != nil {
		if err := s.replaceGroupUsersTx(ctx, tx, g.ID, userIDs); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *groupsStore) Delete(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM groups WHERE id=?`, id)
	return err
}

func (s *groupsStore) SetUserGroups(ctx context.Context, userID int64, groupIDs []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_groups WHERE user_id=?`, userID); err != nil {
		tx.Rollback()
		return err
	}
	for _, gid := range groupIDs {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO user_groups(user_id, group_id) VALUES(?,?)`, userID, gid); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *groupsStore) replaceGroupRolesTx(ctx context.Context, tx *sql.Tx, groupID int64, roles []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM group_roles WHERE group_id=?`, groupID); err != nil {
		return err
	}
	for _, r := range roles {
		roleID, err := ensureRoleTx(ctx, tx, r)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO group_roles(group_id, role_id) VALUES(?,?)`, groupID, roleID); err != nil {
			return err
		}
	}
	return nil
}

func (s *groupsStore) replaceGroupUsersTx(ctx context.Context, tx *sql.Tx, groupID int64, userIDs []int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_groups WHERE group_id=?`, groupID); err != nil {
		return err
	}
	for _, id := range userIDs {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO user_groups(user_id, group_id) VALUES(?,?)`, id, groupID); err != nil {
			return err
		}
	}
	return nil
}

func (s *groupsStore) usersForGroup(ctx context.Context, id int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id FROM user_groups WHERE group_id=?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []int64
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		res = append(res, uid)
	}
	return res, rows.Err()
}

func (s *groupsStore) rolesForGroup(ctx context.Context, id int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT r.name FROM roles r INNER JOIN group_roles gr ON gr.role_id=r.id WHERE gr.group_id=?`, id)
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

func (s *groupsStore) AddMember(ctx context.Context, groupID, userID int64) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO user_groups(user_id, group_id) VALUES(?,?)`, userID, groupID)
	return err
}

func (s *groupsStore) RemoveMember(ctx context.Context, groupID, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM user_groups WHERE user_id=? AND group_id=?`, userID, groupID)
	return err
}

func (s *groupsStore) Members(ctx context.Context, groupID int64) ([]int64, error) {
	return s.usersForGroup(ctx, groupID)
}

func permsToJSON(perms []string) string {
	if perms == nil {
		perms = []string{}
	}
	b, _ := json.Marshal(perms)
	return string(b)
}
