package db

import (
	"context"
	"errors"
	"log"
	"os"

	"picture-this/db/migrations"

	"github.com/golang-migrate/migrate/v4"
	migrationpostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Open connects to Postgres using DATABASE_URL.
func Open() (*gorm.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, errors.New("DATABASE_URL is not set")
	}
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

// Migrate applies the same versioned SQL migrations used by make migrate.
func Migrate(conn *gorm.DB) error {
	if conn == nil {
		return errors.New("db connection is nil")
	}
	sqlDB, err := conn.DB()
	if err != nil {
		return err
	}
	source, err := iofs.New(migrations.Files, ".")
	if err != nil {
		return err
	}
	defer source.Close()
	connection, err := sqlDB.Conn(context.Background())
	if err != nil {
		return err
	}
	defer connection.Close()
	driver, err := migrationpostgres.WithConnection(context.Background(), connection, &migrationpostgres.Config{})
	if err != nil {
		return err
	}
	// Return the dedicated migration connection without closing the shared pool.
	runner, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return err
	}
	if err := runner.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	log.Println("database migrations applied")
	return nil
}
