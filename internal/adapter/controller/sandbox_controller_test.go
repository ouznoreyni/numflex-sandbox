package controller_test

// These 8 test functions come from the deleted internal/api/sandbox_test.go
// (Task 16), unchanged in assertion but for the first, which followed
// SANDBOX_ADMIN out of the configuration. They still exercise the
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

const purgePath = "/api/sandbox/v1/demandes"

// createPortingTo creates an individual request whose recipient — hence its
// creator — is the given account. createPorting only ever does ORANGE → YAS.
func createPortingTo(h *routerharness.RouterHarness, msisdn, username, password, source, recipient string) string {
	h.T.Helper()
	token := h.Token(username, password)
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token, map[string]any{"numero": msisdn})

	body := individualBody(msisdn)
	body["operateurSourceId"] = source
	body["operateurDestinataireId"] = recipient

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/particulier", token, body)
	require.Equal(h.T, http.StatusCreated, resp.StatusCode, body)
	return body["data"].(map[string]any)["id"].(string)
}

func registryNumber(h *routerharness.RouterHarness, msisdn string) (current string, portingDate *string) {
	h.T.Helper()
	require.NoError(h.T, h.DB.Pool.QueryRow(context.Background(),
		`SELECT operateur_actuel_id, date_dernier_portage::text FROM numero WHERE msisdn = $1`,
		msisdn).Scan(&current, &portingDate))
	return current, portingDate
}

func requestCount(h *routerharness.RouterHarness) int {
	h.T.Helper()
	var n int
	require.NoError(h.T, h.DB.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM demande`).Scan(&n))
	return n
}

// The purge is mounted unconditionally — no flag to find, no 404 to
// explain: a sandbox whose reset button waits for an environment variable
// is a sandbox nobody resets. It still answers only to a token.
func TestPurgeIsAlwaysMounted(t *testing.T) {
	h := routerharness.NewRouterHarness(t)

	resp := h.Raw(http.MethodDelete, purgePath, h.Token("yas", "yas2026"), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp = h.Raw(http.MethodDelete, purgePath, "", nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestPurgeRequiresAToken(t *testing.T) {
	h := routerharness.NewRouterHarness(t)

	resp := h.Raw(http.MethodDelete, purgePath, "", nil)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// The scope is createur_operateur_id: what an operator created, and nothing
// else. A request created by a partner survives, even if the caller is a
// party to it.
func TestPurgeOnlyTouchesMyOwnCreations(t *testing.T) {
	h := routerharness.NewRouterHarness(t)

	idYAS := createPorting(h, "771000001") // created by YAS, ORANGE → YAS
	idOrange := createPortingTo(h, "761000001", "orange", "orange2026",
		operatorYAS, operatorOrange)

	resp, body := h.Call(http.MethodDelete, purgePath, h.Token("yas", "yas2026"), nil)

	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	require.Equal(t, true, body["success"])
	require.Equal(t, "Demandes purgées avec succès", body["message"])
	data := body["data"].(map[string]any)
	require.EqualValues(t, 1, data["demandesSupprimees"])

	require.Equal(t, 1, requestCount(h), "ORANGE's request must survive")

	// YAS is recipient of idYAS and source of idOrange: after the purge,
	// only its partner's request remains.
	remaining := h.List("/api/gateway/v1/demandes/mes-demandes", h.Token("yas", "yas2026"))
	require.Len(t, remaining, 1)
	require.Equal(t, idOrange, remaining[0].(map[string]any)["id"])

	resp, _ = h.Call(http.MethodGet, "/api/gateway/v1/demandes/a-accepter/"+idYAS,
		h.Token("orange", "orange2026"), nil)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode,
		"the purged request is not found")
}

// Without restoring the registry, DELAI_PORTAGE_NON_RESPECTE would block the
// number for three months and the purge would be useless for replaying a
// scenario.
func TestPurgeRestoresTheRegistryAndMakesTheNumberReplayable(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	tokenYAS := h.Token("yas", "yas2026")

	id := createPorting(h, "771000001")
	advanceTo(h, id, "ACTIVATION")
	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/traitement", tokenYAS,
		map[string]any{"idDemande": id})
	require.Equal(t, http.StatusOK, resp.StatusCode, body)

	current, portingDate := registryNumber(h, "771000001")
	require.Equal(t, operatorYAS, current, "the porting did transfer the number")
	require.NotNil(t, portingDate)

	_, body = h.Call(http.MethodDelete, purgePath, tokenYAS, nil)
	data := body["data"].(map[string]any)
	require.EqualValues(t, 1, data["demandesSupprimees"])
	require.EqualValues(t, 1, data["numerosRestaures"])

	current, portingDate = registryNumber(h, "771000001")
	require.Equal(t, operatorOrange, current, "the number rejoins its origin operator")
	require.Nil(t, portingDate, "date_dernier_portage cleared")

	// The scenario replays immediately: that is the purge's whole purpose.
	createPorting(h, "771000001")
}

// The OTP is consumed at creation; without a purge the same number could
// not be requested again without going back through otp/send — and the row
// would stay orphaned.
func TestPurgeErasesOTPOfPurgedNumbers(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	createPorting(h, "771000001")

	_, body := h.Call(http.MethodDelete, purgePath, h.Token("yas", "yas2026"), nil)
	require.EqualValues(t, 1, body["data"].(map[string]any)["otpSupprimes"])

	var n int
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM otp WHERE numero = '771000001'`).Scan(&n))
	require.Zero(t, n)
}

func TestPurgeWithNothingToPurgeSucceeds(t *testing.T) {
	h := routerharness.NewRouterHarness(t)

	resp, body := h.Call(http.MethodDelete, purgePath, h.Token("expresso", "expresso2026"), nil)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	data := body["data"].(map[string]any)
	require.EqualValues(t, 0, data["demandesSupprimees"])
	require.EqualValues(t, 0, data["numerosRestaures"])
	require.EqualValues(t, 0, data["otpSupprimes"])
	require.EqualValues(t, 0, data["reverseSupprimees"])
}

// The gateway gains no route: the 33-route invariant holds even with the
// purge enabled.
func TestPurgeAddsNothingToTheGateway(t *testing.T) {
	h := routerharness.NewRouterHarness(t)

	resp := h.Raw(http.MethodDelete, "/api/gateway/v1/demandes",
		h.Token("yas", "yas2026"), nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// reverse_request references demande without ON DELETE CASCADE: without
// explicit handling, its foreign key would block the deletion. The caller's
// reverse requests therefore leave with the rest of its test data.
func TestPurgeTakesReverseRequestsWithIt(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	tokenYAS := h.Token("yas", "yas2026")

	// 77900xxxx: held by ORANGE, originally YAS — YAS may request its reverse.
	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/reverse-requests", tokenYAS,
		map[string]any{"numero": "779000001"})
	require.Equal(t, http.StatusCreated, resp.StatusCode, body)

	_, body = h.Call(http.MethodDelete, purgePath, tokenYAS, nil)
	require.EqualValues(t, 1, body["data"].(map[string]any)["reverseSupprimees"])

	require.Empty(t, h.List("/api/gateway/v1/reverse-requests/mes-demandes", tokenYAS))
}
