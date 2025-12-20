package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	name := flag.String("name", "", "migration name")
	flag.Parse()

	if *name == "" {
		log.Fatal("migration name is required")
	}
	if strings.ContainsAny(*name, " ") {
		log.Fatal("migration name must not contain spaces")
	}

	version := time.Now().UTC().Format("20060102150405")
	base := fmt.Sprintf("%s_%s", version, *name)
	upPath := filepath.Join("db", "migrations", base+".up.sql")
	downPath := filepath.Join("db", "migrations", base+".down.sql")

	if err := os.MkdirAll(filepath.Dir(upPath), 0o755); err != nil {
		log.Fatalf("create migrations dir: %v", err)
	}

	if err := writeFile(upPath, "-- up migration\n"); err != nil {
		log.Fatalf("create up migration: %v", err)
	}
	if err := writeFile(downPath, "-- down migration\n"); err != nil {
		log.Fatalf("create down migration: %v", err)
	}

	log.Printf("created %s and %s", upPath, downPath)
}

func writeFile(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("file already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
