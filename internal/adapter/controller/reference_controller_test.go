package controller_test

// These 6 test functions are moved, unchanged in assertion, from the
// deleted internal/api/referentiels_test.go (Task 6, then Task 11). They
// still exercise the real, live router — routerharness.NewRouterHarness
// wraps api.NewRouter, wired exactly as cmd/server/main.go wires it — so a
// green run here proves a real HTTP request to one of the five reference
// routes goes through the new ReferenceController and not through any
// leftover handler.

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
)

func TestOperatorsReturnsTheThreeAcceptanceIDs(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	data := h.List("/api/gateway/v1/operateurs", h.Token("yas", "yas2026"))

	require.Len(t, data, 3)
	seen := map[string]string{}
	for _, e := range data {
		m := e.(map[string]any)
		require.Len(t, m, 2, "an operator carries only id and nom")
		seen[m["id"].(string)] = m["nom"].(string)
	}
	require.Equal(t, map[string]string{
		"6a21745ce6c37b5b5b487ec1": "ORANGE",
		"6a2174c3e6c37b5b5b487ec4": "YAS",
		"6a217510e6c37b5b5b487ec7": "EXPRESSO",
	}, seen)
}

func TestOperatorsExactMessage(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	_, body := h.Call(http.MethodGet, "/api/gateway/v1/operateurs", h.Token("yas", "yas2026"), nil)
	require.Equal(t, "Opérateurs récupérés avec succès", body["message"])
	require.Equal(t, "SUCCESS", body["code"])
	require.Equal(t, true, body["success"])
}

func TestRejectionReasonsExposeMotifFieldNotLibelle(t *testing.T) {
	// ANO-009: the field is called motif. v2 documents it this way.
	h := routerharness.NewRouterHarness(t)
	data := h.List("/api/gateway/v1/motifs-rejet", h.Token("orange", "orange2026"))

	require.Len(t, data, 6)
	first := data[0].(map[string]any)
	require.Contains(t, first, "motif")
	require.NotContains(t, first, "libelle")
}

func TestRequestTypes(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	data := h.List("/api/gateway/v1/types-demande", h.Token("yas", "yas2026"))

	types := []string{}
	for _, e := range data {
		types = append(types, e.(map[string]any)["type"].(string))
	}
	require.ElementsMatch(t, []string{"PORTAGE", "RESTITUTION", "REVERSE"}, types)
}

func TestProcesses(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	data := h.List("/api/gateway/v1/processus", h.Token("yas", "yas2026"))

	types := []string{}
	for _, e := range data {
		types = append(types, e.(map[string]any)["type"].(string))
	}
	require.ElementsMatch(t, []string{"PREPAID", "POSTPAID"}, types)
}

func TestIncidentTypesCarrySystemLocked(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	data := h.List("/api/gateway/v1/types-incident", h.Token("yas", "yas2026"))

	require.Len(t, data, 2)
	byLabel := map[string]bool{}
	for _, e := range data {
		m := e.(map[string]any)
		byLabel[m["libelle"].(string)] = m["figeSysteme"].(bool)
	}
	require.Equal(t, map[string]bool{"Gateway": false, "Technique": true}, byLabel)
}
