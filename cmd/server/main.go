package main

import (
	"log"
	"net/http"
	"os"

	"picture-this/internal/config"
	"picture-this/internal/db"
	"picture-this/internal/server"
)

func main() {
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Printf("failed to load .env: %v", err)
	}
	cfg := config.Load()

	conn, err := db.Open()
	if err != nil {
		log.Printf("database disabled: %v", err)
	} else if err := db.Migrate(conn); err != nil {
		log.Fatalf("database migration failed: %v", err)
	} else {
		if inserted, err := db.LoadPromptLibrary(conn, "prompts.csv"); err != nil {
			log.Printf("failed to load prompts: %v", err)
		} else if inserted > 0 {
			log.Printf("loaded %d prompts", inserted)
		}
	}

	addr := ":8080"
	if env := os.Getenv("PORT"); env != "" {
		addr = ":" + env
	}

	srv := server.New(conn, cfg)
	log.Printf("picture-this server listening on %s", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
