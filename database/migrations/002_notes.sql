-- +goose Up
ALTER TABLE sessions ADD COLUMN notes TEXT DEFAULT "";

-- +goose Down
ALTER TABLE sessions DROP COLUMN notes;
