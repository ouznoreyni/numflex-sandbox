package store_test

import (
	"context"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport"
	"github.com/stretchr/testify/require"
)

func TestMigrationsEtSchema(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()

	tables := []string{
		"operateur", "utilisateur", "motif_rejet", "type_demande", "processus",
		"type_incident", "numero", "demande", "demande_numero", "demande_client",
		"etape_historique", "confirmation", "otp", "reverse_request", "incident",
	}
	for _, tbl := range tables {
		var n int
		err := db.Pool.QueryRow(ctx,
			"SELECT count(*) FROM information_schema.tables WHERE table_name = $1", tbl).Scan(&n)
		require.NoError(t, err)
		require.Equalf(t, 1, n, "table %s absente", tbl)
	}
}

func TestNewTestDBEstIsolee(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()

	_, err := db.Pool.Exec(ctx,
		`INSERT INTO otp (numero, code, expire_a, tentatives, consomme, cree_le)
		 VALUES ('770000001', '123456', now(), 0, false, now())`)
	require.NoError(t, err)

	db2 := testsupport.NewTestDB(t)
	var n int
	require.NoError(t, db2.Pool.QueryRow(ctx, "SELECT count(*) FROM otp").Scan(&n))
	require.Equal(t, 0, n, "NewTestDB doit repartir d'une base vide")
}
