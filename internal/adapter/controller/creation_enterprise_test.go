package controller_test

// These 8 test functions are moved, unchanged in assertion, from the
// deleted internal/api/creation_entreprise_test.go (Task 12).

import (
	"context"
	"net/http"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
	"github.com/stretchr/testify/require"
)

func enterpriseBody(porteur string, flotte []string) map[string]any {
	return map[string]any{
		"numeroPorteurFlotte":     porteur,
		"otpCode":                 "123456",
		"operateurSourceId":       operatorOrange,
		"operateurDestinataireId": operatorYAS,
		"typePortabilite":         "POSTPAID",
		"numerosFlotte":           flotte,
		"client": map[string]any{
			"raisonSociale": "Entreprise SARL", "numRC": "123456789",
			"prenom": "Ousmane", "nom": "Diallo", "dateNaissance": "1975-03-20",
			"typePiece": "CNI", "numeroPiece": "1234567890123",
		},
	}
}

func TestFleetNominal(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	token := h.Token("yas", "yas2026")
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000001"})

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/entreprise", token,
		enterpriseBody("771000001", []string{"771000001", "771000002", "771000003"}))

	require.Equal(t, http.StatusCreated, resp.StatusCode, body)
	require.Equal(t, "Demande flotte créée", body["message"])

	data := body["data"].(map[string]any)
	request := data["demande"].(map[string]any)
	require.Equal(t, "ENTREPRISE", request["typeAbonne"])
	require.Equal(t, "ACCEPTATION", request["etapeActuelle"])
	require.Equal(t, float64(3), data["numerosPortesCount"])
	require.Equal(t, float64(0), data["numerosExclusCount"])
}

func TestFleetPartialExclusion(t *testing.T) {
	// BR-006 / invariant 11: the fleet succeeds with fewer numbers than requested.
	h := routerharness.NewRouterHarness(t)
	createPorting(h, "771000009") // this number now has a request in progress

	token := h.Token("yas", "yas2026")
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000001"})

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/entreprise", token,
		enterpriseBody("771000001", []string{"771000001", "771000002", "771000009"}))

	require.Equal(t, http.StatusCreated, resp.StatusCode)
	data := body["data"].(map[string]any)
	require.Equal(t, float64(2), data["numerosPortesCount"])
	require.Equal(t, float64(1), data["numerosExclusCount"])
	require.Equal(t, "1 numéro(s) exclu(s) de la demande.", data["avertissement"])

	excluded := data["numerosExclus"].([]any)
	require.Len(t, excluded, 1)
	first := excluded[0].(map[string]any)
	require.Equal(t, "771000009", first["numero"])
	require.Equal(t, "Demande déjà en cours pour ce numéro", first["raison"])
	require.Equal(t, "DEMANDE_EN_COURS_POUR_NUMERO", first["codeErreur"])
}

func TestFleetEmpty(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	token := h.Token("yas", "yas2026")
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000001"})

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/entreprise", token,
		enterpriseBody("771000001", []string{}))

	// In real fidelity, FLOTTE_VIDE comes out as problem-with-message: it is
	// a business validation, not a bean-validation violation, hence no
	// fieldErrors — and no code field (ANO-001). The message stays readable.
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.NotContains(t, body, "fieldErrors")
	require.NotContains(t, body, "code")
	require.Equal(t, "La liste des numéros de flotte est vide", body["detail"])
}

func TestFleetMixedOperators(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	token := h.Token("yas", "yas2026")
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000001"})

	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/demandes/entreprise", token,
		enterpriseBody("771000001", []string{"771000001", "701000001"}))

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestFleetNoEligibleNumber(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	token := h.Token("yas", "yas2026")
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "779000001"})

	// Slice 772: ported 30 days ago, hence under the 3-month delay.
	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/entreprise", token,
		enterpriseBody("779000001", []string{"779000001", "779000002"}))

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "RuntimeException: Aucun numéro de la flotte n'est éligible au portage",
		body["detail"])

	var n int
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM demande").Scan(&n))
	require.Equal(t, 0, n, "no request should be created")
}

func TestFleetOneOTPCoversTheWholeFleet(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	token := h.Token("yas", "yas2026")
	// OTP sent only for the carrier number.
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000001"})

	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/demandes/entreprise", token,
		enterpriseBody("771000001", []string{"771000001", "771000002", "771000003"}))

	require.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestFleetExclusionFreesTheNumberForANewRequest(t *testing.T) {
	// A number excluded from a fleet must no longer be counted as "in
	// progress" once the request that actually blocked it completes —
	// otherwise the excluded row the fleet itself left behind blocks it
	// indefinitely (Task 13+).
	h := routerharness.NewRouterHarness(t)
	blockingID := createPorting(h, "771000009") // real request, still EN_COURS

	token := h.Token("yas", "yas2026")
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000001"})

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/entreprise", token,
		enterpriseBody("771000001", []string{"771000001", "771000002", "771000009"}))
	require.Equal(t, http.StatusCreated, resp.StatusCode, body)
	data := body["data"].(map[string]any)
	require.Equal(t, float64(1), data["numerosExclusCount"])

	// The request that actually blocked 771000009 completes: nothing about
	// it applies any more, including the excluded row from the fleet above.
	_, err := h.DB.Pool.Exec(context.Background(),
		"UPDATE demande SET statut_demande = 'TERMINE' WHERE id = $1", blockingID)
	require.NoError(t, err)

	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000009"})
	rep2, corps2 := h.Call(http.MethodPost, "/api/gateway/v1/demandes/particulier", token,
		individualBody("771000009"))

	require.Equal(t, http.StatusCreated, rep2.StatusCode, corps2)
}

// TestEmptyFleetReturnsFlotteVideCode — guide §9: the FLOTTE_VIDE code
// exists in the catalog for this exact case ("La liste des numéros de
// flotte est vide"). Treating it as a bean-validation violation would make
// it unreachable.
func TestEmptyFleetReturnsFlotteVideCode(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.ContractFidelity)
	token := h.Token("yas", "yas2026")
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000001"})

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/entreprise", token,
		enterpriseBody("771000001", []string{}))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, "FLOTTE_VIDE", body["code"])
	require.Equal(t, "La liste des numéros de flotte est vide", body["message"])
}
