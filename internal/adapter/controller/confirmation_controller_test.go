package controller_test

// These 7 test functions are moved, unchanged in assertion, from the
// deleted internal/api/confirmation_test.go (Task 15). They still exercise
// the real, live router — routerharness.NewRouterHarness wraps
// api.NewRouter, wired exactly as cmd/server/main.go wires it — so a green
// run here proves a real HTTP request to /demandes/a-confirmer goes through
// the new PortingController, ConfirmRequestInteractor and port.UnitOfWork,
// not through any leftover handler.

import (
	"context"
	"net/http"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
	"github.com/stretchr/testify/require"
)

// step reads a request's current step directly from the database — the
// processing endpoints are tested elsewhere.
func step(h *routerharness.RouterHarness, id string) string {
	h.T.Helper()
	var e string
	require.NoError(h.T, h.DB.Pool.QueryRow(context.Background(),
		"SELECT etape_actuelle FROM demande WHERE id = $1", id).Scan(&e))
	return e
}

func TestConfirmationByAllExceptRecipient(t *testing.T) {
	// Measured at the SIT: ORANGE confirms, the step stays EN_COURS; EXPRESSO settles it.
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")
	advanceTo(h, id, "CONFIRMATION")

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Token("orange", "orange2026"),
		map[string]any{"idDemande": id, "commentaire": "Portage confirmé"})

	require.Equal(t, http.StatusOK, resp.StatusCode)
	data := body["data"].(map[string]any)
	require.Equal(t, "CONFIRMATION", data["etapeActuelle"])
	require.Equal(t, "EN_COURS", data["statutEtapeActuel"])

	var prevue *string
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		"SELECT transition_prevue_a::text FROM demande WHERE id = $1", id).Scan(&prevue))
	require.Nil(t, prevue)
	require.Equal(t, "CONFIRMATION", step(h, id), "EXPRESSO's confirmation is missing")

	resp, _ = h.Call(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Token("expresso", "expresso2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Equal(t, "COMPLETION", step(h, id),
		"the last confirmation settles the step within the request")
}

func TestConfirmationByRecipientRefused(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")
	advanceTo(h, id, "CONFIRMATION")

	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Token("yas", "yas2026"), map[string]any{"idDemande": id})

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestDoubleConfirmationRefused(t *testing.T) {
	// TC-041: anti-replay — refused, as HTTP 500.
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")
	advanceTo(h, id, "CONFIRMATION")

	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Token("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Token("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Contains(t, body["detail"], "déjà confirmé")
}

func TestConfirmationOutsideConfirmationStep(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Token("orange", "orange2026"), map[string]any{"idDemande": id})

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Contains(t, body["detail"], "ACCEPTATION")
}

func TestRestitutionRequiresRecipientConfirmation(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	_, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.Token("orange", "orange2026"), map[string]any{"numero": "789001001"})
	id := body["data"].(map[string]any)["id"].(string)
	advanceTo(h, id, "CONFIRMATION")

	// ORANGE is the restitution's recipient and must nonetheless confirm.
	data := h.List("/api/gateway/v1/demandes/a-confirmer", h.Token("orange", "orange2026"))
	require.Len(t, data, 1)

	for _, count := range [][2]string{
		{"orange", "orange2026"}, {"yas", "yas2026"}, {"expresso", "expresso2026"},
	} {
		resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
			h.Token(count[0], count[1]), map[string]any{"idDemande": id})
		require.Equalf(t, http.StatusOK, resp.StatusCode, count[0])
	}

	require.Equal(t, "COMPLETION", step(h, id),
		"everyone confirmed, recipient included: the step is settled")
}

func TestAlreadyConfirmedDoesNotTraceSourceInRealMode(t *testing.T) {
	// ANO-019: ORANGE confirms successfully, its list returns 0.
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")
	advanceTo(h, id, "CONFIRMATION")

	h.Call(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Token("orange", "orange2026"), map[string]any{"idDemande": id})
	h.Call(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Token("expresso", "expresso2026"), map[string]any{"idDemande": id})

	require.Empty(t, h.List("/api/gateway/v1/demandes/deja-confirmees",
		h.Token("orange", "orange2026")), "the source is not traced (ANO-019)")
	require.Len(t, h.List("/api/gateway/v1/demandes/deja-confirmees",
		h.Token("expresso", "expresso2026")), 1, "the third party is")
}

func TestAlreadyConfirmedTracesSourceInContractMode(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.ContractFidelity)
	id := createPorting(h, "771000001")
	advanceTo(h, id, "CONFIRMATION")

	h.Call(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Token("orange", "orange2026"), map[string]any{"idDemande": id})

	require.Len(t, h.List("/api/gateway/v1/demandes/deja-confirmees",
		h.Token("orange", "orange2026")), 1)
}
