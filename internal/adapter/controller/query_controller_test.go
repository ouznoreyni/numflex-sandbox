package controller_test

// These 12 test functions are moved, unchanged in assertion, from the
// deleted internal/api/lecture_test.go (Task 12). They still exercise the
// real, live router — routerharness.NewRouterHarness wraps api.NewRouter,
// wired exactly as cmd/server/main.go wires it — so a green run here proves
// a real HTTP request to one of the seven read-only routes goes through the
// new QueryController and not through any leftover handler. createPorting,
// individualBody, operatorOrange and operatorYAS are the free functions
// and constants creation_particulier_test.go already defines in this same
// package — reused, not copied.

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
)

// advanceTo advances a request to the wanted step by manipulating the
// database directly — the processing endpoints are tested elsewhere.
func advanceTo(h *routerharness.RouterHarness, id, step string) {
	h.T.Helper()
	_, err := h.DB.Pool.Exec(context.Background(),
		`UPDATE demande SET etape_actuelle = $2, statut_etape_actuel = 'EN_COURS',
		                    date_debut_etape = now(), transition_prevue_a = NULL
		  WHERE id = $1`, id, step)
	require.NoError(h.T, err)
}

func TestOwnRequestsSeesSourceAndRecipient(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")

	for _, account := range [][2]string{{"yas", "yas2026"}, {"orange", "orange2026"}} {
		data := h.List("/api/gateway/v1/demandes/mes-demandes", h.Token(account[0], account[1]))
		require.Len(t, data, 1, account[0])
		require.Equal(t, id, data[0].(map[string]any)["id"])
	}

	// EXPRESSO is not a party to anything.
	require.Empty(t, h.List("/api/gateway/v1/demandes/mes-demandes",
		h.Token("expresso", "expresso2026")))
}

func TestOwnRequestsDoesNotAcceptPagination(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	createPorting(h, "771000001")

	_, body := h.Call(http.MethodGet, "/api/gateway/v1/demandes/mes-demandes",
		h.Token("yas", "yas2026"), nil)

	require.NotContains(t, body, "page")
	require.NotContains(t, body, "size")
	require.NotContains(t, body, "totalElements")
}

func TestToAcceptIsReservedToTheSource(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")

	data := h.List("/api/gateway/v1/demandes/a-accepter", h.Token("orange", "orange2026"))
	require.Len(t, data, 1)
	require.Equal(t, id, data[0].(map[string]any)["id"])

	require.Empty(t, h.List("/api/gateway/v1/demandes/a-accepter", h.Token("yas", "yas2026")))
}

func TestToProcessFollowsTheStepOwner(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")

	advanceTo(h, id, "DESACTIVATION")
	require.Len(t, h.List("/api/gateway/v1/demandes/a-traiter", h.Token("orange", "orange2026")), 1)
	require.Empty(t, h.List("/api/gateway/v1/demandes/a-traiter", h.Token("yas", "yas2026")))

	advanceTo(h, id, "ACTIVATION")
	require.Len(t, h.List("/api/gateway/v1/demandes/a-traiter", h.Token("yas", "yas2026")), 1)
	require.Empty(t, h.List("/api/gateway/v1/demandes/a-traiter", h.Token("orange", "orange2026")))
}

func TestToConfirmContainsTheThirdParty(t *testing.T) {
	// D-6, measured at the SIT: EXPRESSO, neither source nor recipient, must confirm.
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")
	advanceTo(h, id, "CONFIRMATION")

	require.Len(t, h.List("/api/gateway/v1/demandes/a-confirmer",
		h.Token("orange", "orange2026")), 1)
	require.Len(t, h.List("/api/gateway/v1/demandes/a-confirmer",
		h.Token("expresso", "expresso2026")), 1)

	// The recipient is auto-confirmed: the request does not appear in its queue.
	require.Empty(t, h.List("/api/gateway/v1/demandes/a-confirmer",
		h.Token("yas", "yas2026")))
}

func TestToConfirmDetailRefusedToRecipient(t *testing.T) {
	// Measured: GET /a-confirmer/{id} with the recipient's token answers 500.
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")
	advanceTo(h, id, "CONFIRMATION")

	resp, _ := h.Call(http.MethodGet, "/api/gateway/v1/demandes/a-confirmer/"+id,
		h.Token("yas", "yas2026"), nil)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	resp, _ = h.Call(http.MethodGet, "/api/gateway/v1/demandes/a-confirmer/"+id,
		h.Token("orange", "orange2026"), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDetailUnknownRequest(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	resp, body := h.Call(http.MethodGet,
		"/api/gateway/v1/demandes/a-traiter/6a0000000000000000000000",
		h.Token("yas", "yas2026"), nil)

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "RuntimeException: Demande introuvable", body["detail"])
}

func TestInAndOutOnlyContainCompletedPortings(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")

	require.Empty(t, h.List("/api/gateway/v1/demandes/in", h.Token("yas", "yas2026")))

	_, err := h.DB.Pool.Exec(context.Background(),
		`UPDATE demande SET statut_demande = 'TERMINE', etape_actuelle = 'COMPLETION',
		                    statut_etape_actuel = 'VALIDE', date_finalisation = now()
		  WHERE id = $1`, id)
	require.NoError(t, err)

	data := h.List("/api/gateway/v1/demandes/in", h.Token("yas", "yas2026"))
	require.Len(t, data, 1)
	require.Equal(t, "TERMINE", data[0].(map[string]any)["statutDemande"])
	require.NotNil(t, data[0].(map[string]any)["dateFinalisation"])

	require.Len(t, h.List("/api/gateway/v1/demandes/out", h.Token("orange", "orange2026")), 1)
	require.Empty(t, h.List("/api/gateway/v1/demandes/out", h.Token("yas", "yas2026")))
}

func TestInExcludesRestitutions(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.Token("orange", "orange2026"), map[string]any{"numero": "773000001"})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	id := body["data"].(map[string]any)["id"].(string)

	_, err := h.DB.Pool.Exec(context.Background(),
		`UPDATE demande SET statut_demande = 'TERMINE', date_finalisation = now() WHERE id = $1`, id)
	require.NoError(t, err)

	require.Empty(t, h.List("/api/gateway/v1/demandes/in", h.Token("orange", "orange2026")),
		"/in only covers portings")
}

func TestListMessages(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	token := h.Token("yas", "yas2026")

	cases := map[string]string{
		"/api/gateway/v1/demandes/mes-demandes":    "Demandes récupérées avec succès",
		"/api/gateway/v1/demandes/a-accepter":      "Demandes à accepter récupérées avec succès",
		"/api/gateway/v1/demandes/a-traiter":       "Demandes à traiter récupérées avec succès",
		"/api/gateway/v1/demandes/a-confirmer":     "Demandes à confirmer récupérées avec succès",
		"/api/gateway/v1/demandes/deja-confirmees": "Demandes déjà confirmées récupérées avec succès",
		"/api/gateway/v1/demandes/in":              "Demandes IN récupérées avec succès",
		"/api/gateway/v1/demandes/out":             "Demandes OUT récupérées avec succès",
	}
	for path, message := range cases {
		_, body := h.Call(http.MethodGet, path, token, nil)
		require.Equalf(t, message, body["message"], path)
	}
}

// TestEmptyListSerializesAsEmptyArrayNeverNull proves, on the raw JSON body
// rather than on the decoded value, that a queue with no result renders
// "data":[] and never "data":null — the behavior rendreListe (before this
// task) and resolveViews/dtoList (since) guarantee by initializing their
// output slice non-nil before the loop that can fail before the first
// append. h.List alone would not distinguish this explicitly enough: its
// type-assertion body["data"].([]any) already fails on a JSON null
// (decoded as a nil interface{} on the Go side), so this assertion checks
// the exact byte of the response body, not merely a failed cast.
func TestEmptyListSerializesAsEmptyArrayNeverNull(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	// EXPRESSO is not a party to any request: all seven queues are empty.
	token := h.Token("expresso", "expresso2026")

	for _, path := range []string{
		"/api/gateway/v1/demandes/mes-demandes",
		"/api/gateway/v1/demandes/a-accepter",
		"/api/gateway/v1/demandes/a-traiter",
		"/api/gateway/v1/demandes/a-confirmer",
		"/api/gateway/v1/demandes/deja-confirmees",
		"/api/gateway/v1/demandes/in",
		"/api/gateway/v1/demandes/out",
	} {
		resp := h.Raw(http.MethodGet, path, token, nil)
		require.Equal(t, http.StatusOK, resp.StatusCode, path)

		var raw struct {
			Data json.RawMessage `json:"data"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&raw), path)
		require.JSONEqf(t, "[]", string(raw.Data), "%s: data must be [] not null", path)
	}
}
