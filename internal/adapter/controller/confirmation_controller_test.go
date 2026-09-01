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

// etape lit l'étape actuelle d'une demande directement en base — les
// endpoints de traitement sont testés ailleurs.
func etape(h *routerharness.RouterHarness, id string) string {
	h.T.Helper()
	var e string
	require.NoError(h.T, h.DB.Pool.QueryRow(context.Background(),
		"SELECT etape_actuelle FROM demande WHERE id = $1", id).Scan(&e))
	return e
}

func TestConfirmationParTousSaufLeDestinataire(t *testing.T) {
	// Mesuré au SIT : ORANGE confirme, l'étape reste EN_COURS ; EXPRESSO la solde.
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")
	avancerA(h, id, "CONFIRMATION")

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Jeton("orange", "orange2026"),
		map[string]any{"idDemande": id, "commentaire": "Portage confirmé"})

	require.Equal(t, http.StatusOK, rep.StatusCode)
	data := corps["data"].(map[string]any)
	require.Equal(t, "CONFIRMATION", data["etapeActuelle"])
	require.Equal(t, "EN_COURS", data["statutEtapeActuel"])

	var prevue *string
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		"SELECT transition_prevue_a::text FROM demande WHERE id = $1", id).Scan(&prevue))
	require.Nil(t, prevue)
	require.Equal(t, "CONFIRMATION", etape(h, id), "il manque la confirmation d'EXPRESSO")

	rep, _ = h.Appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Jeton("expresso", "expresso2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	require.Equal(t, "COMPLETION", etape(h, id),
		"la dernière confirmation solde l'étape dans la requête")
}

func TestConfirmationParLeDestinataireRefusee(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")
	avancerA(h, id, "CONFIRMATION")

	rep, _ := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Jeton("yas", "yas2026"), map[string]any{"idDemande": id})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestDoubleConfirmationRefusee(t *testing.T) {
	// TC-041 : anti-rejeu — refusé, en HTTP 500.
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")
	avancerA(h, id, "CONFIRMATION")

	rep, _ := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Jeton("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Jeton("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Contains(t, corps["detail"], "déjà confirmé")
}

func TestConfirmationHorsEtapeConfirmation(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Jeton("orange", "orange2026"), map[string]any{"idDemande": id})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Contains(t, corps["detail"], "ACCEPTATION")
}

func TestRestitutionExigeLaConfirmationDuDestinataire(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	_, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.Jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})
	id := corps["data"].(map[string]any)["id"].(string)
	avancerA(h, id, "CONFIRMATION")

	// ORANGE est destinataire de la restitution et doit néanmoins confirmer.
	data := h.Liste("/api/gateway/v1/demandes/a-confirmer", h.Jeton("orange", "orange2026"))
	require.Len(t, data, 1)

	for _, compte := range [][2]string{
		{"orange", "orange2026"}, {"yas", "yas2026"}, {"expresso", "expresso2026"},
	} {
		rep, _ := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
			h.Jeton(compte[0], compte[1]), map[string]any{"idDemande": id})
		require.Equalf(t, http.StatusOK, rep.StatusCode, compte[0])
	}

	require.Equal(t, "COMPLETION", etape(h, id),
		"tous ont confirmé, destinataire compris : l'étape est soldée")
}

func TestDejaConfirmeesNeTracePasLaSourceEnModeReel(t *testing.T) {
	// ANO-019 : ORANGE confirme avec succès, sa liste renvoie 0.
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")
	avancerA(h, id, "CONFIRMATION")

	h.Appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Jeton("orange", "orange2026"), map[string]any{"idDemande": id})
	h.Appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Jeton("expresso", "expresso2026"), map[string]any{"idDemande": id})

	require.Empty(t, h.Liste("/api/gateway/v1/demandes/deja-confirmees",
		h.Jeton("orange", "orange2026")), "la source n'est pas tracée (ANO-019)")
	require.Len(t, h.Liste("/api/gateway/v1/demandes/deja-confirmees",
		h.Jeton("expresso", "expresso2026")), 1, "le tiers l'est")
}

func TestDejaConfirmeesTraceLaSourceEnModeContrat(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.FiabiliteContrat)
	id := creerPortage(h, "771000001")
	avancerA(h, id, "CONFIRMATION")

	h.Appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.Jeton("orange", "orange2026"), map[string]any{"idDemande": id})

	require.Len(t, h.Liste("/api/gateway/v1/demandes/deja-confirmees",
		h.Jeton("orange", "orange2026")), 1)
}
