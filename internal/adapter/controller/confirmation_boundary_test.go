//go:build integration

package controller_test

// TestConfirmationCrossesTheThresholdExactly exists because Task 15's
// completion boundary — ConfirmRequestInteractor.Execute's
// count >= len(expected) — is business-critical (it decides when a step
// advances) and was, until this file, only exercised against
// testsupport/inmemory's map-backed Count, never against a real Postgres
// COUNT(*). One test that crosses the boundary is worth more than two that
// sit on either side of it, so this walks a request to exactly one
// confirmation short of the threshold, asserts CONFIRMATION has NOT been
// settled, then adds the final confirmation and asserts it HAS.

import (
	"context"
	"net/http"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
	"github.com/stretchr/testify/require"
)

// operatorExpresso is internal/framework/seed.OperatorExpressoID, copied as a
// literal: a controller test (adapter layer) cannot import
// internal/framework (dependency rule, see test/architecture_test.go), even
// in a //go:build integration file — a precedent already set by
// creation_restitution_test.go (operatorOrange, operatorYAS alongside it)
// and user_gateway_test.go.
const operatorExpresso = "6a217510e6c37b5b5b487ec7"

// accounts maps a seed operator id to its login account, to drive
// confirmations in the order entity.ExpectedConfirmers expects.
var accounts = map[string][2]string{
	operatorOrange:   {"orange", "orange2026"},
	operatorYAS:      {"yas", "yas2026"},
	operatorExpresso: {"expresso", "expresso2026"},
}

func TestConfirmationCrossesTheThresholdExactly(t *testing.T) {
	h := routerharness.NewRouterHarness(t)

	// A RESTITUTION requires everyone, recipient included (unlike a
	// PORTAGE) — the case where the threshold count is the least trivial.
	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.Token("orange", "orange2026"), map[string]any{"numero": "789001001"})
	require.Equal(t, http.StatusCreated, resp.StatusCode, body)
	id := body["data"].(map[string]any)["id"].(string)
	advanceTo(h, id, "CONFIRMATION")

	// entity.ExpectedConfirmers is the sole authority on who must confirm:
	// it is derived here from the created request's real type and
	// recipient, read back from the database, rather than restating the
	// rule inside the test.
	var requestType, recipient string
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		"SELECT type_demande, operateur_destinataire_id FROM demande WHERE id = $1", id).
		Scan(&requestType, &recipient))

	pr := entity.PortingRequest{
		RequestType:         entity.RequestType(requestType),
		RecipientOperatorID: recipient,
	}
	expected := entity.ExpectedConfirmers(pr, []string{operatorOrange, operatorYAS, operatorExpresso})
	require.Len(t, expected, 3, "a RESTITUTION requires all three operators, recipient included")

	// Every confirmation except the last: the step must not advance.
	for _, op := range expected[:len(expected)-1] {
		account := accounts[op]
		resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
			h.Token(account[0], account[1]), map[string]any{"idDemande": id})
		require.Equal(t, http.StatusOK, resp.StatusCode, body)
		require.Equal(t, "CONFIRMATION", step(h, id),
			"one confirmation is still missing: the step must not have advanced")
	}

	// The last confirmation crosses count >= len(expected) exactly.
	last := accounts[expected[len(expected)-1]]
	resp, body = h.Call(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Token(last[0], last[1]), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	require.Equal(t, "COMPLETION", step(h, id),
		"the last confirmation settles the step: the threshold is crossed exactly")
}
