// Le paquet test est un paquet distinct de internal/api : il ne peut pas
// importer les helpers de test de internal/api (fichiers _test.go, non
// exportés hors de leur paquet). Ce fichier reprend donc localement le
// harnais de la Task 9 (internal/api/testutil_test.go), en y ajoutant
// statutEtape, detenteur et postBrut — les seuls ajouts nécessaires aux
// scénarios de bout en bout de cette tâche.
package test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/api"
	"github.com/yas/numflex-sandbox/internal/config"
	"github.com/yas/numflex-sandbox/internal/engine"
	"github.com/yas/numflex-sandbox/internal/httpx"
	"github.com/yas/numflex-sandbox/internal/seed"
	"github.com/yas/numflex-sandbox/internal/store"
)

type harnais struct {
	t      *testing.T
	srv    *httptest.Server
	cfg    *config.Config
	db     *store.DB
	moteur *engine.Engine
}

// nouveauHarnais monte le serveur complet sur une base de test ensemencée, en
// profil déterministe (convergence et latences nulles) sauf réglages explicites.
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

	mot := engine.New(cfg, db)
	d := &api.Deps{
		Cfg: cfg, DB: db,
		R:      httpx.NewRenderer(cfg.Fidelity, cfg.ClockSkew),
		Moteur: mot,
	}
	srv := httptest.NewServer(api.NewRouter(d))
	t.Cleanup(srv.Close)

	return &harnais{t: t, srv: srv, cfg: cfg, db: db, moteur: mot}
}

// converger déclenche un passage explicite du moteur. Tous les scénarios sauf
// celui de l'expiration pilotent le moteur ainsi plutôt que d'attendre son ticker.
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

// statutEtape lit statut_etape_actuel — distinct de statutDemande, qui lit
// statut_demande. C'est lui qui porte EXPIRE quand une étape a été soldée par
// le moteur faute d'action d'un opérateur.
func (h *harnais) statutEtape(id string) string {
	h.t.Helper()
	var s string
	require.NoError(h.t, h.db.Pool.QueryRow(context.Background(),
		"SELECT statut_etape_actuel FROM demande WHERE id = $1", id).Scan(&s))
	return s
}

// detenteur lit l'opérateur actuel d'un numéro au registre national — c'est
// l'effet de bord central du SIT : un portage change ce champ, avec ou sans
// action d'un opérateur.
func (h *harnais) detenteur(msisdn string) string {
	h.t.Helper()
	var op string
	require.NoError(h.t, h.db.Pool.QueryRow(context.Background(),
		"SELECT operateur_actuel_id FROM numero WHERE msisdn = $1", msisdn).Scan(&op))
	return op
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

// appel exécute une requête authentifiée et décode le corps en map, sans
// exiger de statut particulier.
func (h *harnais) appel(methode, chemin, jeton string, corps any) (*http.Response, map[string]any) {
	h.t.Helper()
	rep := h.brut(methode, chemin, jeton, corps)
	var decode map[string]any
	_ = json.NewDecoder(rep.Body).Decode(&decode)
	return rep, decode
}

// post exécute un POST authentifié et exige un statut de succès (2xx) — les
// scénarios nominaux n'ont pas à vérifier ce statut à chaque appel.
func (h *harnais) post(chemin, jeton string, corps any) (*http.Response, map[string]any) {
	h.t.Helper()
	rep, decode := h.appel(http.MethodPost, chemin, jeton, corps)
	require.Lessf(h.t, rep.StatusCode, 300, "%s : réponse inattendue %d (%v)",
		chemin, rep.StatusCode, decode)
	return rep, decode
}

// postBrut exécute un POST authentifié et rend la réponse et son corps décodé
// sans exiger de statut de succès — c'est l'appel de TestAucuneErreurNePorteDeCodeEnModeReel,
// qui provoque des erreurs et inspecte leur forme.
func (h *harnais) postBrut(chemin, jeton string, corps any) (*http.Response, map[string]any) {
	h.t.Helper()
	return h.appel(http.MethodPost, chemin, jeton, corps)
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

// corpsParticulier construit le corps d'une demande particulier nominale
// ORANGE → YAS pour un numéro donné.
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
