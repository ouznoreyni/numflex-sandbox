package store

import (
	"context"
	"os"
	"testing"
)

const truncateSQL = `TRUNCATE
	incident, reverse_request, otp, confirmation, etape_historique,
	demande_client, demande_numero, demande, numero, utilisateur,
	type_incident, processus, type_demande, motif_rejet, operateur
	RESTART IDENTITY CASCADE`

// NewTestDB rend une base migrée, vidée et ensemencée. Le seed est branché en
// Task 3 via SeedFn ; tant qu'il est nil, la base reste vide.
var SeedFn func(ctx context.Context, db *DB) error

func NewTestDB(t *testing.T) *DB {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL absent — lancer via make test")
	}
	if err := Migrate(url); err != nil {
		t.Fatalf("migrations : %v", err)
	}
	ctx := context.Background()
	db, err := Open(ctx, url)
	if err != nil {
		t.Fatalf("ouverture : %v", err)
	}
	t.Cleanup(db.Close)

	if _, err := db.Pool.Exec(ctx, truncateSQL); err != nil {
		t.Fatalf("truncate : %v", err)
	}
	if SeedFn != nil {
		if err := SeedFn(ctx, db); err != nil {
			t.Fatalf("seed : %v", err)
		}
	}
	return db
}
