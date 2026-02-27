package store

import (
	"context"
	"errors"
	"time"
)

func (s *monitoringStore) ListMonitorAssets(ctx context.Context, monitorID int64) ([]AssetLite, error) {
	if monitorID <= 0 {
		return nil, errors.New("bad id")
	}
	query := `
		SELECT a.id, a.name, a.type
		FROM monitor_assets ma
		JOIN assets a ON a.id=ma.asset_id
		WHERE ma.monitor_id=? AND a.deleted_at IS NULL
		ORDER BY LOWER(a.name) ASC, a.id ASC`
	rows, err := s.db.QueryContext(ctx, query, monitorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AssetLite
	for rows.Next() {
		var item AssetLite
		if err := rows.Scan(&item.ID, &item.Name, &item.Type); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *monitoringStore) ReplaceMonitorAssets(ctx context.Context, monitorID int64, assetIDs []int64, userID int64) error {
	if monitorID <= 0 {
		return errors.New("bad id")
	}
	ids := normalizeUniqueInt64(assetIDs)
	if len(ids) > 500 {
		return errors.New("too many items")
	}
	if len(ids) > 0 {
		exists, err := s.assetsExist(ctx, ids)
		if err != nil {
			return err
		}
		if len(exists) != len(ids) {
			return errors.New("monitoring.assets.assetNotFound")
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `DELETE FROM monitor_assets WHERE monitor_id=?`, monitorID); err != nil {
		return err
	}
	if len(ids) == 0 {
		return tx.Commit()
	}
	now := time.Now().UTC()
	for _, assetID := range ids {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO monitor_assets(monitor_id, asset_id, created_by, created_at)
			VALUES(?,?,?,?)
		`, monitorID, assetID, nullableUserID(userID), now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *monitoringStore) assetsExist(ctx context.Context, ids []int64) (map[int64]struct{}, error) {
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	query := `
		SELECT id
		FROM assets
		WHERE deleted_at IS NULL
			AND id IN (` + placeholders(len(ids)) + `)`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]struct{}{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = struct{}{}
	}
	return out, rows.Err()
}

func normalizeUniqueInt64(ids []int64) []int64 {
	seen := map[int64]struct{}{}
	var out []int64
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func nullableUserID(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}
