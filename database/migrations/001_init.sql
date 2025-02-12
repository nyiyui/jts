-- +goose Up
CREATE TABLE sessions (
  rowid INTEGER PRIMARY KEY,
  id TEXT UNIQUE DEFAULT (lower(hex(randomblob(16)))),
  description TEXT NOT NULL
);

CREATE TABLE time_frames (
  rowid INTEGER PRIMARY KEY,
  id TEXT UNIQUE DEFAULT (lower(hex(randomblob(16)))),
  session_id TEXT NOT NULL,
  start_time DATETIME NOT NULL, -- in Unix time
  end_time DATETIME NOT NULL, -- in Unix time
  FOREIGN KEY(session_id) REFERENCES sessions(id)
);

-- +goose Down
DROP TABLE time_frames;
DROP TABLE sessions;
