//go:build integration

package controller_test

// TestConfirmationFranchitLeSeuilExactement exists because Task 15's
// completion boundary — ConfirmRequestInteractor.Execute's
// count >= len(expected) — is business-critical (it decides when a step
// advances) and was, until this file, only exercised against
// testsupport/inmemory's map-backed Count, never against a real Postgres
// COUNT(*). One test that crosses the boundary is worth more than two that
// sit on either side of it, so this walks a request to exactly one
// confirmation short of the threshold, asserts CONFIRMATION has NOT been
// soldée, then adds the final confirmation and asserts it HAS.

import (
	"context"
	"net/http"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
	"github.com/stretchr/testify/require"
)

// operateurExpresso is internal/framework/seed.OperateurExpresso, recopié en
// littéral : un test de contrôleur (couche adapter) ne peut pas importer
// internal/framework (règle de dépendance, voir test/architecture_test.go),
// même dans un fichier //go:build integration — précédent déjà posé par
// creation_restitution_test.go (operateurOrange, operateurYAS ci-contre) et
// user_gateway_test.go.
const operateurExpresso = "6a217510e6c37b5b5b487ec7"

// comptes associe un id opérateur du seed à son compte de connexion, pour
// piloter les confirmations dans l'ordre attendu par entity.ExpectedConfirmers.
var comptes = map[string][2]string{
	operateurOrange:   {"orange", "orange2026"},
	operateurYAS:      {"yas", "yas2026"},
	operateurExpresso: {"expresso", "expresso2026"},
}

func TestConfirmationFranchitLeSeuilExactement(t *testing.T) {
	h := routerharness.NewRouterHarness(t)

	// Une RESTITUTION exige tout le monde, destinataire compris (contrairement
	// à un PORTAGE) — le cas où le compte du seuil est le moins trivial.
	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.Jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})
	require.Equal(t, http.StatusCreated, rep.StatusCode, corps)
	id := corps["data"].(map[string]any)["id"].(string)
	avancerA(h, id, "CONFIRMATION")

	// entity.ExpectedConfirmers est l'unique autorité sur qui doit confirmer :
	// on la dérive ici du type et du destinataire réels de la demande créée,
	// lus en base, plutôt que de réénoncer la règle dans le test.
	var typeDemande, destinataire string
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		"SELECT type_demande, operateur_destinataire_id FROM demande WHERE id = $1", id).
		Scan(&typeDemande, &destinataire))

	pr := entity.PortingRequest{
		RequestType:         entity.RequestType(typeDemande),
		RecipientOperatorID: destinataire,
	}
	expected := entity.ExpectedConfirmers(pr, []string{operateurOrange, operateurYAS, operateurExpresso})
	require.Len(t, expected, 3, "une RESTITUTION exige les trois opérateurs, destinataire compris")

	// Toutes les confirmations sauf la dernière : l'étape ne doit pas avancer.
	for _, op := range expected[:len(expected)-1] {
		compte := comptes[op]
		rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
			h.Jeton(compte[0], compte[1]), map[string]any{"idDemande": id})
		require.Equal(t, http.StatusOK, rep.StatusCode, corps)
		require.Equal(t, "CONFIRMATION", etape(h, id),
			"il manque encore une confirmation : l'étape ne doit pas avoir avancé")
	}

	// La dernière confirmation franchit exactement count >= len(expected).
	dernier := comptes[expected[len(expected)-1]]
	rep, corps = h.Appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Jeton(dernier[0], dernier[1]), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusOK, rep.StatusCode, corps)
	require.Equal(t, "COMPLETION", etape(h, id),
		"la dernière confirmation solde l'étape : le seuil est franchi exactement")
}
