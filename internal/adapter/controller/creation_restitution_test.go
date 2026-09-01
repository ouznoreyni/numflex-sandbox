package controller_test

// These 6 test functions are moved, unchanged in assertion, from the
// deleted internal/api/creation_restitution_test.go (Task 12).

import (
	"net/http"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
	"github.com/stretchr/testify/require"
)

func TestRestitutionNominale(t *testing.T) {
	// Tranche 773 : YAS, portée depuis ORANGE il y a 240 jours.
	h := routerharness.NewRouterHarness(t)
	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.Jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})

	require.Equal(t, http.StatusCreated, rep.StatusCode, corps)
	data := corps["data"].(map[string]any)
	require.Equal(t, "RESTITUTION", data["typeDemande"])
	require.Equal(t, "ACCEPTATION", data["etapeActuelle"])

	// L'opérateur d'origine récupère le numéro : il est destinataire.
	require.Equal(t, operateurOrange,
		data["operateurDestinataire"].(map[string]any)["id"])
	require.Equal(t, operateurYAS,
		data["operateurSource"].(map[string]any)["id"])

	// routageInfo n'existe qu'à partir de la COMPLETION (§7.10).
	require.Nil(t, data["routageInfo"])
}

func TestRestitutionNumeroNonPorte(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.Jeton("orange", "orange2026"), map[string]any{"numero": "771000001"})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t,
		"RuntimeException: Le numéro n'a pas été porté, pas de restitution/reverse possible",
		corps["detail"])
}

func TestRestitutionTropTotEstUne500EncapsulantUne400(t *testing.T) {
	// ANO-020.
	h := routerharness.NewRouterHarness(t)
	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.Jeton("orange", "orange2026"), map[string]any{"numero": "774000001"})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Contains(t, corps["detail"], "error.numeroRestitutionTooEarly")
	require.NotContains(t, corps, "code")
}

func TestRestitutionDejaRestituee(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.Jeton("orange", "orange2026"), map[string]any{"numero": "775000001"})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "RuntimeException: Ce numéro a déjà été restitué", corps["detail"])
}

func TestRestitutionReserveeALOperateurDOrigine(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	// EXPRESSO n'est ni détenteur ni opérateur d'origine du 773000001.
	rep, _ := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.Jeton("expresso", "expresso2026"), map[string]any{"numero": "773000001"})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestRestitutionSansOTP(t *testing.T) {
	// §7.5 : le corps ne porte que le numéro, aucun OTP n'est exigé.
	h := routerharness.NewRouterHarness(t)
	rep, _ := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.Jeton("orange", "orange2026"), map[string]any{"numero": "773000002"})
	require.Equal(t, http.StatusCreated, rep.StatusCode)
}
