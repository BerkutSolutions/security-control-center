-- +goose Up
ALTER TABLE monitors ADD COLUMN IF NOT EXISTS auto_task_on_down INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE monitors DROP COLUMN IF EXISTS auto_task_on_down;
