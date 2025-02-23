package sync

import (
	"fmt"
	"log"
	"slices"
	"sort"

	"github.com/jmoiron/sqlx"
	"nyiyui.ca/jts/data"
	"nyiyui.ca/jts/database"
)

type ExportedDatabase struct {
	Sessions   []data.Session
	Timeframes []data.Timeframe
	Tasks      []data.Task
}

func Export(d *database.Database) (ExportedDatabase, error) {
	var ed ExportedDatabase
	err := d.DB.Select(&ed.Sessions, "SELECT id, description, notes FROM sessions")
	if err != nil {
		return ExportedDatabase{}, err
	}
	err = d.DB.Select(&ed.Timeframes, "SELECT id, session_id, start_time, end_time FROM time_frames")
	if err != nil {
		return ExportedDatabase{}, err
	}
	err = d.DB.Select(&ed.Tasks, "SELECT id, description FROM tasks")
	if err != nil {
		return ExportedDatabase{}, err
	}
	return ed, nil
}

// ReplaceAndImport replaces the database with the exported database and then imports the changes.
func ReplaceAndImport(d *database.Database, ed ExportedDatabase, c Changes) error {
	tx := d.DB.MustBegin()
	if err := replace(tx, ed); err != nil {
		tx.Rollback()
		return err
	}
	if err := importChanges(tx, c); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func replace(tx *sqlx.Tx, ed ExportedDatabase) error {
	_, err := tx.Exec("DELETE FROM sessions")
	if err != nil {
		return err
	}
	_, err = tx.Exec("DELETE FROM time_frames")
	if err != nil {
		return err
	}
	_, err = tx.Exec("DELETE FROM tasks")
	if err != nil {
		return err
	}

	for _, s := range ed.Sessions {
		_, err = tx.Exec("INSERT INTO sessions (id, description, notes) VALUES (?, ?, ?)", s.ID, s.Description, s.Notes)
		if err != nil {
			return err
		}
	}
	for _, tf := range ed.Timeframes {
		_, err = tx.Exec("INSERT INTO time_frames (id, session_id, start_time, end_time) VALUES (?, ?, ?, ?)", tf.ID, tf.SessionID, tf.Start, tf.End)
		if err != nil {
			return err
		}
	}
	for _, tf := range ed.Tasks {
		_, err = tx.Exec("INSERT INTO tasks (id, description) VALUES (?, ?)", tf.ID, tf.Description)
		if err != nil {
			return err
		}
	}
	return nil
}

func ImportChanges(d *database.Database, c Changes) error {
	log.Printf("importing changes %#v", c)
	tx := d.DB.MustBegin()
	if err := importChanges(tx, c); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func importChanges(tx *sqlx.Tx, c Changes) error {
	var err error
	for i, ch := range c.Sessions {
		log.Printf("importing session change %d: %#v", i, ch)
		switch ch.Operation {
		case ChangeOperationExist:
			_, err = tx.Exec("REPLACE INTO sessions (id, description, notes) VALUES (?, ?, ?)", ch.Data.ID, ch.Data.Description, ch.Data.Notes)
		case ChangeOperationRemove:
			_, err = tx.Exec("DELETE FROM sessions WHERE id = ?", ch.Data.ID)
		}
		if err != nil {
			return fmt.Errorf("change %d (%#v): %w", i, ch, err)
		}
	}
	for i, ch := range c.Timeframes {
		log.Printf("importing timeframe change %d: %#v", i, ch)
		switch ch.Operation {
		case ChangeOperationExist:
			_, err = tx.Exec("REPLACE INTO time_frames (id, session_id, start_time, end_time) VALUES (?, ?, ?, ?)", ch.Data.ID, ch.Data.SessionID, ch.Data.Start, ch.Data.End)
		case ChangeOperationRemove:
			_, err = tx.Exec("DELETE FROM time_frames WHERE id = ?", ch.Data.ID)
		}
		if err != nil {
			return fmt.Errorf("change %d (%#v): %w", i, ch, err)
		}
	}
	for i, ch := range c.Tasks {
		log.Printf("importing task change %d: %#v", i, ch)
		switch ch.Operation {
		case ChangeOperationExist:
			_, err = tx.Exec("REPLACE INTO tasks (id, description) VALUES (?, ?)", ch.Data.ID, ch.Data.Description)
		case ChangeOperationRemove:
			_, err = tx.Exec("DELETE FROM tasks WHERE id = ?", ch.Data.ID)
		}
		if err != nil {
			return fmt.Errorf("change %d (%#v): %w", i, ch, err)
		}
	}
	return nil
}

type MergeConflicts struct {
	Sessions   []MergeConflict[data.Session]
	Timeframes []MergeConflict[data.Timeframe]
	Tasks      []MergeConflict[data.Task]
}

type MergeConflict[T any] struct {
	Original, Local, Remote T
}

type Changes struct {
	Sessions   []Change[data.Session]
	Timeframes []Change[data.Timeframe]
	Tasks      []Change[data.Task]
}

type Change[T any] struct {
	Operation ChangeOperation
	Data      T
}

type ChangeOperation int

const (
	ChangeOperationExist ChangeOperation = iota
	ChangeOperationRemove
)

func (co ChangeOperation) String() string {
	switch co {
	case ChangeOperationExist:
		return "exist"
	case ChangeOperationRemove:
		return "remove"
	default:
		return fmt.Sprintf("unknown(%d)", int(co))
	}
}

func Merge(original, local, remote ExportedDatabase) (Changes, MergeConflicts) {
	// sessions
	changesS, conflictsS := mergeSlice(mergeSession, getIDSession, original.Sessions, local.Sessions, remote.Sessions)
	// timeframes
	changesT, conflictsT := mergeSlice(mergeTimeframe, getIDTimeframe, original.Timeframes, local.Timeframes, remote.Timeframes)
	// tasks
	changesTasks, conflictsTasks := mergeSlice(mergeTask, getIDTask, original.Tasks, local.Tasks, remote.Tasks)
	return Changes{changesS, changesT, changesTasks}, MergeConflicts{conflictsS, conflictsT, conflictsTasks}
}

func mergeSlice[T any](merge func(original, local, remote T) ([]Change[T], []MergeConflict[T]), getID func(T) string, original, local, remote []T) ([]Change[T], []MergeConflict[T]) {
	var changes []Change[T]
	var conflicts []MergeConflict[T]
	sort.Slice(local, func(i, j int) bool {
		return getID(local[i]) < getID(local[j])
	})
	sort.Slice(remote, func(i, j int) bool {
		return getID(remote[i]) < getID(remote[j])
	})
	log.Printf("remote length %d", len(remote))
	var i int
	for j := 0; i < len(local) && j < len(remote); {
		log.Printf("comparing local=%s remote=%s", getID(local[i]), getID(remote[j]))
		if getID(local[i]) == getID(remote[j]) {
			log.Printf("merge %s", getID(local[i]))
			originalIndex := slices.IndexFunc(original, func(s T) bool {
				return getID(s) == getID(local[i])
			})
			chs, cfs := merge(original[originalIndex], local[i], remote[j])
			changes = append(changes, chs...)
			conflicts = append(conflicts, cfs...)
			i++
			j++
		} else if getID(local[i]) < getID(remote[j]) {
			// remote is missing local[i]
			log.Printf("remote is missing local=%s", getID(local[i]))
			changes = append(changes, Change[T]{
				ChangeOperationExist,
				local[i],
			})
			i++
		} else {
			// local is missing remote[j]
			// no changes needed to remote
			log.Printf("local is missing remote=%s", getID(remote[j]))
			j++
		}
	}
	for ; i < len(local); i++ {
		log.Printf("() remote is missing local=%s", getID(local[i]))
		changes = append(changes, Change[T]{
			ChangeOperationExist,
			local[i],
		})
	}
	return changes, conflicts
}

// merge returns changes to apply to remote.
func merge[T any](equal func(a, b T) bool, original, local, remote T) ([]Change[T], []MergeConflict[T]) {
	if equal(local, remote) {
		return nil, nil
	}
	if equal(original, local) {
		return nil, nil
	}
	if equal(original, remote) {
		return []Change[T]{
			{ChangeOperationExist, local},
		}, nil
	}
	log.Printf("merge conflict: original=%#v, local=%#v, remote=%#v", original, local, remote)
	return nil, []MergeConflict[T]{
		{original, local, remote},
	}
}

func getIDSession(s data.Session) string {
	return s.ID
}

func getIDTimeframe(tf data.Timeframe) string {
	return tf.ID
}

func getIDTask(t data.Task) string {
	return t.ID
}

func mergeSession(original, local, remote data.Session) ([]Change[data.Session], []MergeConflict[data.Session]) {
	return merge[data.Session](func(a, b data.Session) bool {
		return a.Equal(b)
	}, original, local, remote)
}

func mergeTimeframe(original, local, remote data.Timeframe) ([]Change[data.Timeframe], []MergeConflict[data.Timeframe]) {
	return merge[data.Timeframe](func(a, b data.Timeframe) bool {
		return a.Equal(b)
	}, original, local, remote)
}

func mergeTask(original, local, remote data.Task) ([]Change[data.Task], []MergeConflict[data.Task]) {
	return merge[data.Task](func(a, b data.Task) bool {
		return a.Equal(b)
	}, original, local, remote)
}
