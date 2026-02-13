package store

import (
	"context"
	"database/sql"
	"time"
)

type SQLStore struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

func (s *SQLStore) DB() *sql.DB {
	return s.db
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullableID(id *int64) any {
	if id == nil {
		return nil
	}
	return *id
}

func nullableTime(ts *time.Time) any {
	if ts == nil {
		return nil
	}
	return *ts
}

func nullableInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func toAny(items []int64) []any {
	out := make([]any, 0, len(items))
	for _, v := range items {
		out = append(out, v)
	}
	return out
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, 0, n*2)
	for i := 0; i < n; i++ {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, '?')
	}
	return string(out)
}

func withTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

