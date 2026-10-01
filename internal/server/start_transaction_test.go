package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"picture-this/internal/config"
	"picture-this/internal/db"
)

func newPostgresServerHarness(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL_TEST")
	if dsn == "" {
		t.Skip("set DATABASE_URL_TEST for Postgres startup transaction tests")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Scheme == "" {
		t.Fatal("DATABASE_URL_TEST must be a Postgres URL")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec("CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public").Error; err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("start_test_%d", time.Now().UnixNano())
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
		pool, _ := admin.DB()
		pool.Close()
	})
	query := parsed.Query()
	query.Set("search_path", schema+",public")
	parsed.RawQuery = query.Encode()
	conn, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool, _ := conn.DB(); pool.Close() })
	if err := db.Migrate(conn); err != nil {
		t.Fatal(err)
	}
	srv := New(conn, config.Default())
	ts := newTestServer(t, srv.Handler())
	testServers.Lock()
	testServers.byURL[ts.URL] = srv
	testServers.Unlock()
	t.Cleanup(func() { testServers.Lock(); delete(testServers.byURL, ts.URL); testServers.Unlock(); ts.Close() })
	return srv, ts
}

func TestStartGameRollsBackAndCanRetry(t *testing.T) {
	for _, scenario := range []string{"insufficient_prompts", "second_prompt_write_failure"} {
		t.Run(scenario, func(t *testing.T) {
			srv, ts := newPostgresServerHarness(t)
			gameID := createGame(t, ts)
			hostID := createdHostID(t, ts, gameID)
			joinPlayer(t, ts, gameID, "Guest")
			before, _ := srv.store.GetGame(gameID)
			var beforeRecord db.Game
			if err := srv.db.First(&beforeRecord, before.DBID).Error; err != nil {
				t.Fatal(err)
			}
			var beforeEvents int64
			if err := srv.db.Model(&db.Event{}).Where("game_id = ?", before.DBID).Count(&beforeEvents).Error; err != nil {
				t.Fatal(err)
			}
			if scenario == "second_prompt_write_failure" {
				if err := srv.db.Create(&[]db.PromptLibrary{{Text: "First prompt"}, {Text: "Second prompt"}}).Error; err != nil {
					t.Fatal(err)
				}
				// Reject the second insert, after a phase, round, event and prompt were written.
				if err := srv.db.Exec(`CREATE FUNCTION reject_second_prompt() RETURNS trigger AS $$
    BEGIN
     IF EXISTS (SELECT 1 FROM prompts WHERE round_id = NEW.round_id) THEN
      RAISE EXCEPTION 'injected prompt persistence failure';
     END IF;
     RETURN NEW;
    END;
    $$ LANGUAGE plpgsql;
    CREATE TRIGGER fail_second_prompt BEFORE INSERT ON prompts
    FOR EACH ROW EXECUTE FUNCTION reject_second_prompt();`).Error; err != nil {
					t.Fatal(err)
				}
			}
			resp := doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{"player_id": hostID})
			if resp.StatusCode != http.StatusConflict {
				t.Fatalf("expected failed start, got %d", resp.StatusCode)
			}
			after, _ := srv.store.GetGame(gameID)
			if after.Phase != phaseLobby || after.Version != before.Version || len(after.Rounds) != 0 || len(after.UsedPrompts) != 0 {
				t.Fatal("failed start leaked in-memory state")
			}
			var record db.Game
			if err := srv.db.First(&record, before.DBID).Error; err != nil {
				t.Fatal(err)
			}
			if record.Phase != beforeRecord.Phase || record.Version != beforeRecord.Version {
				t.Fatal("failed start leaked database phase/version")
			}
			for _, model := range []any{&db.Round{}, &db.Prompt{}} {
				var count int64
				if err := srv.db.Model(model).Count(&count).Error; err != nil || count != 0 {
					t.Fatalf("startup rows survived rollback: count=%d err=%v", count, err)
				}
			}
			var afterEvents int64
			if err := srv.db.Model(&db.Event{}).Where("game_id = ?", before.DBID).Count(&afterEvents).Error; err != nil || afterEvents != beforeEvents {
				t.Fatalf("startup events survived rollback: before=%d after=%d err=%v", beforeEvents, afterEvents, err)
			}
			if scenario == "insufficient_prompts" {
				if err := srv.db.Create(&[]db.PromptLibrary{{Text: "First prompt"}, {Text: "Second prompt"}}).Error; err != nil {
					t.Fatal(err)
				}
			} else {
				if err := srv.db.Exec("DROP TRIGGER fail_second_prompt ON prompts").Error; err != nil {
					t.Fatal(err)
				}
			}
			resp = doRequest(t, ts, http.MethodPost, "/api/games/"+gameID+"/start", map[string]any{"player_id": hostID})
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("retry failed: %d %v", resp.StatusCode, decodeBody(t, resp))
			}
			after, _ = srv.store.GetGame(gameID)
			if after.Phase != phaseDrawings || len(after.Rounds) != 1 || len(currentRound(after).Prompts) != 2 {
				t.Fatal("retry did not start a complete round")
			}
			if err := srv.db.First(&record, before.DBID).Error; err != nil {
				t.Fatal(err)
			}
			if record.Phase != after.Phase || record.Version != after.Version {
				t.Fatal("committed phase/version differs from memory")
			}
			var rounds, prompts int64
			if err := srv.db.Model(&db.Round{}).Count(&rounds).Error; err != nil {
				t.Fatal(err)
			}
			if err := srv.db.Model(&db.Prompt{}).Count(&prompts).Error; err != nil {
				t.Fatal(err)
			}
			if rounds != 1 || prompts != 2 {
				t.Fatalf("retry persisted rounds=%d prompts=%d", rounds, prompts)
			}
		})
	}
}
