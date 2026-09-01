package controller_test

// These 12 test functions are moved, unchanged in assertion, from the
// deleted internal/api/lecture_test.go (Task 12). They still exercise the
// real, live router — routerharness.NewRouterHarness wraps api.NewRouter,
// wired exactly as cmd/server/main.go wires it — so a green run here proves
// a real HTTP request to one of the seven read-only routes goes through the
// new QueryController and not through any leftover handler. creerPortage,
// corpsParticulier, operateurOrange and operateurYAS are the free functions
// and constants creation_particulier_test.go already defines in this same
// package — reused, not copied.

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
)

// avancerA fait progresser une demande jusqu'à l'étape voulue en manipulant
// directement la base — les endpoints de traitement sont testés ailleurs.
func avancerA(h *routerharness.RouterHarness, id, etape string) {
	h.T.Helper()
	_, err := h.DB.Pool.Exec(context.Background(),
		`UPDATE demande SET etape_actuelle = $2, statut_etape_actuel = 'EN_COURS',
		                    date_debut_etape = now(), transition_prevue_a = NULL
		  WHERE id = $1`, id, etape)
	require.NoError(h.T, err)
}

func TestMesDemandesVoitSourceEtDestinataire(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")

	for _, compte := range [][2]string{{"yas", "yas2026"}, {"orange", "orange2026"}} {
		data := h.Liste("/api/gateway/v1/demandes/mes-demandes", h.Jeton(compte[0], compte[1]))
		require.Len(t, data, 1, compte[0])
		require.Equal(t, id, data[0].(map[string]any)["id"])
	}

	// EXPRESSO n'est partie à rien.
	require.Empty(t, h.Liste("/api/gateway/v1/demandes/mes-demandes",
		h.Jeton("expresso", "expresso2026")))
}

func TestMesDemandesNAcceptePasDePagination(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	creerPortage(h, "771000001")

	_, corps := h.Appel(http.MethodGet, "/api/gateway/v1/demandes/mes-demandes",
		h.Jeton("yas", "yas2026"), nil)

	require.NotContains(t, corps, "page")
	require.NotContains(t, corps, "size")
	require.NotContains(t, corps, "totalElements")
}

func TestAAccepterEstReserveeALaSource(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")

	data := h.Liste("/api/gateway/v1/demandes/a-accepter", h.Jeton("orange", "orange2026"))
	require.Len(t, data, 1)
	require.Equal(t, id, data[0].(map[string]any)["id"])

	require.Empty(t, h.Liste("/api/gateway/v1/demandes/a-accepter", h.Jeton("yas", "yas2026")))
}

func TestATraiterSuitLeResponsableDeLEtape(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")

	avancerA(h, id, "DESACTIVATION")
	require.Len(t, h.Liste("/api/gateway/v1/demandes/a-traiter", h.Jeton("orange", "orange2026")), 1)
	require.Empty(t, h.Liste("/api/gateway/v1/demandes/a-traiter", h.Jeton("yas", "yas2026")))

	avancerA(h, id, "ACTIVATION")
	require.Len(t, h.Liste("/api/gateway/v1/demandes/a-traiter", h.Jeton("yas", "yas2026")), 1)
	require.Empty(t, h.Liste("/api/gateway/v1/demandes/a-traiter", h.Jeton("orange", "orange2026")))
}

func TestAConfirmerContientLeTiers(t *testing.T) {
	// D-6, mesuré au SIT : EXPRESSO, ni source ni destinataire, doit confirmer.
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")
	avancerA(h, id, "CONFIRMATION")

	require.Len(t, h.Liste("/api/gateway/v1/demandes/a-confirmer",
		h.Jeton("orange", "orange2026")), 1)
	require.Len(t, h.Liste("/api/gateway/v1/demandes/a-confirmer",
		h.Jeton("expresso", "expresso2026")), 1)

	// Le destinataire est auto-confirmé : la demande ne figure pas dans sa file.
	require.Empty(t, h.Liste("/api/gateway/v1/demandes/a-confirmer",
		h.Jeton("yas", "yas2026")))
}

func TestDetailAConfirmerRefuseAuDestinataire(t *testing.T) {
	// Mesuré : GET /a-confirmer/{id} avec le jeton du destinataire répond 500.
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")
	avancerA(h, id, "CONFIRMATION")

	rep, _ := h.Appel(http.MethodGet, "/api/gateway/v1/demandes/a-confirmer/"+id,
		h.Jeton("yas", "yas2026"), nil)
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)

	rep, _ = h.Appel(http.MethodGet, "/api/gateway/v1/demandes/a-confirmer/"+id,
		h.Jeton("orange", "orange2026"), nil)
	require.Equal(t, http.StatusOK, rep.StatusCode)
}

func TestDetailDemandeInconnue(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	rep, corps := h.Appel(http.MethodGet,
		"/api/gateway/v1/demandes/a-traiter/6a0000000000000000000000",
		h.Jeton("yas", "yas2026"), nil)

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "RuntimeException: Demande introuvable", corps["detail"])
}

func TestInEtOutNeContiennentQueLesPortagesTermines(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")

	require.Empty(t, h.Liste("/api/gateway/v1/demandes/in", h.Jeton("yas", "yas2026")))

	_, err := h.DB.Pool.Exec(context.Background(),
		`UPDATE demande SET statut_demande = 'TERMINE', etape_actuelle = 'COMPLETION',
		                    statut_etape_actuel = 'VALIDE', date_finalisation = now()
		  WHERE id = $1`, id)
	require.NoError(t, err)

	data := h.Liste("/api/gateway/v1/demandes/in", h.Jeton("yas", "yas2026"))
	require.Len(t, data, 1)
	require.Equal(t, "TERMINE", data[0].(map[string]any)["statutDemande"])
	require.NotNil(t, data[0].(map[string]any)["dateFinalisation"])

	require.Len(t, h.Liste("/api/gateway/v1/demandes/out", h.Jeton("orange", "orange2026")), 1)
	require.Empty(t, h.Liste("/api/gateway/v1/demandes/out", h.Jeton("yas", "yas2026")))
}

func TestInExclutLesRestitutions(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.Jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})
	require.Equal(t, http.StatusCreated, rep.StatusCode)
	id := corps["data"].(map[string]any)["id"].(string)

	_, err := h.DB.Pool.Exec(context.Background(),
		`UPDATE demande SET statut_demande = 'TERMINE', date_finalisation = now() WHERE id = $1`, id)
	require.NoError(t, err)

	require.Empty(t, h.Liste("/api/gateway/v1/demandes/in", h.Jeton("orange", "orange2026")),
		"/in ne porte que sur les portages")
}

func TestMessagesDesListes(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	jeton := h.Jeton("yas", "yas2026")

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
		_, corps := h.Appel(http.MethodGet, chemin, jeton, nil)
		require.Equalf(t, message, corps["message"], chemin)
	}
}

// TestListeVideSerialiseEnTableauVideJamaisNull prouve, sur le corps JSON
// brut plutôt que sur la valeur décodée, qu'une file sans résultat rend
// "data":[] et jamais "data":null — le comportement que rendreListe (avant
// cette tâche) et resolveViews/dtoList (depuis) garantissent en initialisant
// leur tranche de sortie non-nil avant la boucle qui peut échouer avant le
// premier append. h.Liste seule ne le distinguerait pas assez explicitement :
// son type-assertion corps["data"].([]any) échoue déjà sur un null JSON
// (décodé en interface{} nil côté Go), donc cette assertion vérifie l'octet
// exact du corps de réponse, pas seulement un échec de cast.
func TestListeVideSerialiseEnTableauVideJamaisNull(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	// EXPRESSO n'est partie à aucune demande : toutes les sept files sont vides.
	jeton := h.Jeton("expresso", "expresso2026")

	for _, chemin := range []string{
		"/api/gateway/v1/demandes/mes-demandes",
		"/api/gateway/v1/demandes/a-accepter",
		"/api/gateway/v1/demandes/a-traiter",
		"/api/gateway/v1/demandes/a-confirmer",
		"/api/gateway/v1/demandes/deja-confirmees",
		"/api/gateway/v1/demandes/in",
		"/api/gateway/v1/demandes/out",
	} {
		rep := h.Brut(http.MethodGet, chemin, jeton, nil)
		require.Equal(t, http.StatusOK, rep.StatusCode, chemin)

		var brut struct {
			Data json.RawMessage `json:"data"`
		}
		require.NoError(t, json.NewDecoder(rep.Body).Decode(&brut), chemin)
		require.JSONEqf(t, "[]", string(brut.Data), "%s : data doit être [] et non null", chemin)
	}
}
