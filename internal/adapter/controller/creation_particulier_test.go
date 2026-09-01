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

// Identifiants d'opérateurs du seed de recette (internal/framework/seed) —
// recopiés en littéral plutôt qu'importés : un test de contrôleur ne peut
// pas importer internal/framework (règle de dépendance), et
// reference_controller_test.go établit déjà ce précédent.
const (
	operateurOrange = "6a21745ce6c37b5b5b487ec1"
	operateurYAS    = "6a2174c3e6c37b5b5b487ec4"
)

func corpsParticulier(numero string) map[string]any {
	return map[string]any{
		"numero":                  numero,
		"otpCode":                 "123456",
		"operateurSourceId":       operateurOrange,
		"operateurDestinataireId": operateurYAS,
		"typePortabilite":         "PREPAID",
		"client": map[string]any{
			"nom": "Diallo", "prenom": "Mamadou",
			"dateNaissance": "1975-03-20", "lieuNaissance": "Dakar",
			"typePiece": "CNI", "numeroPiece": "1234567890123",
		},
	}
}

// creerPortage envoie l'OTP puis crée une demande particulier ORANGE → YAS.
func creerPortage(h *routerharness.RouterHarness, numero string) string {
	h.T.Helper()
	jeton := h.Jeton("yas", "yas2026")
	h.Appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton, map[string]any{"numero": numero})

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		jeton, corpsParticulier(numero))
	require.Equal(h.T, http.StatusCreated, rep.StatusCode, corps)

	data := corps["data"].(map[string]any)
	return data["id"].(string)
}

func TestCreationParticulierNominale(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	jeton := h.Jeton("yas", "yas2026")
	h.Appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		jeton, corpsParticulier("771000001"))

	require.Equal(t, http.StatusCreated, rep.StatusCode)
	require.Equal(t, "Demande particulier créée avec succès", corps["message"])

	data := corps["data"].(map[string]any)
	require.Regexp(t, `^[0-9a-f]{24}$`, data["id"])
	require.Equal(t, "771000001", data["numero"])
	require.Equal(t, "PARTICULIER", data["typeAbonne"])
	require.Equal(t, "PORTAGE", data["typeDemande"])
	require.Equal(t, "EN_COURS", data["statutDemande"])
	require.Equal(t, "ACCEPTATION", data["etapeActuelle"])
	require.Equal(t, "PREPAID", data["processus"])
	require.Equal(t, "191", data["routageInfo"], "routage initial de la source")

	src := data["operateurSource"].(map[string]any)
	require.Equal(t, operateurOrange, src["id"])
	require.Equal(t, "ORANGE", src["nom"])
	dst := data["operateurDestinataire"].(map[string]any)
	require.Equal(t, "YAS", dst["nom"])
}

func TestCreationParticulierLieuNaissanceObligatoire(t *testing.T) {
	// TC-050 / ANO-010 : le guide le documente facultatif, la plateforme le rejette.
	h := routerharness.NewRouterHarness(t)
	jeton := h.Jeton("yas", "yas2026")
	h.Appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000002"})

	c := corpsParticulier("771000002")
	delete(c["client"].(map[string]any), "lieuNaissance")

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/particulier", jeton, c)

	require.Equal(t, http.StatusBadRequest, rep.StatusCode)
	champs := corps["fieldErrors"].([]any)
	require.Len(t, champs, 1)
	require.Equal(t, "client.lieuNaissance", champs[0].(map[string]any)["field"])
}

func TestCreationParticulierDoitEtreLeDestinataire(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	jetonOrange := h.Jeton("orange", "orange2026")
	h.Appel(http.MethodPost, "/api/gateway/v1/otp/send", jetonOrange,
		map[string]any{"numero": "771000003"})

	rep, _ := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		jetonOrange, corpsParticulier("771000003"))

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestCreationParticulierDelaiPortageSePresenteCommeUnePanne(t *testing.T) {
	// ANO-002.
	h := routerharness.NewRouterHarness(t)
	jeton := h.Jeton("yas", "yas2026")
	h.Appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "772000001"})

	c := corpsParticulier("772000001")
	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/particulier", jeton, c)

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "Unexpected runtime exception", corps["detail"])
	require.NotContains(t, corps, "code")
}

func TestCreationParticulierSansOTP(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		h.Jeton("yas", "yas2026"), corpsParticulier("771000004"))

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "Aucun OTP actif pour ce numéro", corps["detail"])
}

func TestCreationParticulierConsommeLOTP(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	creerPortage(h, "771000005")

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/otp/verify",
		h.Jeton("yas", "yas2026"),
		map[string]any{"numero": "771000005", "otpCode": "123456"})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "RuntimeException: Code déjà utilisé", corps["detail"])
}

func TestCreationParticulierDemandeDejaEnCours(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	creerPortage(h, "771000006")

	jeton := h.Jeton("yas", "yas2026")
	h.Appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000006"})
	rep, _ := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		jeton, corpsParticulier("771000006"))

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestCreationParticulierEnModeContratRendUnCodeMetier(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.FiabiliteContrat)
	jeton := h.Jeton("yas", "yas2026")
	h.Appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "772000002"})

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		jeton, corpsParticulier("772000002"))

	require.Equal(t, http.StatusConflict, rep.StatusCode)
	require.Equal(t, "DELAI_PORTAGE_NON_RESPECTE", corps["code"])
	require.Equal(t, false, corps["success"])
}
