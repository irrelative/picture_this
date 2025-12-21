package server

import (
	"log"
	"net/http"
	"strings"

	"picture-this/internal/web"

	"github.com/a-h/templ"
)

func (s *Server) handleAdminView(w http.ResponseWriter, r *http.Request) {
	gameID := strings.TrimPrefix(r.URL.Path, "/admin/")
	gameID = strings.Trim(gameID, "/")
	if gameID == "" || strings.Contains(gameID, "/") {
		http.NotFound(w, r)
		return
	}
	if _, ok := s.store.GetGame(gameID); !ok {
		log.Printf("admin view missing game_id=%s", gameID)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	templ.Handler(web.Admin(gameID)).ServeHTTP(w, r)
}
