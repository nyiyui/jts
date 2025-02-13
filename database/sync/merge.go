package sync

import (
	"sort"

	"nyiyui.ca/jts/data"
)

type MergeConflicts struct {
	Sessions   []MergeConflict[data.Session]
	Timeframes []MergeConflict[data.Timeframe]
}

type MergeConflict[T any] struct {
	Original, Local, Remote T
}

type Changes struct {
	Sessions   []Change[data.Session]
	Timeframes []Change[data.Timeframe]
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

func Merge(original, local, remote ExportedDatabase) (Changes, MergeConflicts) {
	// sessions
	changesS, conflictsS := mergeSlice(mergeSession, getIDSession, original.Sessions, local.Sessions, remote.Sessions)
	// timeframes
	changesT, conflictsT := mergeSlice(mergeTimeframe, getIDTimeframe, original.Timeframes, local.Timeframes, remote.Timeframes)
	return Changes{changesS, changesT}, MergeConflicts{conflictsS, conflictsT}
}

func getIDSession(s data.Session) string {
	return s.ID
}

func getIDTimeframe(tf data.Timeframe) string {
	return tf.ID
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
	var i int
	for j := 0; i < len(local) && j < len(remote); {
		if getID(local[i]) == getID(remote[j]) {
			chs, cfs := merge(original[i], local[i], remote[j])
			changes = append(changes, chs...)
			conflicts = append(conflicts, cfs...)
			i++
			j++
		} else if getID(local[i]) < getID(remote[j]) {
			// remote is missing local[i]
			changes = append(changes, Change[T]{
				ChangeOperationExist,
				local[i],
			})
			i++
		} else {
			// local is missing remote[j]
			changes = append(changes, Change[T]{
				ChangeOperationRemove,
				remote[j],
			})
			j++
		}
	}
	if i < len(local) {
		for ; i < len(local); i++ {
			changes = append(changes, Change[T]{
				ChangeOperationExist,
				local[i],
			})
		}
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
	return nil, []MergeConflict[T]{
		{original, local, remote},
	}
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
