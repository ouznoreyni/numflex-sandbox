//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// operateurOrangeConfirmation and operateurYASConfirmation are
// internal/framework/seed.OperateurOrange and .OperateurYAS, recopiés en
// littéral : un test de gateway (couche adapter) ne peut pas importer
// internal/framework (règle de dépendance, voir test/architecture_test.go),
// même dans un fichier //go:build integration — précédent déjà posé par
// user_gateway_test.go (operateurYAS).
const (
	operateurOrangeConfirmation = "6a21745ce6c37b5b5b487ec1"
	operateurYASConfirmation    = "6a2174c3e6c37b5b5b487ec4"
)

// TestConfirmationGatewayConfirmTranslatesUniqueViolation pins the one
// translation this gateway exists to make: Postgres reports a duplicate
// (demande_id, operateur_id) insert as error code 23505, and Confirm must
// turn that into port.ErrAlreadyConfirmed — not let the raw *pgconn.PgError
// leak past the gateway boundary. An operator confirming the same request
// twice is an ordinary scenario (TC-041's anti-replay guard), and this test
// causes a genuine duplicate insert against the real confirmation table
// rather than injecting the sentinel through an in-memory double, so the
// mapping from a real database error actually runs.
func TestConfirmationGatewayConfirmTranslatesUniqueViolation(t *testing.T) {
	db := testsupport.NewTestDB(t)
	ctx := context.Background()
	const id = "6a2100000000000000000101"
	now := time.Now()
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO demande
		  (id, numero, type_abonne, type_demande, statut_demande, etape_actuelle,
		   statut_etape_actuel, operateur_source_id, operateur_destinataire_id,
		   createur_operateur_id, date_demande, date_debut_etape)
		VALUES ($1, '771000101', 'PARTICULIER', 'PORTAGE', 'EN_COURS', 'CONFIRMATION',
		        'EN_COURS', $2, $3, $3, $4, $4)`,
		id, operateurOrangeConfirmation, operateurYASConfirmation, now)
	if err != nil {
		t.Fatalf("seed demande : %v", err)
	}

	g := postgres.NewConfirmationGateway(db.Pool)

	// The first confirmation must succeed — precondition, or the second
	// call's unique violation would prove nothing.
	if err := g.Confirm(ctx, id, operateurOrangeConfirmation, "première confirmation", now); err != nil {
		t.Fatalf("première confirmation : %v attendu nil", err)
	}

	// The second confirmation, same (demande_id, operateur_id), hits the
	// real primary key and must come back as port.ErrAlreadyConfirmed.
	err = g.Confirm(ctx, id, operateurOrangeConfirmation, "rejeu", now)
	if !errors.Is(err, port.ErrAlreadyConfirmed) {
		t.Fatalf("Confirm (rejeu) = %v, want port.ErrAlreadyConfirmed", err)
	}
}
