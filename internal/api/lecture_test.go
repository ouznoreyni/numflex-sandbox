package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// avancerA fait progresser une demande jusqu'à l'étape voulue en manipulant
// directement la base — les endpoints de traitement sont testés ailleurs.
func (h *harnais) avancerA(id, etape string) {
	h.t.Helper()
	_, err := h.db.Pool.Exec(context.Background(),
		`UPDATE demande SET etape_actuelle = $2, statut_etape_actuel = 'EN_COURS',
		                    date_debut_etape = now(), transition_prevue_a = NULL
		  WHERE id = $1`, id, etape)
	require.NoError(h.t, err)
}

func TestMesDemandesVoitSourceEtDestinataire(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	for _, compte := range [][2]string{{"yas", "yas2026"}, {"orange", "orange2026"}} {
		data := h.liste("/api/gateway/v1/demandes/mes-demandes", h.jeton(compte[0], compte[1]))
		require.Len(t, data, 1, compte[0])
		require.Equal(t, id, data[0].(map[string]any)["id"])
	}

	// EXPRESSO n'est partie à rien.
	require.Empty(t, h.liste("/api/gateway/v1/demandes/mes-demandes",
		h.jeton("expresso", "expresso2026")))
}

func TestMesDemandesNAcceptePasDePagination(t *testing.T) {
	h := nouveauHarnais(t)
	h.creerPortage("771000001")

	_, corps := h.appel(http.MethodGet, "/api/gateway/v1/demandes/mes-demandes",
		h.jeton("yas", "yas2026"), nil)

	require.NotContains(t, corps, "page")
	require.NotContains(t, corps, "size")
	require.NotContains(t, corps, "totalElements")
}

func TestAAccepterEstReserveeALaSource(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	data := h.liste("/api/gateway/v1/demandes/a-accepter", h.jeton("orange", "orange2026"))
	require.Len(t, data, 1)
	require.Equal(t, id, data[0].(map[string]any)["id"])

	require.Empty(t, h.liste("/api/gateway/v1/demandes/a-accepter", h.jeton("yas", "yas2026")))
}

func TestATraiterSuitLeResponsableDeLEtape(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	h.avancerA(id, "DESACTIVATION")
	require.Len(t, h.liste("/api/gateway/v1/demandes/a-traiter", h.jeton("orange", "orange2026")), 1)
	require.Empty(t, h.liste("/api/gateway/v1/demandes/a-traiter", h.jeton("yas", "yas2026")))

	h.avancerA(id, "ACTIVATION")
	require.Len(t, h.liste("/api/gateway/v1/demandes/a-traiter", h.jeton("yas", "yas2026")), 1)
	require.Empty(t, h.liste("/api/gateway/v1/demandes/a-traiter", h.jeton("orange", "orange2026")))
}

func TestAConfirmerContientLeTiers(t *testing.T) {
	// D-6, mesuré au SIT : EXPRESSO, ni source ni destinataire, doit confirmer.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "CONFIRMATION")

	require.Len(t, h.liste("/api/gateway/v1/demandes/a-confirmer",
		h.jeton("orange", "orange2026")), 1)
	require.Len(t, h.liste("/api/gateway/v1/demandes/a-confirmer",
		h.jeton("expresso", "expresso2026")), 1)

	// Le destinataire est auto-confirmé : la demande ne figure pas dans sa file.
	require.Empty(t, h.liste("/api/gateway/v1/demandes/a-confirmer",
		h.jeton("yas", "yas2026")))
}

func TestDetailAConfirmerRefuseAuDestinataire(t *testing.T) {
	// Mesuré : GET /a-confirmer/{id} avec le jeton du destinataire répond 500.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "CONFIRMATION")

	rep, _ := h.appel(http.MethodGet, "/api/gateway/v1/demandes/a-confirmer/"+id,
		h.jeton("yas", "yas2026"), nil)
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)

	rep, _ = h.appel(http.MethodGet, "/api/gateway/v1/demandes/a-confirmer/"+id,
		h.jeton("orange", "orange2026"), nil)
	require.Equal(t, http.StatusOK, rep.StatusCode)
}

func TestDetailDemandeInconnue(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodGet,
		"/api/gateway/v1/demandes/a-traiter/6a0000000000000000000000",
		h.jeton("yas", "yas2026"), nil)

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "RuntimeException: Demande introuvable", corps["detail"])
}

func TestInEtOutNeContiennentQueLesPortagesTermines(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	require.Empty(t, h.liste("/api/gateway/v1/demandes/in", h.jeton("yas", "yas2026")))

	_, err := h.db.Pool.Exec(context.Background(),
		`UPDATE demande SET statut_demande = 'TERMINE', etape_actuelle = 'COMPLETION',
		                    statut_etape_actuel = 'VALIDE', date_finalisation = now()
		  WHERE id = $1`, id)
	require.NoError(t, err)

	data := h.liste("/api/gateway/v1/demandes/in", h.jeton("yas", "yas2026"))
	require.Len(t, data, 1)
	require.Equal(t, "TERMINE", data[0].(map[string]any)["statutDemande"])
	require.NotNil(t, data[0].(map[string]any)["dateFinalisation"])

	require.Len(t, h.liste("/api/gateway/v1/demandes/out", h.jeton("orange", "orange2026")), 1)
	require.Empty(t, h.liste("/api/gateway/v1/demandes/out", h.jeton("yas", "yas2026")))
}

func TestInExclutLesRestitutions(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})
	require.Equal(t, http.StatusCreated, rep.StatusCode)
	id := corps["data"].(map[string]any)["id"].(string)

	_, err := h.db.Pool.Exec(context.Background(),
		`UPDATE demande SET statut_demande = 'TERMINE', date_finalisation = now() WHERE id = $1`, id)
	require.NoError(t, err)

	require.Empty(t, h.liste("/api/gateway/v1/demandes/in", h.jeton("orange", "orange2026")),
		"/in ne porte que sur les portages")
}

func TestMessagesDesListes(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")

	cas := map[string]string{
		"/api/gateway/v1/demandes/mes-demandes":    "Demandes récupérées avec succès",
		"/api/gateway/v1/demandes/a-accepter":      "Demandes à accepter récupérées avec succès",
		"/api/gateway/v1/demandes/a-traiter":       "Demandes à traiter récupérées avec succès",
		"/api/gateway/v1/demandes/a-confirmer":     "Demandes à confirmer récupérées avec succès",
		"/api/gateway/v1/demandes/deja-confirmees": "Demandes déjà confirmées récupérées avec succès",
		"/api/gateway/v1/demandes/in":              "Demandes IN récupérées avec succès",
		"/api/gateway/v1/demandes/out":             "Demandes OUT récupérées avec succès",
	}
	for chemin, message := range cas {
		_, corps := h.appel(http.MethodGet, chemin, jeton, nil)
		require.Equalf(t, message, corps["message"], chemin)
	}
}
