package controller_test

// The first 9 test functions below are moved, unchanged in assertion, from
// the deleted internal/api/acceptation_test.go (Task 14). They still
// exercise the real, live router — routerharness.NewRouterHarness wraps
// api.NewRouter, wired exactly as cmd/server/main.go wires it — so a green
// run here proves a real HTTP request to /demandes/acceptation or
// /demandes/:id/acceptation goes through the new AcceptanceController, the
// two acceptance interactors and port.UnitOfWork, not through any leftover
// handler.
//
// TestAcceptationPlaceGeleePrimeSurCorpsInvalide, at the end of this file,
// is new — fix round 1 on this task: it pins the frozen-market gate's
// position relative to JSON binding, which no test caught the first time
// around.
//
import (
	"context"
	"net/http"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
	"github.com/stretchr/testify/require"
)

// Motif ids come from internal/framework/seed as literals, not an import
// (a controller test cannot import internal/framework — the dependency
// rule applies to _test.go files too), the same precedent
// creation_particulier_test.go already sets for operateurOrange/operateurYAS.
const (
	motifIdentiteNonProuvee = "6a2175f3e6c37b5b5b487edf"
	motifNumeroInactif      = "6a2175e7e6c37b5b5b487ede"
	motifDonneesManquantes  = "6a2175d9e6c37b5b5b487edd"
)

func TestAcceptationNominale(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.Jeton("orange", "orange2026"),
		map[string]any{"idDemande": id, "accepte": true, "commentaire": "Demande conforme"})

	require.Equal(t, http.StatusOK, rep.StatusCode, corps)
	require.Equal(t, "Décision d'acceptation enregistrée", corps["message"])

	// La réponse porte l'étape SUIVANTE : capture « 1. orange_2_ACCEPTATION
	// Accepter ou rejeter une demande_next_DESACTIVATION ».
	data := corps["data"].(map[string]any)
	require.Equal(t, "DESACTIVATION", data["etapeActuelle"])

	// La transition a été appliquée dans la requête, rien ne reste planifié.
	var prevue *string
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		"SELECT transition_prevue_a::text FROM demande WHERE id = $1", id).Scan(&prevue))
	require.Nil(t, prevue)
}

func TestAcceptationParLeDestinataireRefusee(t *testing.T) {
	// TC-034 : refusé — mais en HTTP 500.
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.Jeton("yas", "yas2026"), map[string]any{"idDemande": id, "accepte": true})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.NotContains(t, corps, "code")
}

func TestRejetSansMotifRefuse(t *testing.T) {
	// TC-044 : refusé — en HTTP 500.
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.Jeton("orange", "orange2026"), map[string]any{"idDemande": id, "accepte": false})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t,
		"RuntimeException: Un motif de rejet est obligatoire pour rejeter une demande",
		corps["detail"])
}

func TestRejetAvecMotifTermineLaDemande(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")

	rep, _ := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.Jeton("orange", "orange2026"), map[string]any{
			"idDemande": id, "accepte": false,
			"motifRejetId": motifIdentiteNonProuvee,
			"commentaire":  "Contrat non résilié",
		})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	var statut, motif string
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		"SELECT statut_demande, motif_rejet_id FROM demande WHERE id = $1", id).
		Scan(&statut, &motif))
	require.Equal(t, "REJETE", statut)
	require.Equal(t, motifIdentiteNonProuvee, motif)
}

func TestAcceptationIdDemandeInconnu(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.Jeton("orange", "orange2026"),
		map[string]any{"idDemande": "6a0000000000000000000000", "accepte": true})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "RuntimeException: Demande introuvable", corps["detail"])
}

func TestAcceptationNumeroRefuseCarV2IdentifieParIdDemande(t *testing.T) {
	// Rupture v1 → v2 : le champ numero n'est plus reconnu.
	h := routerharness.NewRouterHarness(t)
	creerPortage(h, "771000001")

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.Jeton("orange", "orange2026"),
		map[string]any{"numero": "771000001", "accepte": true})

	require.Equal(t, http.StatusBadRequest, rep.StatusCode)
	champs := corps["fieldErrors"].([]any)
	require.Equal(t, "idDemande", champs[0].(map[string]any)["field"])
}

func TestAcceptationFlotteAvecRejetPartiel(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	jeton := h.Jeton("yas", "yas2026")
	h.Appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})
	_, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("771000001", []string{"771000001", "771000002", "771000003"}))
	id := corps["data"].(map[string]any)["demande"].(map[string]any)["id"].(string)

	rep, _ := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/acceptation",
		h.Jeton("orange", "orange2026"), map[string]any{
			"accepte": true,
			"numerosRejetes": []map[string]any{
				{"numero": "771000002", "motifRejetId": motifNumeroInactif},
			},
			"commentaire": "Numéro 771000002 non conforme",
		})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	var statut string
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		"SELECT statut FROM demande_numero WHERE demande_id = $1 AND numero = '771000002'", id).
		Scan(&statut))
	require.Equal(t, "REJETE", statut)

	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		"SELECT statut FROM demande_numero WHERE demande_id = $1 AND numero = '771000001'", id).
		Scan(&statut))
	require.Equal(t, "EN_COURS", statut)
}

func TestAcceptationFlotteRejetTotal(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	jeton := h.Jeton("yas", "yas2026")
	h.Appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})
	_, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("771000001", []string{"771000001", "771000002"}))
	id := corps["data"].(map[string]any)["demande"].(map[string]any)["id"].(string)

	rep, _ := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/acceptation",
		h.Jeton("orange", "orange2026"), map[string]any{
			"accepte": false, "motifRejetId": motifDonneesManquantes,
			"commentaire": "Dossier incomplet",
		})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	var statut string
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		"SELECT statut_demande FROM demande WHERE id = $1", id).Scan(&statut))
	require.Equal(t, "REJETE", statut)
}

func TestAcceptationAccepteAvecMotifRejetInconnuRefuse(t *testing.T) {
	// Le contrôle d'existence du motifRejetId ne dépend pas de accepte : un
	// identifiant inconnu est refusé même sur une acceptation.
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.Jeton("orange", "orange2026"),
		map[string]any{"idDemande": id, "accepte": true, "motifRejetId": "inconnu-000"})

	require.Equal(t, http.StatusBadRequest, rep.StatusCode, corps)
	require.Equal(t, "Motif de rejet inconnu", corps["detail"])
}

// TestAcceptationPlaceGeleePrimeSurCorpsInvalide pins fix round 1's
// correction: the frozen-market gate must run BEFORE the request body is
// even decoded, so a request that carries both a frozen market and a
// malformed body gets the frozen-market response, not a JSON-format error.
// A caller sends "corps-invalide" — a bare JSON string, which fails to bind
// into acceptationRequest exactly as a syntactically broken body would —
// and the response must still be the frozen-market one: proof the check
// really does run first, not merely that it runs at all.
func TestAcceptationPlaceGeleePrimeSurCorpsInvalide(t *testing.T) {
	h := routerharness.NewRouterHarness(t)

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/incidents/interne",
		h.Jeton("expresso", "expresso2026"),
		map[string]any{"commentaire": "Panne du système de routage interne, portages bloqués"})
	require.Equal(t, http.StatusCreated, rep.StatusCode, corps)

	rep, corps = h.Appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.Jeton("orange", "orange2026"), "corps-invalide")

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode, corps)
	require.Equal(t,
		"RuntimeException: Le traitement des demandes est gelé par un incident interne en cours.",
		corps["detail"])
}
