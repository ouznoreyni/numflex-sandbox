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

func insertReverse(t *testing.T, db *persistence.DB, id string) {
	t.Helper()
	_, err := db.Pool.Exec(context.Background(),
		`INSERT INTO reverse_request (id, numero, operateur_id, statut, date_demande)
		 VALUES ($1,'773000001',$2,'EN_ATTENTE',now())`, id, seed.OperatorOrangeID)
	require.NoError(t, err)
}

func TestValidationCreatesARequestDirectlyInConfirmation(t *testing.T) {
	e, db := newTestEngine(t)
	insertReverse(t, db, "r1")

	require.NoError(t, ValidateReverse(context.Background(), db, "r1"))

	var status string
	var requestID *string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT statut, demande_id FROM reverse_request WHERE id = 'r1'`).
		Scan(&status, &requestID))
	require.Equal(t, "VALIDE", status)
	require.NotNil(t, requestID)

	var requestType, step string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT type_demande, etape_actuelle FROM demande WHERE id = $1`, *requestID).
		Scan(&requestType, &step))
	require.Equal(t, "REVERSE", requestType)
	require.Equal(t, "CONFIRMATION", step, "ni ACCEPTATION, ni DESACTIVATION/ACTIVATION")

	_ = e
}

func TestRejectionCreatesNoRequest(t *testing.T) {
	_, db := newTestEngine(t)
	insertReverse(t, db, "r1")

	require.NoError(t, RejectReverse(context.Background(), db, "r1"))

	var status string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT statut FROM reverse_request WHERE id = 'r1'`).Scan(&status))
	require.Equal(t, "REJETE", status)

	var n int
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM demande`).Scan(&n))
	require.Equal(t, 0, n)
}

func TestAutomaticValidationAfterDelay(t *testing.T) {
	e, db := newTestEngine(t, func(c *config.Config) {
		c.ReverseAutoValidation = time.Nanosecond
	})
	insertReverse(t, db, "r1")

	require.NoError(t, e.Tick(context.Background()))

	var status string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT statut FROM reverse_request WHERE id = 'r1'`).Scan(&status))
	require.Equal(t, "VALIDE", status)
}

func TestNoAutomaticValidationByDefault(t *testing.T) {
	e, db := newTestEngine(t) // ReverseAutoValidation = 0
	insertReverse(t, db, "r1")

	require.NoError(t, e.Tick(context.Background()))

	var status string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT statut FROM reverse_request WHERE id = 'r1'`).Scan(&status))
	require.Equal(t, "EN_ATTENTE", status)
}

func TestARTPCompletesAReverseOnceEveryoneHasConfirmed(t *testing.T) {
	e, db := newTestEngine(t)
	insertReverse(t, db, "r1")
	require.NoError(t, ValidateReverse(context.Background(), db, "r1"))

	var requestID string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT demande_id FROM reverse_request WHERE id = 'r1'`).Scan(&requestID))

	for _, op := range []string{seed.OperatorOrangeID, seed.OperatorYASID, seed.OperatorExpressoID} {
		_, err := db.Pool.Exec(context.Background(),
			`INSERT INTO confirmation (demande_id, operateur_id, date_conf) VALUES ($1,$2,now())`,
			requestID, op)
		require.NoError(t, err)
	}

	require.NoError(t, e.Tick(context.Background()))
	require.NoError(t, e.Tick(context.Background()))

	step, _, requestStatus := requestState(t, db, requestID)
	require.Equal(t, "TERMINE", requestStatus)
	require.Equal(t, "COMPLETION", step)
}
