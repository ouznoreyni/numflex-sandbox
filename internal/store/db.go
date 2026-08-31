package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func Open(ctx context.Context, url string) (*DB, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("ouverture du pool : %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("connexion à la base : %w", err)
	}
	return &DB{Pool: pool}, nil
}

func (d *DB) Close() { d.Pool.Close() }

func Migrate(url string) error {
	dir, err := RepertoireMigrations()
	if err != nil {
		return err
	}
	m, err := migrate.New("file://"+dir, url)
	if err != nil {
		return fmt.Errorf("initialisation des migrations : %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("application des migrations : %w", err)
	}
	return nil
}

// RepertoireMigrations remonte l'arborescence jusqu'à trouver le dossier migrations,
// pour que les tests s'exécutent depuis n'importe quel paquet.
func RepertoireMigrations() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 8; i++ {
		candidat := filepath.Join(dir, "migrations")
		if st, err := os.Stat(candidat); err == nil && st.IsDir() {
			return candidat, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("répertoire migrations introuvable")
}
