package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/config"
	"github.com/stretchr/testify/require"
)

func TestConfirmationParTousSaufLeDestinataire(t *testing.T) {
	// Mesuré au SIT : ORANGE confirme, l'étape reste EN_COURS ; EXPRESSO la solde.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "CONFIRMATION")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.jeton("orange", "orange2026"),
		map[string]any{"idDemande": id, "commentaire": "Portage confirmé"})

	require.Equal(t, http.StatusOK, rep.StatusCode)
	data := corps["data"].(map[string]any)
	require.Equal(t, "CONFIRMATION", data["etapeActuelle"])
	require.Equal(t, "EN_COURS", data["statutEtapeActuel"])

	var prevue *string
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT transition_prevue_a::text FROM demande WHERE id = $1", id).Scan(&prevue))
	require.Nil(t, prevue)
	require.Equal(t, "CONFIRMATION", h.etape(id), "il manque la confirmation d'EXPRESSO")

	rep, _ = h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.jeton("expresso", "expresso2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	require.Equal(t, "COMPLETION", h.etape(id),
		"la dernière confirmation solde l'étape dans la requête")
}

func TestConfirmationParLeDestinataireRefusee(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "CONFIRMATION")

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.jeton("yas", "yas2026"), map[string]any{"idDemande": id})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestDoubleConfirmationRefusee(t *testing.T) {
	// TC-041 : anti-rejeu — refusé, en HTTP 500.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "CONFIRMATION")

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Contains(t, corps["detail"], "déjà confirmé")
}

func TestConfirmationHorsEtapeConfirmation(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Contains(t, corps["detail"], "ACCEPTATION")
}

func TestRestitutionExigeLaConfirmationDuDestinataire(t *testing.T) {
	h := nouveauHarnais(t)
	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})
	id := corps["data"].(map[string]any)["id"].(string)
	h.avancerA(id, "CONFIRMATION")

	// ORANGE est destinataire de la restitution et doit néanmoins confirmer.
	data := h.liste("/api/gateway/v1/demandes/a-confirmer", h.jeton("orange", "orange2026"))
	require.Len(t, data, 1)

	for _, compte := range [][2]string{
		{"orange", "orange2026"}, {"yas", "yas2026"}, {"expresso", "expresso2026"},
	} {
		rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
			h.jeton(compte[0], compte[1]), map[string]any{"idDemande": id})
		require.Equalf(t, http.StatusOK, rep.StatusCode, compte[0])
	}

	require.Equal(t, "COMPLETION", h.etape(id),
		"tous ont confirmé, destinataire compris : l'étape est soldée")
}

func TestDejaConfirmeesNeTracePasLaSourceEnModeReel(t *testing.T) {
	// ANO-019 : ORANGE confirme avec succès, sa liste renvoie 0.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "CONFIRMATION")

	h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})
	h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.jeton("expresso", "expresso2026"), map[string]any{"idDemande": id})

	require.Empty(t, h.liste("/api/gateway/v1/demandes/deja-confirmees",
		h.jeton("orange", "orange2026")), "la source n'est pas tracée (ANO-019)")
	require.Len(t, h.liste("/api/gateway/v1/demandes/deja-confirmees",
		h.jeton("expresso", "expresso2026")), 1, "le tiers l'est")
}

func TestDejaConfirmeesTraceLaSourceEnModeContrat(t *testing.T) {
	h := nouveauHarnais(t, func(c *config.Config) { c.Fidelity = config.FidelityContract })
	id := h.creerPortage("771000001")
	h.avancerA(id, "CONFIRMATION")

	h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})

	require.Len(t, h.liste("/api/gateway/v1/demandes/deja-confirmees",
		h.jeton("orange", "orange2026")), 1)
}
