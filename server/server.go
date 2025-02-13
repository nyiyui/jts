package server

import (
	"net/http"
	"sync"

	"nyiyui.ca/jts/database"
	"nyiyui.ca/jts/tokens"
)

type Server struct {
	mux    *http.ServeMux
	lock   *serverLock
	tokens map[tokens.TokenHash]TokenInfo
	db     *database.Database
}

func (s *Server) setupHandlers() {
	s.mux.Handle("POST /lock", s.apiAuthz(PermissionSyncDatabase)(http.HandlerFunc(s.handleLock)))
	s.mux.Handle("POST /unlock", s.apiAuthz(PermissionSyncDatabase)(http.HandlerFunc(s.handleUnlock)))
	s.mux.Handle("GET /database", s.apiAuthz(PermissionSyncDatabase)(http.HandlerFunc(s.handleGetDatabase)))
	s.mux.Handle("PUT /database", s.apiAuthz(PermissionSyncDatabase)(http.HandlerFunc(s.handlePutDatabase)))
	s.mux.Handle("POST /database/changes", s.apiAuthz(PermissionSyncDatabase)(http.HandlerFunc(s.handlePostDatabaseChanges)))
}

type serverLock struct {
	mutex    sync.Mutex
	locked   bool
	lockedBy string
}

func (sl *serverLock) TryLock(lockedBy string) (ok bool) {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()
	if !sl.locked {
		sl.locked = true
		sl.lockedBy = lockedBy
		ok = true
	}
	return
}

func (sl *serverLock) Unlock(mustBeLockedBy string) (ok bool) {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()
	if sl.lockedBy == mustBeLockedBy {
		sl.locked = false
		sl.lockedBy = ""
		ok = true
	}
	return
}

func (sl *serverLock) LockedBy() string {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()
	return sl.lockedBy
}
