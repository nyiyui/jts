-- +goose Up
ALTER TABLE sessions DROP COLUMN done;
ALTER TABLE time_frames ADD COLUMN done BOOLEAN DEFAULT FALSE;

-- +goose Down
ALTER TABLE time_frames DROP COLUMN done;
ALTER TABLE sessions ADD COLUMN done BOOLEAN DEFAULT FALSE;
