-- +goose Up
-- Note: goose already runs this in a transaction for us
CREATE TABLE tasks(
  rowid INTEGER PRIMARY KEY,
  id TEXT UNIQUE DEFAULT (lower(hex(randomblob(16)))),
  description TEXT NOT NULL
);
-- add foreign key to sessions
ALTER TABLE sessions RENAME TO sessions_old;
CREATE TABLE sessions (
  rowid INTEGER PRIMARY KEY,
  id TEXT UNIQUE DEFAULT (lower(hex(randomblob(16)))),
  description TEXT NOT NULL,
  notes TEXT DEFAULT "",
  task_id TEXT DEFAULT NULL, -- new column
  done BOOLEAN DEFAULT FALSE, -- new column
  FOREIGN KEY(task_id) REFERENCES tasks(id) -- new foreign key constraint
);
INSERT INTO sessions (id, description, notes) SELECT id, description, notes FROM sessions_old;
DROP TABLE sessions_old;

-- +goose Down
ALTER TABLE sessions DROP COLUMN task_id;
ALTER TABLE sessions DROP COLUMN done;
DROP TABLE tasks;
