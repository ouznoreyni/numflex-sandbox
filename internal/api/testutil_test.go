package api

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
	"github.com/ouznoreyni/numflex-sandbox/internal/httpx"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport"
	"github.com/stretchr/testify/require"
)

type harnais struct {
	t      *testing.T
	srv    *httptest.Server
	cfg    *config.Config
	db     *persistence.DB
	moteur *engine.Engine
}

// nouveauHarnais mounts the whole server on a seeded test database, in the
// deterministic profile unless told otherwise.
func nouveauHarnais(t *testing.T, ajuste ...func(*config.Config)) *harnais {
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
	for _, f := range ajuste {
		f(cfg)
	}

	mot := engine.New(cfg, db)
	d := &Deps{
		Cfg: cfg, DB: db,
		R:      httpx.NewRenderer(cfg.Fidelity, cfg.ClockSkew),
		Moteur: mot,
	}
	srv := httptest.NewServer(NewRouter(d))
	t.Cleanup(srv.Close)

	return &harnais{t: t, srv: srv, cfg: cfg, db: db, moteur: mot}
}

// avancerA walks a request forward to the wanted step by writing to the
// database directly — the processing endpoints are tested elsewhere. Moved
// from internal/api/lecture_test.go (deleted, Task 12): still used by
// annulation_test.go, confirmation_test.go, traitement_test.go,
// incidents_test.go, sandbox_test.go and conformite_captures_test.go, none of
// which migrates in this task.
func (h *harnais) avancerA(id, etape string) {
	h.t.Helper()
	_, err := h.db.Pool.Exec(context.Background(),
		`UPDATE demande SET etape_actuelle = $2, statut_etape_actuel = 'EN_COURS',
		                    date_debut_etape = now(), transition_prevue_a = NULL
		  WHERE id = $1`, id, etape)
	require.NoError(h.t, err)
}

// converger triggers one pass of the engine and checks no transition is left
// due. The tests drive the engine explicitly rather than waiting on its
// ticker.
func (h *harnais) converger() {
	h.t.Helper()
	require.NoError(h.t, h.moteur.Tick(context.Background()))
}

func (h *harnais) etape(id string) string {
	h.t.Helper()
	var e string
	require.NoError(h.t, h.db.Pool.QueryRow(context.Background(),
		"SELECT etape_actuelle FROM demande WHERE id = $1", id).Scan(&e))
	return e
}

func (h *harnais) statutDemande(id string) string {
	h.t.Helper()
	var s string
	require.NoError(h.t, h.db.Pool.QueryRow(context.Background(),
		"SELECT statut_demande FROM demande WHERE id = $1", id).Scan(&s))
	return s
}

// jeton authenticates a seeded account and returns its id_token.
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

// appel runs an authenticated request and decodes the body into a map.
func (h *harnais) appel(methode, chemin, jeton string, corps any) (*http.Response, map[string]any) {
	h.t.Helper()
	rep := h.brut(methode, chemin, jeton, corps)
	var decode map[string]any
	_ = json.NewDecoder(rep.Body).Decode(&decode)
	return rep, decode
}

// corpsParticulier builds the body of a valid individual request, ORANGE →
// YAS — shared by creerPortage and by the tests that still need to build that
// body themselves (moved from internal/api/creation_particulier_test.go,
// deleted in Task 12: this copy now serves only the api package's other
// capabilities, not yet migrated, which use creerPortage to prime their
// fixtures).
func corpsParticulier(numero string) map[string]any {
	return map[string]any{
		"numero":                  numero,
		"otpCode":                 "123456",
		"operateurSourceId":       seed.OperateurOrange,
		"operateurDestinataireId": seed.OperateurYAS,
		"typePortabilite":         "PREPAID",
		"client": map[string]any{
			"nom": "Diallo", "prenom": "Mamadou",
			"dateNaissance": "1975-03-20", "lieuNaissance": "Dakar",
			"typePiece": "CNI", "numeroPiece": "1234567890123",
		},
	}
}

// corpsEntreprise builds the body of a valid fleet request, ORANGE → YAS —
// moved from internal/api/creation_entreprise_test.go, deleted in Task 12, for
// the other capabilities' tests (acceptance) that still need it to prime a
// fleet.
func corpsEntreprise(porteur string, flotte []string) map[string]any {
	return map[string]any{
		"numeroPorteurFlotte":     porteur,
		"otpCode":                 "123456",
		"operateurSourceId":       seed.OperateurOrange,
		"operateurDestinataireId": seed.OperateurYAS,
		"typePortabilite":         "POSTPAID",
		"numerosFlotte":           flotte,
		"client": map[string]any{
			"raisonSociale": "Entreprise SARL", "numRC": "123456789",
			"prenom": "Ousmane", "nom": "Diallo", "dateNaissance": "1975-03-20",
			"typePiece": "CNI", "numeroPiece": "1234567890123",
		},
	}
}

// creerPortage sends the OTP then creates an individual request ORANGE → YAS
// through the live router — so now through CreationController (Task 12), no
// differently than before as far as this method's callers are concerned.
func (h *harnais) creerPortage(numero string) string {
	h.t.Helper()
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton, map[string]any{"numero": numero})

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		jeton, corpsParticulier(numero))
	require.Equal(h.t, http.StatusCreated, rep.StatusCode, corps)

	data := corps["data"].(map[string]any)
	return data["id"].(string)
}

// liste runs an authenticated GET whose data is an array.
func (h *harnais) liste(chemin, jeton string) []any {
	h.t.Helper()
	rep, corps := h.appel(http.MethodGet, chemin, jeton, nil)
	require.Equal(h.t, http.StatusOK, rep.StatusCode, chemin)
	data, ok := corps["data"].([]any)
	require.Truef(h.t, ok, "%s : data n'est pas un tableau (%v)", chemin, corps)
	return data
}

// brutAvecEnTetes runs a request with arbitrary headers added — needed by the
// CORS tests, which turn on Origin and on the preflight.
func (h *harnais) brutAvecEnTetes(methode, chemin, jeton string, corps any,
	entetes map[string]string) *http.Response {
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
	for k, v := range entetes {
		req.Header.Set(k, v)
	}
	rep, err := http.DefaultClient.Do(req)
	require.NoError(h.t, err)
	h.t.Cleanup(func() { rep.Body.Close() })
	return rep
}
