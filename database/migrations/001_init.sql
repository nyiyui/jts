-- +goose Up
CREATE TABLE sessions (
  id INTEGER PRIMARY KEY,
  description TEXT NOT NULL
);

CREATE TABLE time_frames (
  id INTEGER PRIMARY KEY,
  session_id INTEGER NOT NULL,
  start_time DATETIME NOT NULL, -- in Unix time
  end_time DATETIME NOT NULL, -- in Unix time
  FOREIGN KEY(session_id) REFERENCES sessions(id)
);

-- +goose Down
DROP TABLE time_frames;
DROP TABLE sessions;
