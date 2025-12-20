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

	conn, err := db.Open()
	if err != nil {
		log.Printf("database disabled: %v", err)
	} else if err := db.Migrate(conn); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}

	addr := ":8080"
	if env := os.Getenv("PORT"); env != "" {
		addr = ":" + env
	}

	srv := server.New()
	log.Printf("picture-this server listening on %s", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
