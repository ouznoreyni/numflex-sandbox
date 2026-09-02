package persistence

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
)

func Migrate(url string) error {
	dir, err := MigrationsDir()
	if err != nil {
		return err
	}
	m, err := migrate.New("file://"+dir, url)
	if err != nil {
		return fmt.Errorf("initialising the migrations: %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("applying the migrations: %w", err)
	}
	return nil
}

// MigrationsDir walks up the directory tree until it finds the migrations
// folder, so tests run correctly regardless of which package invokes them.
func MigrationsDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "migrations")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("migrations directory not found")
}
