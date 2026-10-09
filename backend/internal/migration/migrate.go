package migration

import (
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Up runs all pending migrations from the given source (file path or URL).
func Up(sourceURL, dsn string) error {
	m, err := migrate.New(sourceURL, dsn)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// Down rolls back all migrations (or n steps if n > 0).
func Down(sourceURL, dsn string, n int) error {
	m, err := migrate.New(sourceURL, dsn)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()
	if n > 0 {
		if err := m.Steps(-n); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("migrate down %d: %w", n, err)
		}
	} else {
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("migrate down: %w", err)
		}
	}
	return nil
}

// Version returns the current migration version and whether the schema is dirty.
func Version(sourceURL, dsn string) (uint, bool, error) {
	m, err := migrate.New(sourceURL, dsn)
	if err != nil {
		return 0, false, fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()
	return m.Version()
}
