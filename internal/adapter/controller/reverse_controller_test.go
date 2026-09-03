package controller_test

// These 6 test functions are moved, unchanged in assertion, from the
// deleted internal/api/reverse_test.go (Task 16). They still exercise the
// real, live router — routerharness.NewRouterHarness wraps api.NewRouter,
// wired exactly as cmd/server/main.go wires it — so a green run here proves
// a real HTTP request to /reverse-requests goes through the new
// ReverseController, its two interactors (internal/usecase/reverse) and
// port.UnitOfWork, not through any leftover handler.

import (
	"context"
	"net/http"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
	"github.com/stretchr/testify/require"
)

// requestStatus reads a request's status directly from the database.
func requestStatus(h *routerharness.RouterHarness, id string) string {
	h.T.Helper()
	var s string
	require.NoError(h.T, h.DB.Pool.QueryRow(context.Background(),
		"SELECT statut_demande FROM demande WHERE id = $1", id).Scan(&s))
	return s
}

func TestSubmitReverseByOriginOperator(t *testing.T) {
	// Slice 773: YAS currently, ORANGE at origin.
	h := routerharness.NewRouterHarness(t)
	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.Token("orange", "orange2026"), map[string]any{"numero": "789001001"})

	require.Equal(t, http.StatusCreated, resp.StatusCode, body)
	require.Equal(t, "Demande de reverse soumise avec succès", body["message"])

	data := body["data"].(map[string]any)
	require.Equal(t, "789001001", data["numero"])
	require.Equal(t, "EN_ATTENTE", data["statut"])
	require.Equal(t, operatorOrange, data["operateur"].(map[string]any)["id"])
}

func TestSubmitReverseByAnotherOperatorRefused(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.Token("yas", "yas2026"), map[string]any{"numero": "789001001"})
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestOwnReverseRequests(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	h.Call(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.Token("orange", "orange2026"), map[string]any{"numero": "789001001"})

	data := h.List("/api/gateway/v1/reverse-requests/mes-demandes",
		h.Token("orange", "orange2026"))
	require.Len(t, data, 1)
	require.Equal(t, "EN_ATTENTE", data[0].(map[string]any)["statut"])

	require.Empty(t, h.List("/api/gateway/v1/reverse-requests/mes-demandes",
		h.Token("yas", "yas2026")))
}

func TestNoCancelEndpointForReverse(t *testing.T) {
	// §7.6: "There is no endpoint to cancel a reverse request."
	h := routerharness.NewRouterHarness(t)
	_, body := h.Call(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.Token("orange", "orange2026"), map[string]any{"numero": "789001001"})
	id := body["data"].(map[string]any)["id"].(string)

	resp := h.Raw(http.MethodPost, "/api/gateway/v1/reverse-requests/"+id+"/annuler",
		h.Token("orange", "orange2026"), nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestReverseReachesCompletedThroughRealEndpoints proves the complete flow
// by going through the real endpoints — /reverse-requests then
// /demandes/a-confirmer, as a real operator would — rather than inserting
// confirmations directly in SQL. postAConfirmer is agnostic of the request
// type: when the last confirmation lands, it schedules a generic
// transition (ScheduleTransition). On the next tick,
// appliquerConvergencesDues advances the request from CONFIRMATION to
// COMPLETION through that common path, before completerReversesConfirmes
// runs — which must therefore know how to catch up a REVERSE request
// already at COMPLETION, or else it stays stuck there forever, since no
// operator can process the COMPLETION of a REVERSE.
func TestReverseReachesCompletedThroughRealEndpoints(t *testing.T) {
	h := routerharness.NewRouterHarness(t)

	// 1. Submission by the origin operator (slice 773: YAS currently,
	// ORANGE at origin).
	_, body := h.Call(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.Token("orange", "orange2026"), map[string]any{"numero": "789001001"})
	reverseID := body["data"].(map[string]any)["id"].(string)

	// 2. An ARTP act, outside the API: validation — creates the REVERSE
	// request at CONFIRMATION.
	h.ValidateReverse(reverseID)

	var requestID string
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		`SELECT demande_id FROM reverse_request WHERE id = $1`, reverseID).Scan(&requestID))

	// 3. Every operator confirms via the real endpoint — recipient
	// (the number's origin operator) included, as a REVERSE requires.
	for _, account := range [][2]string{
		{"orange", "orange2026"}, {"yas", "yas2026"}, {"expresso", "expresso2026"},
	} {
		resp, corpsConf := h.Call(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
			h.Token(account[0], account[1]), map[string]any{"idDemande": requestID})
		require.Equal(t, http.StatusOK, resp.StatusCode, corpsConf)
	}

	// 4. Engine convergence: the request must reach TERMINE, and the
	// number must be back with its origin operator in the registry.
	h.Converge()
	h.Converge()

	require.Equal(t, "TERMINE", requestStatus(h, requestID))

	var currentOperator string
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		`SELECT operateur_actuel_id FROM numero WHERE msisdn = '789001001'`).
		Scan(&currentOperator))
	require.Equal(t, operatorOrange, currentOperator)
}

// TestReverseCompletionAlwaysRefusedToOperators — guide §7.9: an attempt
// at COMPLETION on a REVERSE returns DEMANDE_ACCES_REFUSE with the
// documented message. The engine plays the ARTP's part and completes the
// request as soon as every confirmation is in; if the refusal only held
// while the request is EN_COURS, the window would be one tick and the
// documented message would become unreachable in practice — the operator
// would receive a generic ETAPE_INVALIDE instead.
func TestReverseCompletionAlwaysRefusedToOperators(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.ContractFidelity)

	_, body := h.Call(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.Token("orange", "orange2026"), map[string]any{"numero": "789001001"})
	reverseID := body["data"].(map[string]any)["id"].(string)
	h.ValidateReverse(reverseID)

	var requestID string
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		`SELECT demande_id FROM reverse_request WHERE id = $1`, reverseID).Scan(&requestID))

	for _, account := range [][2]string{
		{"orange", "orange2026"}, {"yas", "yas2026"}, {"expresso", "expresso2026"},
	} {
		h.Call(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
			h.Token(account[0], account[1]), map[string]any{"idDemande": requestID})
	}
	h.Converge()
	h.Converge()
	require.Equal(t, "COMPLETION", step(h, requestID))

	// Recipient like source: the refusal is the same, and it is the guide's.
	for _, account := range [][2]string{{"orange", "orange2026"}, {"yas", "yas2026"}} {
		resp, corpsRefus := h.Call(http.MethodPost, "/api/gateway/v1/demandes/traitement",
			h.Token(account[0], account[1]), map[string]any{"idDemande": requestID})

		require.Equal(t, http.StatusForbidden, resp.StatusCode, account[0])
		require.Equal(t, "DEMANDE_ACCES_REFUSE", corpsRefus["code"], account[0])
		require.Equal(t,
			"La complétion (COMPLETION) d'une demande REVERSE est réservée à l'ARTP, "+
				"une fois que tous les opérateurs ont confirmé.",
			corpsRefus["message"], account[0])
	}
}
