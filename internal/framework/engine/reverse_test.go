package engine

import (
	"context"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/seed"
	"github.com/stretchr/testify/require"
)

func insererReverse(t *testing.T, db *persistence.DB, id string) {
	t.Helper()
	_, err := db.Pool.Exec(context.Background(),
		`INSERT INTO reverse_request (id, numero, operateur_id, statut, date_demande)
		 VALUES ($1,'773000001',$2,'EN_ATTENTE',now())`, id, seed.OperateurOrange)
	require.NoError(t, err)
}

func TestValidationCreeUneDemandeDirectementEnConfirmation(t *testing.T) {
	e, db := moteur(t)
	insererReverse(t, db, "r1")

	require.NoError(t, ValiderReverse(context.Background(), db, "r1"))

	var statut string
	var demandeID *string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT statut, demande_id FROM reverse_request WHERE id = 'r1'`).
		Scan(&statut, &demandeID))
	require.Equal(t, "VALIDE", statut)
	require.NotNil(t, demandeID)

	var typeDem, etape string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT type_demande, etape_actuelle FROM demande WHERE id = $1`, *demandeID).
		Scan(&typeDem, &etape))
	require.Equal(t, "REVERSE", typeDem)
	require.Equal(t, "CONFIRMATION", etape, "ni ACCEPTATION, ni DESACTIVATION/ACTIVATION")

	_ = e
}

func TestRejetNeCreeAucuneDemande(t *testing.T) {
	_, db := moteur(t)
	insererReverse(t, db, "r1")

	require.NoError(t, RejeterReverse(context.Background(), db, "r1"))

	var statut string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT statut FROM reverse_request WHERE id = 'r1'`).Scan(&statut))
	require.Equal(t, "REJETE", statut)

	var n int
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM demande`).Scan(&n))
	require.Equal(t, 0, n)
}

func TestValidationAutomatiqueApresDelai(t *testing.T) {
	e, db := moteur(t, func(c *config.Config) {
		c.ReverseAutoValidation = time.Nanosecond
	})
	insererReverse(t, db, "r1")

	require.NoError(t, e.Tick(context.Background()))

	var statut string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT statut FROM reverse_request WHERE id = 'r1'`).Scan(&statut))
	require.Equal(t, "VALIDE", statut)
}

func TestPasDeValidationAutomatiqueParDefaut(t *testing.T) {
	e, db := moteur(t) // ReverseAutoValidation = 0
	insererReverse(t, db, "r1")

	require.NoError(t, e.Tick(context.Background()))

	var statut string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT statut FROM reverse_request WHERE id = 'r1'`).Scan(&statut))
	require.Equal(t, "EN_ATTENTE", statut)
}

func TestLARTPCompleteUnReverseUneFoisToutLeMondeConfirme(t *testing.T) {
	e, db := moteur(t)
	insererReverse(t, db, "r1")
	require.NoError(t, ValiderReverse(context.Background(), db, "r1"))

	var demandeID string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT demande_id FROM reverse_request WHERE id = 'r1'`).Scan(&demandeID))

	for _, op := range []string{seed.OperateurOrange, seed.OperateurYAS, seed.OperateurExpresso} {
		_, err := db.Pool.Exec(context.Background(),
			`INSERT INTO confirmation (demande_id, operateur_id, date_conf) VALUES ($1,$2,now())`,
			demandeID, op)
		require.NoError(t, err)
	}

	require.NoError(t, e.Tick(context.Background()))
	require.NoError(t, e.Tick(context.Background()))

	etape, _, statutDemande := etatDemande(t, db, demandeID)
	require.Equal(t, "TERMINE", statutDemande)
	require.Equal(t, "COMPLETION", etape)
}
