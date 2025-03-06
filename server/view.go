package server

import (
	"net/http"
)

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	session, err := s.db.GetSession(id)
	if err != nil {
		http.Error(w, "session not found", 404)
		return
	}
	s.renderTemplate("session.html", w, r, map[string]interface{}{
		"Session": session,
	})
}
