package main

import (
	"log"
	"net/http"
	"os"

	"picture-this/internal/server"
)

func main() {
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
