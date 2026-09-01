package controller_test

// These 8 test functions are moved, unchanged in assertion, from the
// deleted internal/api/sandbox_test.go (Task 16). They still exercise the
// real, live router — routerharness.NewRouterHarness wraps api.NewRouter,
// wired exactly as cmd/server/main.go wires it — so a green run here proves
// a real HTTP request to DELETE /api/sandbox/v1/demandes goes through the
// new SandboxController, PurgeTestDataInteractor and port.UnitOfWork, not
// through any leftover handler.

import (
	"context"
	"net/http"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
	"github.com/stretchr/testify/require"
)

const cheminPurge = "/api/sandbox/v1/demandes"

// creerPortageVers crée une demande particulier dont le destinataire — donc le
// créateur — est le compte donné. creerPortage ne sait faire que ORANGE → YAS.
func creerPortageVers(h *routerharness.RouterHarness, numero, username, motDePasse, source, destinataire string) string {
	h.T.Helper()
	jeton := h.Jeton(username, motDePasse)
	h.Appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton, map[string]any{"numero": numero})

	corpsDem := corpsParticulier(numero)
	corpsDem["operateurSourceId"] = source
	corpsDem["operateurDestinataireId"] = destinataire

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/particulier", jeton, corpsDem)
	require.Equal(h.T, http.StatusCreated, rep.StatusCode, corps)
	return corps["data"].(map[string]any)["id"].(string)
}

func numeroRegistre(h *routerharness.RouterHarness, msisdn string) (actuel string, portage *string) {
	h.T.Helper()
	require.NoError(h.T, h.DB.Pool.QueryRow(context.Background(),
		`SELECT operateur_actuel_id, date_dernier_portage::text FROM numero WHERE msisdn = $1`,
		msisdn).Scan(&actuel, &portage))
	return actuel, portage
}

func compteDemandes(h *routerharness.RouterHarness) int {
	h.T.Helper()
	var n int
	require.NoError(h.T, h.DB.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM demande`).Scan(&n))
	return n
}

// La surface du sandbox doit rester celle de l'ARTP tant que la purge n'est pas
// demandée : route non enregistrée, donc 404 — indistinguable d'un chemin qui
// n'existe pas, y compris avec un jeton valide.
func TestPurgeAbsenteParDefaut(t *testing.T) {
	h := routerharness.NewRouterHarness(t)

	rep := h.Brut(http.MethodDelete, cheminPurge, h.Jeton("yas", "yas2026"), nil)
	require.Equal(t, http.StatusNotFound, rep.StatusCode)

	rep = h.Brut(http.MethodDelete, cheminPurge, "", nil)
	require.Equal(t, http.StatusNotFound, rep.StatusCode)
}

func TestPurgeExigeUnJeton(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.SandboxAdmin)

	rep := h.Brut(http.MethodDelete, cheminPurge, "", nil)
	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
}

// Le périmètre est createur_operateur_id : ce qu'un opérateur a fabriqué, et
// rien d'autre. Une demande créée par un partenaire survit, même si l'appelant
// y est partie.
func TestPurgeNeToucheQueMesCreations(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.SandboxAdmin)

	idYAS := creerPortage(h, "771000001") // créée par YAS, ORANGE → YAS
	idOrange := creerPortageVers(h, "761000001", "orange", "orange2026",
		operateurYAS, operateurOrange)

	rep, corps := h.Appel(http.MethodDelete, cheminPurge, h.Jeton("yas", "yas2026"), nil)

	require.Equal(t, http.StatusOK, rep.StatusCode, corps)
	require.Equal(t, true, corps["success"])
	require.Equal(t, "Demandes purgées avec succès", corps["message"])
	data := corps["data"].(map[string]any)
	require.EqualValues(t, 1, data["demandesSupprimees"])

	require.Equal(t, 1, compteDemandes(h), "la demande d'ORANGE doit survivre")

	// YAS est destinataire de idYAS et source de idOrange : après purge il ne
	// lui reste que celle du partenaire.
	restantes := h.Liste("/api/gateway/v1/demandes/mes-demandes", h.Jeton("yas", "yas2026"))
	require.Len(t, restantes, 1)
	require.Equal(t, idOrange, restantes[0].(map[string]any)["id"])

	rep, _ = h.Appel(http.MethodGet, "/api/gateway/v1/demandes/a-accepter/"+idYAS,
		h.Jeton("orange", "orange2026"), nil)
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode,
		"la demande purgée est introuvable")
}

// Sans restauration du registre, DELAI_PORTAGE_NON_RESPECTE bloquerait le
// numéro trois mois et la purge ne servirait à rien pour rejouer un scénario.
func TestPurgeRestaureLeRegistreEtRendLeNumeroRejouable(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.SandboxAdmin)
	jetonYAS := h.Jeton("yas", "yas2026")

	id := creerPortage(h, "771000001")
	avancerA(h, id, "ACTIVATION")
	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/traitement", jetonYAS,
		map[string]any{"idDemande": id})
	require.Equal(t, http.StatusOK, rep.StatusCode, corps)

	actuel, portage := numeroRegistre(h, "771000001")
	require.Equal(t, operateurYAS, actuel, "le portage a bien transféré le numéro")
	require.NotNil(t, portage)

	_, corps = h.Appel(http.MethodDelete, cheminPurge, jetonYAS, nil)
	data := corps["data"].(map[string]any)
	require.EqualValues(t, 1, data["demandesSupprimees"])
	require.EqualValues(t, 1, data["numerosRestaures"])

	actuel, portage = numeroRegistre(h, "771000001")
	require.Equal(t, operateurOrange, actuel, "le numéro rejoint son opérateur d'origine")
	require.Nil(t, portage, "date_dernier_portage effacée")

	// Le scénario se rejoue immédiatement : c'est la raison d'être de la purge.
	creerPortage(h, "771000001")
}

// L'OTP est consommé à la création ; sans purge le même numéro ne pourrait pas
// être redemandé sans repasser par otp/send — et la ligne resterait orpheline.
func TestPurgeEffaceLesOTPDesNumerosPurges(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.SandboxAdmin)
	creerPortage(h, "771000001")

	_, corps := h.Appel(http.MethodDelete, cheminPurge, h.Jeton("yas", "yas2026"), nil)
	require.EqualValues(t, 1, corps["data"].(map[string]any)["otpSupprimes"])

	var n int
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM otp WHERE numero = '771000001'`).Scan(&n))
	require.Zero(t, n)
}

func TestPurgeSansRienAPurgerReussit(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.SandboxAdmin)

	rep, corps := h.Appel(http.MethodDelete, cheminPurge, h.Jeton("expresso", "expresso2026"), nil)

	require.Equal(t, http.StatusOK, rep.StatusCode)
	data := corps["data"].(map[string]any)
	require.EqualValues(t, 0, data["demandesSupprimees"])
	require.EqualValues(t, 0, data["numerosRestaures"])
	require.EqualValues(t, 0, data["otpSupprimes"])
	require.EqualValues(t, 0, data["reverseSupprimees"])
}

// La gateway ne gagne aucune route : l'invariant des 33 routes tient même
// purge activée.
func TestPurgeNAjouteRienALaGateway(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.SandboxAdmin)

	rep := h.Brut(http.MethodDelete, "/api/gateway/v1/demandes",
		h.Jeton("yas", "yas2026"), nil)
	require.Equal(t, http.StatusNotFound, rep.StatusCode)
}

// reverse_request référence demande sans ON DELETE CASCADE : sans traitement
// explicite, sa clé étrangère bloquerait la suppression. Les demandes de
// reverse de l'appelant partent donc avec le reste de ses données de test.
func TestPurgeEmporteLesDemandesDeReverse(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.SandboxAdmin)
	jetonYAS := h.Jeton("yas", "yas2026")

	// 77200xxxx : détenu par ORANGE, d'origine YAS — YAS peut en demander le reverse.
	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/reverse-requests", jetonYAS,
		map[string]any{"numero": "772000001"})
	require.Equal(t, http.StatusCreated, rep.StatusCode, corps)

	_, corps = h.Appel(http.MethodDelete, cheminPurge, jetonYAS, nil)
	require.EqualValues(t, 1, corps["data"].(map[string]any)["reverseSupprimees"])

	require.Empty(t, h.Liste("/api/gateway/v1/reverse-requests/mes-demandes", jetonYAS))
}
