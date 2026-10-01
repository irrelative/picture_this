package db

import (
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Each case uses its own schema and leaves existing game data untouched.
func TestMigratePostgres(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL_TEST")
	if dsn == "" {
		t.Skip("set DATABASE_URL_TEST to run Postgres migration integration tests")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer adminSQL.Close()
	// Keep the shared extension outside the disposable schemas.
	if err := admin.Exec("CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public").Error; err != nil {
		t.Fatal(err)
	}
	models := []any{&User{}, &Game{}, &Player{}, &Round{}, &Prompt{}, &Drawing{}, &Guess{}, &Vote{}, &Like{}, &Event{}, &PromptLibrary{}, &Session{}}
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy_gorm_%t", legacy), func(t *testing.T) {
			schema := fmt.Sprintf("migration_test_%d", time.Now().UnixNano())
			if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
					t.Error(err)
				}
			})
			parsed, err := url.Parse(dsn)
			if err != nil || parsed.Scheme == "" {
				t.Fatal("DATABASE_URL_TEST must be a Postgres URL")
			}
			query := parsed.Query()
			query.Set("search_path", schema+",public")
			parsed.RawQuery = query.Encode()
			conn, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := conn.DB()
			if err != nil {
				t.Fatal(err)
			}
			defer sqlDB.Close()
			sqlDB.SetMaxOpenConns(1)
			if legacy {
				if err := conn.AutoMigrate(models...); err != nil {
					t.Fatal(err)
				}
				if err := conn.Create(&PromptLibrary{Text: "Existing prompt"}).Error; err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < 2; i++ {
				if err := Migrate(conn); err != nil {
					t.Fatal(err)
				}
				if sqlDB.Stats().InUse != 0 {
					t.Fatal("migration connection was not returned to the pool")
				}
			}
			// All persisted fields must be supplied by SQL, without AutoMigrate at startup.
			for _, model := range models {
				statement := &gorm.Statement{DB: conn}
				if err := statement.Parse(model); err != nil {
					t.Fatal(err)
				}
				for _, field := range statement.Schema.Fields {
					if field.DBName != "" && !conn.Migrator().HasColumn(model, field.DBName) {
						t.Errorf("missing column %s.%s", statement.Schema.Table, field.DBName)
					}
				}
			}
			if !conn.Migrator().HasColumn(&PromptLibrary{}, "embedding") {
				t.Fatal("missing pgvector embedding column")
			}
			var dirty bool
			if err := conn.Raw("SELECT dirty FROM schema_migrations").Scan(&dirty).Error; err != nil {
				t.Fatal(err)
			}
			if dirty {
				t.Fatal("migration history is dirty")
			}
			if legacy {
				var count int64
				if err := conn.Model(&PromptLibrary{}).Where("text = ?", "Existing prompt").Count(&count).Error; err != nil || count != 1 {
					t.Fatalf("existing prompt lost: count=%d err=%v", count, err)
				}
			}
			user := User{Email: "unique@example.com", Username: "Host", PasswordHash: "test"}
			if err := conn.Create(&user).Error; err != nil {
				t.Fatal(err)
			}
			user.ID = 0
			if err := conn.Create(&user).Error; err == nil {
				t.Fatal("duplicate email was accepted")
			}
		})
	}
}
