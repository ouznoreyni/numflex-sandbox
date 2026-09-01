package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/engine"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
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

// nouveauHarnais monte le serveur complet sur une base de test ensemencée,
// en profil déterministe sauf réglages explicites.
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

// converger déclenche un passage du moteur et vérifie qu'aucune transition ne
// reste due. Les tests pilotent le moteur explicitement plutôt que d'attendre
// son ticker.
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

// corpsParticulier construit le corps d'une demande particulier valide,
// ORANGE → YAS — partagé par creerPortage et par les tests qui ont encore
// besoin de construire ce corps eux-mêmes (déplacé depuis
// internal/api/creation_particulier_test.go, supprimé en Task 12 : cette
// copie ne sert plus qu'aux autres capacités du paquet api, pas encore
// migrées, qui utilisent creerPortage pour amorcer leurs fixtures).
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

// corpsEntreprise construit le corps d'une demande flotte valide, ORANGE →
// YAS — déplacé depuis internal/api/creation_entreprise_test.go, supprimé en
// Task 12, pour les tests d'autres capacités (acceptation) qui en ont
// encore besoin pour amorcer une flotte.
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

// creerPortage envoie l'OTP puis crée une demande particulier ORANGE → YAS,
// via le routeur vivant — donc désormais via CreationController (Task 12),
// pas différemment qu'avant pour les appelants de cette méthode.
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

// liste exécute un GET authentifié dont data est un tableau.
func (h *harnais) liste(chemin, jeton string) []any {
	h.t.Helper()
	rep, corps := h.appel(http.MethodGet, chemin, jeton, nil)
	require.Equal(h.t, http.StatusOK, rep.StatusCode, chemin)
	data, ok := corps["data"].([]any)
	require.Truef(h.t, ok, "%s : data n'est pas un tableau (%v)", chemin, corps)
	return data
}

// brutAvecEnTetes exécute une requête en ajoutant des en-têtes arbitraires —
// nécessaire pour les tests CORS, qui portent sur Origin et sur le préambule.
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
