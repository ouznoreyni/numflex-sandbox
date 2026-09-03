package controller_test

// GET /api/sandbox/v1/numeros/tranches, exercised through the real, live
// router — routerharness wires it exactly as cmd/server/main.go does — so a
// green run here proves the request really goes through SandboxController,
// CountNumberRangesInteractor and the Postgres aggregate. The harness seeds
// seed.TestVolumes: a thousand numbers per range, four thousand in a ported
// one, which is what the expectations below count.

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
)

const rangesPath = "/api/sandbox/v1/numeros/tranches"

// tranches reads the route's data payload for one operator.
func tranches(h *routerharness.RouterHarness, operator string) map[string]any {
	h.T.Helper()
	resp, body := h.Call(http.MethodGet, rangesPath+"?operateur="+operator,
		h.Token("yas", "yas2026"), nil)
	require.Equal(h.T, http.StatusOK, resp.StatusCode, body)
	return body["data"].(map[string]any)
}

// The route is mounted unconditionally, like the purge beside it.
func TestRangesAreMounted(t *testing.T) {
	h := routerharness.NewRouterHarness(t)

	data := tranches(h, "YAS")

	// YAS holds eight never-ported ranges, the historical 761, and the
	// already-ported 789 — ten in all, thirteen thousand numbers.
	require.Equal(t, float64(10), data["nombreTranches"])
	require.Equal(t, float64(13000), data["totalNumeros"])
	require.Equal(t, "YAS", data["operateur"])
}

// The nature of a range is read from its rows: 789 carries porting dates,
// 781 does not.
func TestRangesCarryTheirNature(t *testing.T) {
	h := routerharness.NewRouterHarness(t)

	nature := map[string]string{}
	bounds := map[string][2]string{}
	for _, e := range tranches(h, "YAS")["tranches"].([]any) {
		m := e.(map[string]any)
		nature[m["prefixe"].(string)] = m["nature"].(string)
		bounds[m["prefixe"].(string)] = [2]string{
			m["premier"].(string), m["dernier"].(string)}
	}

	require.Equal(t, "JAMAIS_PORTE", nature["781"])
	require.Equal(t, "DEJA_PORTE", nature["789"])
	require.Equal(t, [2]string{"781000000", "781000999"}, bounds["781"])
	require.Equal(t, [2]string{"789000000", "789003999"}, bounds["789"])
}

// Outside the ARTP contract there is no ANO-003 to reproduce: an unknown
// operator is the 400 a bean validation would give, naming what is
// accepted — not a 500.
func TestRangesRejectAnUnknownOperator(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	token := h.Token("yas", "yas2026")

	for _, query := range []string{"", "?operateur=", "?operateur=SONATEL"} {
		resp, body := h.Call(http.MethodGet, rangesPath+query, token, nil)

		require.Equal(t, http.StatusBadRequest, resp.StatusCode, query)
		require.Equal(t, "error.validation", body["message"], query)
		fields := body["fieldErrors"].([]any)
		require.Len(t, fields, 1)
		field := fields[0].(map[string]any)
		require.Equal(t, "operateur", field["field"])
		require.Contains(t, field["message"], "ORANGE")
	}
}

func TestRangesRequireAToken(t *testing.T) {
	h := routerharness.NewRouterHarness(t)

	resp := h.Raw(http.MethodGet, rangesPath+"?operateur=YAS", "", nil)

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
