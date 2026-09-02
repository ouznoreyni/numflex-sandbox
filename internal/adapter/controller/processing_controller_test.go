package controller_test

// These 9 test functions are moved, unchanged in assertion, from the
// deleted internal/api/traitement_test.go (Task 15). They still exercise
// the real, live router — routerharness.NewRouterHarness wraps
// api.NewRouter, wired exactly as cmd/server/main.go wires it — so a green
// run here proves a real HTTP request to /demandes/traitement goes through
// the new PortingController, ProcessStepInteractor (which itself delegates
// every authorization rule to entity.CanProcess) and port.UnitOfWork, not
// through any leftover handler.

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
	"github.com/stretchr/testify/require"
)

func TestProcessDeactivationBySource(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")
	advanceTo(h, id, "DESACTIVATION")

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.Token("orange", "orange2026"),
		map[string]any{"idDemande": id, "commentaire": "Numéro désactivé"})

	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	require.Equal(t, "Étape traitée avec succès", body["message"])
	// The response carries the next step: capture
	// « 1.orange_3_DESACTIVATION…_next_ACTIVATION ».
	require.Equal(t, "ACTIVATION", body["data"].(map[string]any)["etapeActuelle"])
}

func TestProcessByTheWrongOperator(t *testing.T) {
	// TC-036 in principle: the step is not the caller's responsibility.
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")
	advanceTo(h, id, "DESACTIVATION")

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.Token("yas", "yas2026"), map[string]any{"idDemande": id})

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Contains(t, body["detail"], "source")
	require.NotContains(t, body, "code")
}

func TestProcessRefusesAcceptanceAndConfirmation(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.Token("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t,
		"RuntimeException: L'étape ACCEPTATION se traite via POST /api/gateway/v1/demandes/acceptation.",
		body["detail"])

	advanceTo(h, id, "CONFIRMATION")
	resp, body = h.Call(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.Token("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t,
		"RuntimeException: L'étape CONFIRMATION se traite via POST /api/gateway/v1/demandes/a-confirmer.",
		body["detail"])
}

func TestProcessInContractModeRendersInvalidStepAs409(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.ContractFidelity)
	id := createPorting(h, "771000001")
	advanceTo(h, id, "CONFIRMATION")

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.Token("orange", "orange2026"), map[string]any{"idDemande": id})

	require.Equal(t, http.StatusConflict, resp.StatusCode)
	require.Equal(t, "ETAPE_INVALIDE", body["code"])
	require.Equal(t,
		"L'étape CONFIRMATION se traite via POST /api/gateway/v1/demandes/a-confirmer.",
		body["message"])
}

func TestStepFieldAcceptedAndIgnoredSilently(t *testing.T) {
	// ANO-018: a v1 integration that has not migrated does not fail — it
	// silently executes the current step, whatever it is.
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")
	advanceTo(h, id, "DESACTIVATION")

	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.Token("orange", "orange2026"),
		map[string]any{"idDemande": id, "etape": "CONFIRMATION"})

	require.Equal(t, http.StatusOK, resp.StatusCode, "neither rejected nor warned")

	// The history row is written by the engine at transition time, not by
	// the handler — so convergence must run before reading it (R-10).
	h.Converge()

	var gotStep, gotStatus string
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		`SELECT etape, statut FROM etape_historique WHERE demande_id = $1`, id).
		Scan(&gotStep, &gotStatus))
	require.Equal(t, "DESACTIVATION", gotStep, "it is the current step that was executed")
}

func TestSecondProcessDuringConvergenceRefused(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.Convergence(time.Hour, time.Hour))
	id := createPorting(h, "771000001")
	advanceTo(h, id, "DESACTIVATION")
	token := h.Token("orange", "orange2026")

	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/demandes/traitement", token,
		map[string]any{"idDemande": id})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp, _ = h.Call(http.MethodPost, "/api/gateway/v1/demandes/traitement", token,
		map[string]any{"idDemande": id})
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestReverseCompletionReservedToARTP(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")
	_, err := h.DB.Pool.Exec(context.Background(),
		`UPDATE demande SET type_demande = 'REVERSE', etape_actuelle = 'COMPLETION',
		                    date_debut_etape = now() WHERE id = $1`, id)
	require.NoError(t, err)

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.Token("yas", "yas2026"), map[string]any{"idDemande": id})

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t,
		"RuntimeException: La complétion (COMPLETION) d'une demande REVERSE est réservée à l'ARTP, une fois que tous les opérateurs ont confirmé.",
		body["detail"])
}

func TestCompletionLatency(t *testing.T) {
	// ANO-005: COMPLETION answers in ~30s. Reduced here to 300ms for the test.
	h := routerharness.NewRouterHarness(t, routerharness.CompletionLatency(300*time.Millisecond))
	id := createPorting(h, "771000001")
	advanceTo(h, id, "COMPLETION")

	start := time.Now()
	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.Token("yas", "yas2026"), map[string]any{"idDemande": id})
	elapsed := time.Since(start)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.GreaterOrEqual(t, elapsed, 300*time.Millisecond)
}

func TestNoIdempotencyKeyIsRead(t *testing.T) {
	// ANO-005: NumFlex accepts no Idempotency-Key; a replay falls on a
	// state error, indistinguishable from a failure.
	h := routerharness.NewRouterHarness(t, routerharness.Convergence(time.Hour, time.Hour))
	id := createPorting(h, "771000001")
	advanceTo(h, id, "DESACTIVATION")

	req := map[string]any{"idDemande": id}
	token := h.Token("orange", "orange2026")
	h.Call(http.MethodPost, "/api/gateway/v1/demandes/traitement", token, req)

	resp := h.Raw(http.MethodPost, "/api/gateway/v1/demandes/traitement", token, req)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
