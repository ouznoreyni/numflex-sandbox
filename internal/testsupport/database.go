package testsupport

import (
	"context"
	"os"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/seed"
)

const truncateSQL = `TRUNCATE
	incident, reverse_request, otp, confirmation, etape_historique,
	demande_client, demande_numero, demande, numero, utilisateur,
	type_incident, processus, type_demande, motif_rejet, operateur
	RESTART IDENTITY CASCADE`

// NewTestDB returns a migrated, emptied and seeded database.
func NewTestDB(t *testing.T) *persistence.DB {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL absent — run via make test")
	}
	if err := persistence.Migrate(url); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	ctx := context.Background()
	db, err := persistence.Open(ctx, url)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(db.Close)

	if _, err := db.Pool.Exec(ctx, truncateSQL); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if err := seed.Run(ctx, db); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return db
}
