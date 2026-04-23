package main

import (
	"embed"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// runMigrations applies all pending SQL migrations from the embedded migrations/ directory.
// It is idempotent: running it on an up-to-date schema is a no-op.
func runMigrations(dsn string) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	// golang-migrate pgx/v5 driver requires a "pgx5://" scheme
	dbURL := toMigrateURL(dsn)

	m, err := migrate.NewWithSourceInstance("iofs", src, dbURL)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Printf("Migrations: schema already up to date")
			return nil
		}
		return fmt.Errorf("migration failed: %w", err)
	}

	version, _, _ := m.Version()
	log.Printf("Migrations: applied successfully (schema version %d)", version)
	return nil
}

// toMigrateURL converts a postgres:// or postgresql:// DSN to the pgx5:// scheme
// required by golang-migrate's pgx/v5 driver.
func toMigrateURL(dsn string) string {
	for _, prefix := range []string{"postgresql://", "postgres://"} {
		if strings.HasPrefix(dsn, prefix) {
			return "pgx5://" + dsn[len(prefix):]
		}
	}
	return dsn
}
