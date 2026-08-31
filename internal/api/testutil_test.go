package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/config"
	"github.com/yas/numflex-sandbox/internal/httpx"
	_ "github.com/yas/numflex-sandbox/internal/seed"
	"github.com/yas/numflex-sandbox/internal/store"
)

type harnais struct {
	t   *testing.T
	srv *httptest.Server
	cfg *config.Config
	db  *store.DB
}

// nouveauHarnais monte le serveur complet sur une base de test ensemencée,
// en profil déterministe sauf réglages explicites.
func nouveauHarnais(t *testing.T, ajuste ...func(*config.Config)) *harnais {
	t.Helper()
	db := store.NewTestDB(t)

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
	for _, f := range ajuste {
		f(cfg)
	}

	// Le champ Moteur reste nil jusqu'à la Task 9, qui met ce harnais à jour.
	d := &Deps{Cfg: cfg, DB: db, R: httpx.NewRenderer(cfg.Fidelity, cfg.ClockSkew)}
	srv := httptest.NewServer(NewRouter(d))
	t.Cleanup(srv.Close)

	return &harnais{t: t, srv: srv, cfg: cfg, db: db}
}

// jeton authentifie un compte du seed et retourne son id_token.
func (h *harnais) jeton(username, motDePasse string) string {
	h.t.Helper()
	rep := h.brut(http.MethodPost, "/api/authenticate", "", map[string]any{
		"username": username, "password": motDePasse, "rememberMe": false,
	})
	require.Equal(h.t, http.StatusOK, rep.StatusCode)

	var corps struct {
		IDToken string `json:"id_token"`
	}
	require.NoError(h.t, json.NewDecoder(rep.Body).Decode(&corps))
	require.NotEmpty(h.t, corps.IDToken)
	return corps.IDToken
}

func (h *harnais) brut(methode, chemin, jeton string, corps any) *http.Response {
	h.t.Helper()
	var body *bytes.Reader
	if corps != nil {
		b, err := json.Marshal(corps)
		require.NoError(h.t, err)
		body = bytes.NewReader(b)
	} else {
		body = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(methode, h.srv.URL+chemin, body)
	require.NoError(h.t, err)
	req.Header.Set("Content-Type", "application/json")
	if jeton != "" {
		req.Header.Set("Authorization", "Bearer "+jeton)
	}
	rep, err := http.DefaultClient.Do(req)
	require.NoError(h.t, err)
	h.t.Cleanup(func() { rep.Body.Close() })
	return rep
}

// appel exécute une requête authentifiée et décode le corps en map.
func (h *harnais) appel(methode, chemin, jeton string, corps any) (*http.Response, map[string]any) {
	h.t.Helper()
	rep := h.brut(methode, chemin, jeton, corps)
	var decode map[string]any
	_ = json.NewDecoder(rep.Body).Decode(&decode)
	return rep, decode
}

// liste exécute un GET authentifié dont data est un tableau.
func (h *harnais) liste(chemin, jeton string) []any {
	h.t.Helper()
	rep, corps := h.appel(http.MethodGet, chemin, jeton, nil)
	require.Equal(h.t, http.StatusOK, rep.StatusCode, chemin)
	data, ok := corps["data"].([]any)
	require.Truef(h.t, ok, "%s : data n'est pas un tableau (%v)", chemin, corps)
	return data
}
