package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"embed"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/kirsle/configdir"
	"github.com/mattn/go-sqlite3"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
	"nyiyui.ca/jts/data"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Database struct {
	DB        *sqlx.DB
	notifyFns []UpdateHookFn
}

type connector struct {
	conns  []*sqlite3.SQLiteConn
	driver *sqlite3.SQLiteDriver
	dsn    string
}

func newConnector(dsn string, hook UpdateHookFn) *connector {
	c := new(connector)
	c.dsn = dsn
	c.driver = &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			c.conns = append(c.conns, conn)
			conn.RegisterUpdateHook(hook)
			return nil
		},
	}
	return c
}

func (c *connector) Connect(ctx context.Context) (driver.Conn, error) {
	return c.driver.Open(c.dsn)
}

func (c *connector) Driver() driver.Driver {
	return c.driver
}

// NewDatabase creates (if necessary) and opens a database.
// If dbPath is the empty string, a default database path is used.
func NewDatabase(dbPath string) (*Database, error) {
	if dbPath == "" {
		path := configdir.LocalConfig("jts")
		if err := configdir.MakePath(path); err != nil {
			return nil, fmt.Errorf("failed to create config dir: %w", err)
		}
		dbPath = filepath.Join(path, "jts.db")
	}

	db := new(Database)
	c := newConnector(dbPath, db.updateHook)
	db_ := sql.OpenDB(c)
	err := db_.Ping()
	if err != nil {
		return nil, err
	}
	db.DB = sqlx.NewDb(db_, "sqlite3")
	return db, nil
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
	err := d.DB.Select(&sessions, `
SELECT id, description, notes FROM sessions
ORDER BY (SELECT MAX(end_time) FROM time_frames WHERE session_id = sessions.id)
DESC LIMIT ? OFFSET ?
`, limit, offset)
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

func (d *Database) EditTimeframe(sessionID, timeframeID string, tf data.Timeframe) error {
	tx := d.DB.MustBegin()
	_, err := tx.Exec("UPDATE time_frames SET start_time = ?, end_time = ? WHERE session_id = ? AND id = ?", tf.Start, tf.End, sessionID, timeframeID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (d *Database) DeleteTimeframe(sessionID, timeframeID string) error {
	tx := d.DB.MustBegin()
	_, err := tx.Exec("DELETE FROM time_frames WHERE session_id = ? AND id = ?", sessionID, timeframeID)
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

// UpdateHookFn is called when a database update is made.
// op is one of SQLITE_INSERT, SQLITE_UPDATE, SQLITE_DELETE.
// cf. sqlite3.SQLiteConn.RegisterUpdateHook
type UpdateHookFn func(op int, name, table string, rowid int64)

func (d *Database) Notify(fn UpdateHookFn) {
	d.notifyFns = append(d.notifyFns, fn)
}

func (d *Database) updateHook(op int, name, table string, rowid int64) {
	log.Println("updateHook", op, name, table, rowid)
	for _, fn := range d.notifyFns {
		go fn(op, name, table, rowid)
	}
}
