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

// Reason ids come from internal/framework/seed as literals, not an import
// (a controller test cannot import internal/framework — the dependency
// rule applies to _test.go files too), the same precedent
// creation_particulier_test.go already sets for operateurOrange/operateurYAS.
const (
	reasonIdentityNotProven = "6a2175f3e6c37b5b5b487edf"
	reasonNumberInactive    = "6a2175e7e6c37b5b5b487ede"
	reasonMissingData       = "6a2175d9e6c37b5b5b487edd"
)

func TestAcceptNominal(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.Token("orange", "orange2026"),
		map[string]any{"idDemande": id, "accepte": true, "commentaire": "Demande conforme"})

	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	require.Equal(t, "Décision d'acceptation enregistrée", body["message"])

	// The response carries the NEXT step: capture « 1. orange_2_ACCEPTATION
	// Accepter ou rejeter une demande_next_DESACTIVATION ».
	data := body["data"].(map[string]any)
	require.Equal(t, "DESACTIVATION", data["etapeActuelle"])

	// The transition was applied within the request, nothing stays scheduled.
	var prevue *string
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		"SELECT transition_prevue_a::text FROM demande WHERE id = $1", id).Scan(&prevue))
	require.Nil(t, prevue)
}

func TestAcceptByRecipientRefused(t *testing.T) {
	// TC-034: refused — but as HTTP 500.
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.Token("yas", "yas2026"), map[string]any{"idDemande": id, "accepte": true})

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.NotContains(t, body, "code")
}

func TestRejectWithoutReasonRefused(t *testing.T) {
	// TC-044: refused — as HTTP 500.
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.Token("orange", "orange2026"), map[string]any{"idDemande": id, "accepte": false})

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t,
		"RuntimeException: Un motif de rejet est obligatoire pour rejeter une demande",
		body["detail"])
}

func TestRejectWithReasonCompletesTheRequest(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")

	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.Token("orange", "orange2026"), map[string]any{
			"idDemande": id, "accepte": false,
			"motifRejetId": reasonIdentityNotProven,
			"commentaire":  "Contrat non résilié",
		})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var status, reason string
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		"SELECT statut_demande, motif_rejet_id FROM demande WHERE id = $1", id).
		Scan(&status, &reason))
	require.Equal(t, "REJETE", status)
	require.Equal(t, reasonIdentityNotProven, reason)
}

func TestAcceptUnknownRequestID(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.Token("orange", "orange2026"),
		map[string]any{"idDemande": "6a0000000000000000000000", "accepte": true})

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "RuntimeException: Demande introuvable", body["detail"])
}

func TestAcceptByMSISDNRefusedBecauseV2IdentifiesByRequestID(t *testing.T) {
	// v1 → v2 break: the numero field is no longer recognised.
	h := routerharness.NewRouterHarness(t)
	createPorting(h, "771000001")

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.Token("orange", "orange2026"),
		map[string]any{"numero": "771000001", "accepte": true})

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	champs := body["fieldErrors"].([]any)
	require.Equal(t, "idDemande", champs[0].(map[string]any)["field"])
}

func TestAcceptFleetWithPartialRejection(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	token := h.Token("yas", "yas2026")
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000001"})
	_, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/entreprise", token,
		enterpriseBody("771000001", []string{"771000001", "771000002", "771000003"}))
	id := body["data"].(map[string]any)["demande"].(map[string]any)["id"].(string)

	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/acceptation",
		h.Token("orange", "orange2026"), map[string]any{
			"accepte": true,
			"numerosRejetes": []map[string]any{
				{"numero": "771000002", "motifRejetId": reasonNumberInactive},
			},
			"commentaire": "Numéro 771000002 non conforme",
		})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var status string
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		"SELECT statut FROM demande_numero WHERE demande_id = $1 AND numero = '771000002'", id).
		Scan(&status))
	require.Equal(t, "REJETE", status)

	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		"SELECT statut FROM demande_numero WHERE demande_id = $1 AND numero = '771000001'", id).
		Scan(&status))
	require.Equal(t, "EN_COURS", status)
}

func TestAcceptFleetTotalRejection(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	token := h.Token("yas", "yas2026")
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000001"})
	_, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/entreprise", token,
		enterpriseBody("771000001", []string{"771000001", "771000002"}))
	id := body["data"].(map[string]any)["demande"].(map[string]any)["id"].(string)

	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/acceptation",
		h.Token("orange", "orange2026"), map[string]any{
			"accepte": false, "motifRejetId": reasonMissingData,
			"commentaire": "Dossier incomplet",
		})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var status string
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		"SELECT statut_demande FROM demande WHERE id = $1", id).Scan(&status))
	require.Equal(t, "REJETE", status)
}

func TestAcceptWithUnknownRejectionReasonRefused(t *testing.T) {
	// The motifRejetId existence check does not depend on accepte: an
	// unknown id is refused even on an acceptance.
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.Token("orange", "orange2026"),
		map[string]any{"idDemande": id, "accepte": true, "motifRejetId": "inconnu-000"})

	require.Equal(t, http.StatusBadRequest, resp.StatusCode, body)
	require.Equal(t, "Motif de rejet inconnu", body["detail"])
}

// TestAcceptationPlaceGeleePrimeSurCorpsInvalide pins fix round 1's
// correction: the frozen-market gate must run BEFORE the request body is
// even decoded, so a request that carries both a frozen market and a
// malformed body gets the frozen-market response, not a JSON-format error.
// A caller sends "corps-invalide" — a bare JSON string, which fails to bind
// into acceptRequestDTO exactly as a syntactically broken body would —
// and the response must still be the frozen-market one: proof the check
// really does run first, not merely that it runs at all.
func TestAcceptFrozenMarketBeatsInvalidBody(t *testing.T) {
	h := routerharness.NewRouterHarness(t)

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/incidents/interne",
		h.Token("expresso", "expresso2026"),
		map[string]any{"commentaire": "Panne du système de routage interne, portages bloqués"})
	require.Equal(t, http.StatusCreated, resp.StatusCode, body)

	resp, body = h.Call(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.Token("orange", "orange2026"), "corps-invalide")

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode, body)
	require.Equal(t,
		"RuntimeException: Le traitement des demandes est gelé par un incident interne en cours.",
		body["detail"])
}
