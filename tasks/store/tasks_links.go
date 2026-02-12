package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"berkut-scc/tasks"
)

func (s *SQLiteStore) CreateTaskWithLinks(ctx context.Context, task *tasks.Task, assignments []int64, links []tasks.Link) (int64, error) {
	var taskID int64
	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		id, err := s.createTaskTx(ctx, tx, task, assignments)
		if err != nil {
			return err
		}
		taskID = id
		if len(links) > 0 {
			now := time.Now().UTC()
			for i := range links {
				links[i].SourceType = "task"
				links[i].SourceID = fmt.Sprintf("%d", taskID)
				if _, err := addEntityLinkTx(ctx, tx, &links[i], now); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return taskID, nil
}
