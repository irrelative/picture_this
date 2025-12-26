package server

import (
	"bytes"
	"context"

	"picture-this/internal/web"
)

func (s *Server) renderHomeGamesHTML() string {
	var buf bytes.Buffer
	if err := web.ActiveGamesList(s.homeSummaries()).Render(context.Background(), &buf); err != nil {
		return ""
	}
	return buf.String()
}
