package controller_test

// These 8 test functions are moved, unchanged in assertion, from the
// deleted internal/api/creation_particulier_test.go (Task 12). They still
// exercise the real, live router — routerharness.NewRouterHarness wraps
// api.NewRouter, wired exactly as cmd/server/main.go wires it — so a green
// run here proves a real HTTP request to /demandes/particulier goes through
// the new CreationController, port.UnitOfWork and the creation interactors,
// not through any leftover handler.

import (
	"net/http"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
	"github.com/stretchr/testify/require"
)

// Operator ids from the acceptance seed (internal/framework/seed) — copied
// as literals rather than imported: a controller test cannot import
// internal/framework (dependency rule), and reference_controller_test.go
// already sets this precedent.
const (
	operatorOrange = "6a21745ce6c37b5b5b487ec1"
	operatorYAS    = "6a2174c3e6c37b5b5b487ec4"
)

func individualBody(msisdn string) map[string]any {
	return map[string]any{
		"numero":                  msisdn,
		"otpCode":                 "123456",
		"operateurSourceId":       operatorOrange,
		"operateurDestinataireId": operatorYAS,
		"typePortabilite":         "PREPAID",
		"client": map[string]any{
			"nom": "Diallo", "prenom": "Mamadou",
			"dateNaissance": "1975-03-20", "lieuNaissance": "Dakar",
			"typePiece": "CNI", "numeroPiece": "1234567890123",
		},
	}
}

// createPorting sends the OTP then creates an individual request ORANGE → YAS.
func createPorting(h *routerharness.RouterHarness, msisdn string) string {
	h.T.Helper()
	token := h.Token("yas", "yas2026")
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token, map[string]any{"numero": msisdn})

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		token, individualBody(msisdn))
	require.Equal(h.T, http.StatusCreated, resp.StatusCode, body)

	data := body["data"].(map[string]any)
	return data["id"].(string)
}

func TestCreateIndividualNominal(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	token := h.Token("yas", "yas2026")
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000001"})

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		token, individualBody("771000001"))

	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Equal(t, "Demande particulier créée avec succès", body["message"])

	data := body["data"].(map[string]any)
	require.Regexp(t, `^[0-9a-f]{24}$`, data["id"])
	require.Equal(t, "771000001", data["numero"])
	require.Equal(t, "PARTICULIER", data["typeAbonne"])
	require.Equal(t, "PORTAGE", data["typeDemande"])
	require.Equal(t, "EN_COURS", data["statutDemande"])
	require.Equal(t, "ACCEPTATION", data["etapeActuelle"])
	require.Equal(t, "PREPAID", data["processus"])
	require.Equal(t, "191", data["routageInfo"], "source's initial routing")

	src := data["operateurSource"].(map[string]any)
	require.Equal(t, operatorOrange, src["id"])
	require.Equal(t, "ORANGE", src["nom"])
	dst := data["operateurDestinataire"].(map[string]any)
	require.Equal(t, "YAS", dst["nom"])
}

func TestCreateIndividualBirthPlaceRequired(t *testing.T) {
	// TC-050 / ANO-010: the guide documents it as optional, the platform rejects it.
	h := routerharness.NewRouterHarness(t)
	token := h.Token("yas", "yas2026")
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000002"})

	c := individualBody("771000002")
	delete(c["client"].(map[string]any), "lieuNaissance")

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/particulier", token, c)

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	fields := body["fieldErrors"].([]any)
	require.Len(t, fields, 1)
	require.Equal(t, "client.lieuNaissance", fields[0].(map[string]any)["field"])
}

func TestCreateIndividualMustBeTheRecipient(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	tokenOrange := h.Token("orange", "orange2026")
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", tokenOrange,
		map[string]any{"numero": "771000003"})

	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		tokenOrange, individualBody("771000003"))

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestCreateIndividualPortingDelayPresentsAsAFailure(t *testing.T) {
	// ANO-002.
	h := routerharness.NewRouterHarness(t)
	token := h.Token("yas", "yas2026")
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "779000001"})

	c := individualBody("779000001")
	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/particulier", token, c)

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "Unexpected runtime exception", body["detail"])
	require.NotContains(t, body, "code")
}

func TestCreateIndividualWithoutOTP(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		h.Token("yas", "yas2026"), individualBody("771000004"))

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "Aucun OTP actif pour ce numéro", body["detail"])
}

func TestCreateIndividualConsumesOTP(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	createPorting(h, "771000005")

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/otp/verify",
		h.Token("yas", "yas2026"),
		map[string]any{"numero": "771000005", "otpCode": "123456"})

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "RuntimeException: Code déjà utilisé", body["detail"])
}

func TestCreateIndividualRequestAlreadyInProgress(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	createPorting(h, "771000006")

	token := h.Token("yas", "yas2026")
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000006"})
	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		token, individualBody("771000006"))

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestCreateIndividualInContractModeReturnsABusinessCode(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.ContractFidelity)
	token := h.Token("yas", "yas2026")
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "779000002"})

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		token, individualBody("779000002"))

	require.Equal(t, http.StatusConflict, resp.StatusCode)
	require.Equal(t, "DELAI_PORTAGE_NON_RESPECTE", body["code"])
	require.Equal(t, false, body["success"])
}
