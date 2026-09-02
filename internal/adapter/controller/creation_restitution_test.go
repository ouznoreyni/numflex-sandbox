package controller_test

// These 6 test functions are moved, unchanged in assertion, from the
// deleted internal/api/creation_restitution_test.go (Task 12).

import (
	"net/http"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
	"github.com/stretchr/testify/require"
)

func TestRestitutionNominal(t *testing.T) {
	// Slice 773: YAS, ported from ORANGE 240 days ago.
	h := routerharness.NewRouterHarness(t)
	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.Token("orange", "orange2026"), map[string]any{"numero": "773000001"})

	require.Equal(t, http.StatusCreated, resp.StatusCode, body)
	data := body["data"].(map[string]any)
	require.Equal(t, "RESTITUTION", data["typeDemande"])
	require.Equal(t, "ACCEPTATION", data["etapeActuelle"])

	// The origin operator recovers the number: it is the recipient.
	require.Equal(t, operatorOrange,
		data["operateurDestinataire"].(map[string]any)["id"])
	require.Equal(t, operatorYAS,
		data["operateurSource"].(map[string]any)["id"])

	// routageInfo only exists from COMPLETION onward (§7.10).
	require.Nil(t, data["routageInfo"])
}

func TestRestitutionNumberNeverPorted(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.Token("orange", "orange2026"), map[string]any{"numero": "771000001"})

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t,
		"RuntimeException: Le numéro n'a pas été porté, pas de restitution/reverse possible",
		body["detail"])
}

func TestRestitutionTooSoonIsA500WrappingA400(t *testing.T) {
	// ANO-020.
	h := routerharness.NewRouterHarness(t)
	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.Token("orange", "orange2026"), map[string]any{"numero": "774000001"})

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Contains(t, body["detail"], "error.numeroRestitutionTooEarly")
	require.NotContains(t, body, "code")
}

func TestRestitutionAlreadyRestituted(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.Token("orange", "orange2026"), map[string]any{"numero": "775000001"})

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "RuntimeException: Ce numéro a déjà été restitué", body["detail"])
}

func TestRestitutionReservedToTheOriginOperator(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	// EXPRESSO is neither the holder nor the origin operator of 773000001.
	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.Token("expresso", "expresso2026"), map[string]any{"numero": "773000001"})

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestRestitutionWithoutOTP(t *testing.T) {
	// §7.5: the body carries only the number, no OTP is required.
	h := routerharness.NewRouterHarness(t)
	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.Token("orange", "orange2026"), map[string]any{"numero": "773000002"})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
}
