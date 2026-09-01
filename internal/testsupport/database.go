// Package testsupport holds test-only infrastructure shared across packages:
// the test database helper today, in-memory port doubles under ./inmemory.
package testsupport

import (
	"context"
	"os"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/seed"
	"github.com/ouznoreyni/numflex-sandbox/internal/store"
)

const truncateSQL = `TRUNCATE
	incident, reverse_request, otp, confirmation, etape_historique,
	demande_client, demande_numero, demande, numero, utilisateur,
	type_incident, processus, type_demande, motif_rejet, operateur
	RESTART IDENTITY CASCADE`

// NewTestDB rend une base migrée, vidée et ensemencée.
func NewTestDB(t *testing.T) *store.DB {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL absent — lancer via make test")
	}
	if err := store.Migrate(url); err != nil {
		t.Fatalf("migrations : %v", err)
	}
	ctx := context.Background()
	db, err := store.Open(ctx, url)
	if err != nil {
		t.Fatalf("ouverture : %v", err)
	}
	t.Cleanup(db.Close)

	if _, err := db.Pool.Exec(ctx, truncateSQL); err != nil {
		t.Fatalf("truncate : %v", err)
	}
	if err := seed.Run(ctx, db); err != nil {
		t.Fatalf("seed : %v", err)
	}
	return db
}
