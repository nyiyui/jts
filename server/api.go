package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"slices"

	"nyiyui.ca/jts/database/sync"
	"nyiyui.ca/jts/tokens"
)

var TokenInfoKey = &tokenInfoKey{}

type tokenInfoKey struct{}

type TokenInfo struct {
	Name        string
	Permissions []Permission
}

type Permission string

const (
	PermissionSyncDatabase Permission = "database:sync"
)

func (s *Server) apiAuthz(permissionsRequired ...Permission) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := tokens.ParseToken(r.Header.Get("X-API-Token"))
			if err != nil {
				http.Error(w, "invalid token format", 400)
				return
			}
			tokenInfo := s.tokens[token.Hash()]
			for _, permissionRequired := range permissionsRequired {
				if !slices.Contains(tokenInfo.Permissions, permissionRequired) {
					http.Error(w, "insufficient permissions", 403)
					return
				}
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), TokenInfoKey, tokenInfo)))
		})
	}
}

func (s *Server) handleLock(w http.ResponseWriter, r *http.Request) {
	tokenInfo := r.Context().Value(TokenInfoKey).(TokenInfo)
	ok := s.lock.TryLock(tokenInfo.Name)
	if !ok {
		http.Error(w, "already locked", 409)
		return
	}
	http.Error(w, "locked", 200)
}

func (s *Server) handleUnlock(w http.ResponseWriter, r *http.Request) {
	tokenInfo := r.Context().Value(TokenInfoKey).(TokenInfo)
	ok := s.lock.Unlock(tokenInfo.Name)
	if !ok {
		http.Error(w, "failed to unlock", 403)
		return
	}
	http.Error(w, "unlocked", 200)
}

func (s *Server) handleGetDatabase(w http.ResponseWriter, r *http.Request) {
	ed, err := sync.Export(s.db)
	if err != nil {
		http.Error(w, "failed to export database", 500)
		return
	}
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(ed)
	if err != nil {
		// cannot WriteHeader now
		w.Write([]byte("failed to encode database export"))
		return
	}
	return
}

func (s *Server) handlePostDatabaseChanges(w http.ResponseWriter, r *http.Request) {
	var changes sync.Changes
	err := json.NewDecoder(r.Body).Decode(&changes)
	if err != nil {
		http.Error(w, "failed to decode database changes", 400)
		return
	}
	log.Printf("importing changes: sessions=%d, timeframes=%d", len(changes.Sessions), len(changes.Timeframes))
	err = sync.ImportChanges(s.db, changes)
	if err != nil {
		http.Error(w, "failed to import database changes", 500)
		return
	}
	w.WriteHeader(200)
}
