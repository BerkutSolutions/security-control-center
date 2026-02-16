package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

func (s *SQLStore) ListTaskTags(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name FROM task_tags ORDER BY name ASC`)
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

func (s *SQLStore) ListTaskTagsForTasks(ctx context.Context, taskIDs []int64) (map[int64][]string, error) {
	result := map[int64][]string{}
	if len(taskIDs) == 0 {
		return result, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT tl.task_id, tt.name
		FROM task_tag_links tl
		JOIN task_tags tt ON tt.id=tl.tag_id
		WHERE tl.task_id IN (`+placeholders(len(taskIDs))+`)
		ORDER BY tt.name ASC`, toAny(taskIDs)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var taskID int64
		var name string
		if err := rows.Scan(&taskID, &name); err != nil {
			return nil, err
		}
		result[taskID] = append(result[taskID], name)
	}
	return result, rows.Err()
}

func (s *SQLStore) SetTaskTags(ctx context.Context, taskID int64, tags []string) error {
	clean := []string{}
	seen := map[string]struct{}{}
	for _, tag := range tags {
		name := strings.TrimSpace(tag)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		clean = append(clean, name)
	}
	return withTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM task_tag_links WHERE task_id=?`, taskID); err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, name := range clean {
			var tagID int64
			row := tx.QueryRowContext(ctx, `SELECT id FROM task_tags WHERE lower(name)=lower(?)`, name)
			if err := row.Scan(&tagID); err != nil {
				if err == sql.ErrNoRows {
					res, err := tx.ExecContext(ctx, `INSERT INTO task_tags(name, created_at) VALUES(?,?)`, name, now)
					if err != nil {
						return err
					}
					tagID, _ = res.LastInsertId()
				} else {
					return err
				}
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO task_tag_links(task_id, tag_id, created_at)
				VALUES(?,?,?)`, taskID, tagID, now); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM task_tags
			WHERE id NOT IN (
				SELECT DISTINCT tl.tag_id
				FROM task_tag_links tl
				JOIN tasks t ON t.id=tl.task_id
				JOIN task_boards b ON b.id=t.board_id
				WHERE t.is_archived=0 AND b.is_active=1
			)`); err != nil {
			return err
		}
		return nil
	})
}

