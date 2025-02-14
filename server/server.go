package server

import (
	"net/http"
	"sync"

	"github.com/google/safehtml/template"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"nyiyui.ca/jts/database"
	"nyiyui.ca/jts/tokens"
)

type Server struct {
	mux         *http.ServeMux
	lock        *serverLock
	tokens      map[tokens.TokenHash]TokenInfo
	users       map[string]UserInfo
	db          *database.Database
	store       sessions.Store
	oauthConfig *oauth2.Config
	tps         map[string]*template.Template
}

func New(oauthConfig *oauth2.Config, db *database.Database, tokens map[tokens.TokenHash]TokenInfo, users map[string]UserInfo, store sessions.Store) (*Server, error) {
	s := &Server{
		mux:         http.NewServeMux(),
		lock:        new(serverLock),
		tokens:      tokens,
		users:       users,
		db:          db,
		store:       store,
		oauthConfig: oauthConfig,
		tps:         make(map[string]*template.Template),
	}
	s.setupHandlers()
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) setupHandlers() {
	s.mux.Handle("POST /lock", s.apiAuthz(PermissionSyncDatabase)(http.HandlerFunc(s.handleLock)))
	s.mux.Handle("POST /unlock", s.apiAuthz(PermissionSyncDatabase)(http.HandlerFunc(s.handleUnlock)))
	s.mux.Handle("GET /database", s.apiAuthz(PermissionSyncDatabase)(http.HandlerFunc(s.handleGetDatabase)))
	s.mux.Handle("PUT /database", s.apiAuthz(PermissionSyncDatabase)(http.HandlerFunc(s.handlePutDatabase)))
	s.mux.Handle("POST /database/changes", s.apiAuthz(PermissionSyncDatabase)(http.HandlerFunc(s.handlePostDatabaseChanges)))

	s.mux.HandleFunc("GET /login", s.handleLogin)
	s.mux.HandleFunc("GET /login/callback", s.handleLoginCallback)
	s.mux.HandleFunc("GET /login/settings", s.handleLoginSettings)
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
