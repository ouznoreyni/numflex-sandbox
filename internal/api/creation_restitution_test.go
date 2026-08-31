package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/seed"
)

func TestRestitutionNominale(t *testing.T) {
	// Tranche 773 : YAS, portée depuis ORANGE il y a 240 jours.
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})

	require.Equal(t, http.StatusCreated, rep.StatusCode, corps)
	data := corps["data"].(map[string]any)
	require.Equal(t, "RESTITUTION", data["typeDemande"])
	require.Equal(t, "ACCEPTATION", data["etapeActuelle"])

	// L'opérateur d'origine récupère le numéro : il est destinataire.
	require.Equal(t, seed.OperateurOrange,
		data["operateurDestinataire"].(map[string]any)["id"])
	require.Equal(t, seed.OperateurYAS,
		data["operateurSource"].(map[string]any)["id"])

	// routageInfo n'existe qu'à partir de la COMPLETION (§7.10).
	require.Nil(t, data["routageInfo"])
}

func TestRestitutionNumeroNonPorte(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "771000001"})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t,
		"RuntimeException: Le numéro n'a pas été porté, pas de restitution/reverse possible",
		corps["detail"])
}

func TestRestitutionTropTotEstUne500EncapsulantUne400(t *testing.T) {
	// ANO-020.
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "774000001"})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Contains(t, corps["detail"], "error.numeroRestitutionTooEarly")
	require.NotContains(t, corps, "code")
}

func TestRestitutionDejaRestituee(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "775000001"})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "RuntimeException: Ce numéro a déjà été restitué", corps["detail"])
}

func TestRestitutionReserveeALOperateurDOrigine(t *testing.T) {
	h := nouveauHarnais(t)
	// EXPRESSO n'est ni détenteur ni opérateur d'origine du 773000001.
	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.jeton("expresso", "expresso2026"), map[string]any{"numero": "773000001"})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestRestitutionSansOTP(t *testing.T) {
	// §7.5 : le corps ne porte que le numéro, aucun OTP n'est exigé.
	h := nouveauHarnais(t)
	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "773000002"})
	require.Equal(t, http.StatusCreated, rep.StatusCode)
}
