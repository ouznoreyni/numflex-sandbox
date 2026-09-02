// Package test is a package distinct from internal/framework/web: it cannot
// import that package's own test helpers (_test.go files, not exported
// outside their package). This file therefore locally reimplements Task 9's
// harness (originally internal/api/testutil_test.go), adding stepStatus,
// holder and postRaw — the only additions this task's end-to-end scenarios
// need — then, in task 18, advanceTo and createPorting to host
// captured_responses_test.go, moved from internal/api (deleted).
package test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/engine"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/seed"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/web"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport"
	"github.com/stretchr/testify/require"
)

type harness struct {
	t      *testing.T
	srv    *httptest.Server
	cfg    *config.Config
	db     *persistence.DB
	engine *engine.Engine
}

// newHarness mounts the full server against a seeded test database, in a
// deterministic profile (zero convergence and latency) unless overridden
// explicitly.
func newHarness(t *testing.T, adjust ...func(*config.Config)) *harness {
	t.Helper()
	db := testsupport.NewTestDB(t)

	cfg := &config.Config{
		Port:           "0",
		JWTSecret:      "test-secret",
		JWTTTL:         24 * time.Hour,
		Fidelity:       config.FidelityReal,
		EngineTick:     10 * time.Millisecond,
		OTPStaticCode:  "123456",
		OTPTTL:         5 * time.Minute,
		OTPMaxAttempts: 3,
	}
	for _, f := range adjust {
		f(cfg)
	}

	eng := engine.New(cfg, db)
	d := &web.Deps{
		Cfg: cfg, DB: db,
		Engine: eng,
	}
	srv := httptest.NewServer(web.NewRouter(d))
	t.Cleanup(srv.Close)

	return &harness{t: t, srv: srv, cfg: cfg, db: db, engine: eng}
}

// converge triggers one explicit engine pass. Every scenario except the
// expiration one drives the engine this way rather than waiting on its
// ticker.
func (h *harness) converge() {
	h.t.Helper()
	require.NoError(h.t, h.engine.Tick(context.Background()))
}

func (h *harness) step(id string) string {
	h.t.Helper()
	var e string
	require.NoError(h.t, h.db.Pool.QueryRow(context.Background(),
		"SELECT etape_actuelle FROM demande WHERE id = $1", id).Scan(&e))
	return e
}

func (h *harness) requestStatus(id string) string {
	h.t.Helper()
	var s string
	require.NoError(h.t, h.db.Pool.QueryRow(context.Background(),
		"SELECT statut_demande FROM demande WHERE id = $1", id).Scan(&s))
	return s
}

// stepStatus reads statut_etape_actuel — distinct from requestStatus, which
// reads statut_demande. It is the one that carries EXPIRE when a step was
// closed by the engine for lack of an operator action.
func (h *harness) stepStatus(id string) string {
	h.t.Helper()
	var s string
	require.NoError(h.t, h.db.Pool.QueryRow(context.Background(),
		"SELECT statut_etape_actuel FROM demande WHERE id = $1", id).Scan(&s))
	return s
}

// holder reads a number's current operator in the national registry — the
// SIT's central side effect: a porting changes this field, with or without
// an operator action.
func (h *harness) holder(msisdn string) string {
	h.t.Helper()
	var op string
	require.NoError(h.t, h.db.Pool.QueryRow(context.Background(),
		"SELECT operateur_actuel_id FROM numero WHERE msisdn = $1", msisdn).Scan(&op))
	return op
}

// token authenticates a seeded account and returns its id_token.
func (h *harness) token(username, password string) string {
	h.t.Helper()
	resp := h.raw(http.MethodPost, "/api/authenticate", "", map[string]any{
		"username": username, "password": password, "rememberMe": false,
	})
	require.Equal(h.t, http.StatusOK, resp.StatusCode)

	var body struct {
		IDToken string `json:"id_token"`
	}
	require.NoError(h.t, json.NewDecoder(resp.Body).Decode(&body))
	require.NotEmpty(h.t, body.IDToken)
	return body.IDToken
}

func (h *harness) raw(method, path, token string, body any) *http.Response {
	h.t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(h.t, err)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, h.srv.URL+path, reader)
	require.NoError(h.t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(h.t, err)
	h.t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// call executes an authenticated request and decodes the body into a map,
// without requiring any particular status.
func (h *harness) call(method, path, token string, body any) (*http.Response, map[string]any) {
	h.t.Helper()
	resp := h.raw(method, path, token, body)
	var parsed map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&parsed)
	return resp, parsed
}

// post executes an authenticated POST and requires a success status (2xx) —
// nominal scenarios do not have to check this status on every call.
func (h *harness) post(path, token string, body any) (*http.Response, map[string]any) {
	h.t.Helper()
	resp, parsed := h.call(http.MethodPost, path, token, body)
	require.Lessf(h.t, resp.StatusCode, 300, "%s: unexpected response %d (%v)",
		path, resp.StatusCode, parsed)
	return resp, parsed
}

// postRaw executes an authenticated POST and returns the response and its
// decoded body without requiring a success status — it is the call
// TestNoErrorCarriesCodeInRealMode uses, which triggers errors and inspects
// their shape.
func (h *harness) postRaw(path, token string, body any) (*http.Response, map[string]any) {
	h.t.Helper()
	return h.call(http.MethodPost, path, token, body)
}

// list executes an authenticated GET whose data is an array.
func (h *harness) list(path, token string) []any {
	h.t.Helper()
	resp, body := h.call(http.MethodGet, path, token, nil)
	require.Equal(h.t, http.StatusOK, resp.StatusCode, path)
	data, ok := body["data"].([]any)
	require.Truef(h.t, ok, "%s: data is not an array (%v)", path, body)
	return data
}

// individualBody builds a nominal individual request body ORANGE → YAS for
// a given number.
func individualBody(number string) map[string]any {
	return map[string]any{
		"numero":                  number,
		"otpCode":                 "123456",
		"operateurSourceId":       seed.OperatorOrangeID,
		"operateurDestinataireId": seed.OperatorYASID,
		"typePortabilite":         "PREPAID",
		"client": map[string]any{
			"nom": "Diallo", "prenom": "Mamadou",
			"dateNaissance": "1975-03-20", "lieuNaissance": "Dakar",
			"typePiece": "CNI", "numeroPiece": "1234567890123",
		},
	}
}

// advanceTo walks a request forward to the wanted step by writing to the
// database directly — the processing endpoints are tested elsewhere. Moved
// from internal/api/testutil_test.go (Task 18, alongside
// captured_responses_test.go).
func (h *harness) advanceTo(id, step string) {
	h.t.Helper()
	_, err := h.db.Pool.Exec(context.Background(),
		`UPDATE demande SET etape_actuelle = $2, statut_etape_actuel = 'EN_COURS',
		                    date_debut_etape = now(), transition_prevue_a = NULL
		  WHERE id = $1`, id, step)
	require.NoError(h.t, err)
}

// createPorting sends the OTP then creates an individual request ORANGE →
// YAS through the live router. Moved from internal/api/testutil_test.go
// (Task 18, alongside captured_responses_test.go).
func (h *harness) createPorting(number string) string {
	h.t.Helper()
	tok := h.token("yas", "yas2026")
	h.call(http.MethodPost, "/api/gateway/v1/otp/send", tok, map[string]any{"numero": number})

	resp, body := h.call(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		tok, individualBody(number))
	require.Equal(h.t, http.StatusCreated, resp.StatusCode, body)

	data := body["data"].(map[string]any)
	return data["id"].(string)
}
