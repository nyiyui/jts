package server

import "net/http"

type Server struct {
	mux *http.ServeMux
}

func (s *Server) setupHandlers() {
	s.mux.HandleFunc("POST /lock", s.handleLock)
	s.mux.HandleFunc("POST /unlock", s.handleUnlock)
	s.mux.HandleFunc("GET /database.json", s.handleExportDatabase)
	s.mux.HandleFunc("PUT /database.json", s.handleImportDatabase)
}
