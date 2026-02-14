package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"picture-this/internal/config"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	forceTo := flag.Int("to", -1, "force schema_migrations version to this value (default: previous migration from current dirty version)")
	runUp := flag.Bool("up", true, "run migrations up after forcing")
	flag.Parse()

	if err := config.LoadDotEnv(".env"); err != nil {
		log.Printf("failed to load .env: %v", err)
	}

	sourceURL, hasFiles, migrationsPath, err := migrationSourceURL()
	if err != nil {
		log.Fatalf("migration source error: %v", err)
	}
	if !hasFiles {
		log.Println("no migrations found")
		return
	}

	m, err := migrate.New(sourceURL, mustDatabaseURL())
	if err != nil {
		log.Fatalf("migration setup failed: %v", err)
	}

	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		log.Fatalf("read migration version failed: %v", err)
	}

	if err == migrate.ErrNilVersion {
		log.Println("schema_migrations is empty; nothing to repair")
		if *runUp {
			applyUp(m)
		}
		return
	}

	if !dirty {
		log.Printf("database is not dirty at version %d", version)
		if *runUp {
			applyUp(m)
		}
		return
	}

	target, targetSource, err := determineForceTarget(*forceTo, version, migrationsPath)
	if err != nil {
		log.Fatalf("determine force target failed: %v", err)
	}
	if err := m.Force(int(target)); err != nil {
		log.Fatalf("force migration version failed: %v", err)
	}
	log.Printf("forced schema_migrations to version %d (%s)", target, targetSource)

	if *runUp {
		applyUp(m)
	}
}

func applyUp(m *migrate.Migrate) {
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("database migration failed after repair: %v", err)
	}
	log.Println("database migrations applied")
}

func determineForceTarget(flagValue int, currentVersion uint, migrationsPath string) (uint, string, error) {
	if flagValue >= 0 {
		return uint(flagValue), "explicit --to", nil
	}
	versions, err := migrationVersions(migrationsPath)
	if err != nil {
		return 0, "", err
	}
	for i := len(versions) - 1; i >= 0; i-- {
		if versions[i] < currentVersion {
			return versions[i], "previous migration file", nil
		}
	}
	return 0, "baseline version", nil
}

func mustDatabaseURL() string {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	return dsn
}

func migrationSourceURL() (sourceURL string, hasFiles bool, path string, err error) {
	path = filepath.Join("db", "migrations")
	if err = os.MkdirAll(path, 0o755); err != nil {
		return "", false, path, err
	}
	if !hasMigrationFiles(path) {
		return "", false, path, nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false, path, err
	}
	return "file://" + abs, true, path, nil
}

func hasMigrationFiles(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".sql") {
			return true
		}
	}
	return false
}

func migrationVersions(path string) ([]uint, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	seen := map[uint]struct{}{}
	versions := make([]uint, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		prefix, _, _ := strings.Cut(name, "_")
		if prefix == "" {
			return nil, fmt.Errorf("invalid migration file name: %s", name)
		}
		value, parseErr := strconv.ParseUint(prefix, 10, 64)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid migration version in file %s: %w", name, parseErr)
		}
		if value > uint64(^uint(0)) {
			return nil, fmt.Errorf("migration version %d exceeds uint width", value)
		}
		version := uint(value)
		if _, exists := seen[version]; exists {
			continue
		}
		seen[version] = struct{}{}
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })
	return versions, nil
}
