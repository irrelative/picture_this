package server

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"picture-this/internal/db"
	"picture-this/internal/web"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAdminView(c *gin.Context) {
	gameID := strings.TrimSpace(c.Param("gameID"))
	if gameID == "" {
		c.Status(http.StatusNotFound)
		return
	}
	displayID := gameID
	if data, ok := s.loadAdminDataFromMemory(gameID); ok {
		s.populateAdminRuntimeState(&data, gameID)
		templ.Handler(web.Admin(displayID, data)).ServeHTTP(c.Writer, c.Request)
		return
	}
	if s.db == nil {
		templ.Handler(web.Admin(displayID, web.AdminData{Error: "Database not configured."})).ServeHTTP(c.Writer, c.Request)
		return
	}
	dbID, resolvedID, err := s.resolveAdminGameID(gameID)
	if err != nil {
		log.Printf("admin view missing game_id=%s", gameID)
		c.Redirect(http.StatusFound, "/admin")
		return
	}
	displayID = resolvedID
	data, errMsg := s.loadAdminDataByID(dbID)
	if errMsg != "" {
		data.Error = errMsg
	}
	s.populateAdminRuntimeState(&data, displayID)
	templ.Handler(web.Admin(displayID, data)).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) handleAdminHome(c *gin.Context) {
	active := s.homeSummaries()
	dbGames := make([]web.AdminDBGameSummary, 0)
	if s.db != nil {
		var records []db.Game
		if err := s.db.Order("created_at desc").Find(&records).Error; err == nil {
			counts := map[uint]int{}
			type countRow struct {
				GameID uint
				Total  int
			}
			var rows []countRow
			_ = s.db.Model(&db.Player{}).Select("game_id, count(*) as total").Group("game_id").Scan(&rows).Error
			for _, row := range rows {
				counts[row.GameID] = row.Total
			}
			for _, record := range records {
				dbGames = append(dbGames, web.AdminDBGameSummary{
					ID:        record.ID,
					JoinCode:  record.JoinCode,
					Phase:     record.Phase,
					Players:   counts[record.ID],
					CreatedAt: record.CreatedAt,
					UpdatedAt: record.UpdatedAt,
				})
			}
		}
	}
	templ.Handler(web.AdminHome(active, dbGames)).ServeHTTP(c.Writer, c.Request)
}

func (s *Server) loadAdminDataFromMemory(gameID string) (web.AdminData, bool) {
	data := web.AdminData{}
	if s.db == nil {
		data.Error = "Database not configured."
		return data, true
	}
	game, ok := s.findGameInStore(gameID)
	if !ok || game == nil {
		return web.AdminData{}, false
	}
	if err := s.ensureGameDBID(game); err != nil {
		data.Error = "Failed to resolve game in database."
		return data, true
	}
	if game.DBID == 0 {
		data.Error = "Game not found in database."
		return data, true
	}
	loaded, errMsg := s.loadAdminDataByID(game.DBID)
	if errMsg != "" {
		loaded.Error = errMsg
	}
	return loaded, true
}

func (s *Server) resolveAdminGameID(param string) (uint, string, error) {
	candidate := strings.TrimPrefix(param, "db-")
	if id, err := strconv.Atoi(candidate); err == nil && id > 0 {
		var record db.Game
		if err := s.db.First(&record, uint(id)).Error; err == nil {
			return record.ID, "db-" + candidate, nil
		}
	}
	var record db.Game
	if err := s.db.Where("join_code = ?", param).First(&record).Error; err != nil {
		return 0, "", err
	}
	return record.ID, record.JoinCode, nil
}

func (s *Server) loadAdminDataByID(gameDBID uint) (web.AdminData, string) {
	data := web.AdminData{}
	if err := s.db.First(&data.Game, gameDBID).Error; err != nil {
		return data, "Failed to load game record."
	}
	if err := s.db.Where("game_id = ?", gameDBID).Order("id asc").Find(&data.Players).Error; err != nil {
		return data, "Failed to load players."
	}
	if err := s.db.Where("game_id = ?", gameDBID).Order("number asc").Find(&data.Rounds).Error; err != nil {
		return data, "Failed to load rounds."
	}
	if err := s.db.Where("game_id = ?", gameDBID).Order("created_at asc").Find(&data.Events).Error; err != nil {
		return data, "Failed to load events."
	}

	roundIDs := make([]uint, 0, len(data.Rounds))
	for _, round := range data.Rounds {
		roundIDs = append(roundIDs, round.ID)
	}
	if len(roundIDs) > 0 {
		if err := s.db.Where("round_id IN ?", roundIDs).Order("id asc").Find(&data.Prompts).Error; err != nil {
			return data, "Failed to load prompts."
		}
		if err := s.db.Where("round_id IN ?", roundIDs).Order("id asc").Find(&data.Drawings).Error; err != nil {
			return data, "Failed to load drawings."
		}
		if err := s.db.Where("round_id IN ?", roundIDs).Order("id asc").Find(&data.Guesses).Error; err != nil {
			return data, "Failed to load guesses."
		}
		if err := s.db.Where("round_id IN ?", roundIDs).Order("id asc").Find(&data.Votes).Error; err != nil {
			return data, "Failed to load votes."
		}
	}
	return data, ""
}

func (s *Server) populateAdminRuntimeState(data *web.AdminData, gameID string) {
	if data == nil {
		return
	}
	game, ok := s.findGameInStore(gameID)
	if !ok || game == nil {
		return
	}
	data.InMemory = true
	data.TotalPlayers = len(game.Players)
	data.ClaimedPlayers = countClaimed(game.Players)
	if game.Phase == phasePaused {
		data.Paused = true
		data.PausedPhase = game.PausedPhase
	}
}
