package database

import (
	"embed"
	"fmt"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/kirsle/configdir"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
	"nyiyui.ca/jts/data"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Database struct {
	DB *sqlx.DB
}

func NewDatabase() (*Database, error) {
	path := configdir.LocalConfig("jts")
	if err := configdir.MakePath(path); err != nil {
		return nil, fmt.Errorf("failed to create config dir: %w", err)
	}
	dbPath := filepath.Join(path, "jts.db")

	db, err := sqlx.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	return &Database{DB: db}, nil
}

func (d *Database) Migrate() error {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		panic(err)
	}
	if err := goose.Up(d.DB.DB, "migrations"); err != nil {
		return err
	}
	return nil
}

func (d *Database) GetLatestSessions(limit, offset int) ([]data.Session, error) {
	var sessions []data.Session
	err := d.DB.Select(&sessions, "SELECT id, description, notes FROM sessions ORDER BY id DESC LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get sessions: %w", err)
	}
	for i := range sessions {
		var timeframes []data.Timeframe
		err = d.DB.Select(&timeframes, "SELECT id, session_id, start_time, end_time FROM time_frames WHERE session_id = ?", sessions[i].ID)
		if err != nil {
			return nil, fmt.Errorf("get timeframes for session %s: %w", sessions[i].ID, err)
		}
		sessions[i].Timeframes = timeframes
	}
	return sessions, nil
}

func (d *Database) GetSession(id string) (data.Session, error) {
	var session data.Session
	err := d.DB.Get(&session, "SELECT id, description, notes FROM sessions WHERE id = ?", id)
	if err != nil {
		return data.Session{}, err
	}
	var timeframes []data.Timeframe
	err = d.DB.Select(&timeframes, "SELECT id, session_id, start_time, end_time FROM time_frames WHERE session_id = ?", id)
	if err != nil {
		return data.Session{}, err
	}
	session.Timeframes = timeframes
	return session, nil
}

func (d *Database) AddSession(session data.Session) (string, error) {
	tx := d.DB.MustBegin()
	res, err := tx.Exec("INSERT INTO sessions (description) VALUES (?)", session.Description)
	if err != nil {
		return "", err
	}
	rowid, err := res.LastInsertId()
	if err != nil {
		return "", err
	}
	var id string
	err = tx.Get(&id, "SELECT id FROM sessions WHERE rowid = ?", rowid)
	if err != nil {
		return "", err
	}
	for _, tf := range session.Timeframes {
		_, err := tx.Exec("INSERT INTO time_frames (session_id, start_time, end_time) VALUES (?, ?, ?)", id, tf.Start, tf.End)
		if err != nil {
			return "", err
		}
	}
	return id, tx.Commit()
}

func (d *Database) AddTimeframe(sessionID string, tf data.Timeframe) error {
	tx := d.DB.MustBegin()
	_, err := tx.Exec("INSERT INTO time_frames (session_id, start_time, end_time) VALUES (?, ?, ?)", sessionID, tf.Start, tf.End)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (d *Database) ExtendSession(sessionID string, extendTo time.Time) error {
	tx := d.DB.MustBegin()
	_, err := tx.Exec("UPDATE time_frames SET end_time = ? WHERE session_id = ? AND end_time = (SELECT MAX(end_time) FROM time_frames WHERE session_id = ?)", extendTo, sessionID, sessionID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (d *Database) EditSessionProperties(session data.Session) error {
	tx := d.DB.MustBegin()
	_, err := tx.Exec("UPDATE sessions SET description = ?, notes = ? WHERE id = ?", session.Description, session.Notes, session.ID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (d *Database) DeleteSession(id string) error {
	tx := d.DB.MustBegin()
	_, err := tx.Exec("DELETE FROM sessions WHERE id = ?", id)
	if err != nil {
		return err
	}
	return tx.Commit()
}
