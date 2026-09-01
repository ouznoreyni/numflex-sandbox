package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/presenter"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/token"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/web/middleware"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/inmemory"
)

const secret = "test-secret"

// fakeUsers is a map-backed port.UserGateway double, local to this test file
// so that Task 6 does not have to anticipate the shape Task 10 gives
// internal/testsupport/inmemory.NewUserGateway.
type fakeUsers struct {
	byUsername map[string]entity.Caller
}

func (f fakeUsers) ByCredentials(context.Context, string, string) (entity.Caller, bool, error) {
	panic("not used by Authenticate")
}

func (f fakeUsers) ByUsername(_ context.Context, username string) (entity.Caller, bool, error) {
	c, ok := f.byUsername[username]
	return c, ok, nil
}

// realPresenter is the default presenter for tests that don't exercise the
// "user not found" branch — the first two outcomes never reach it, and its
// choice of fidelity mode is therefore irrelevant to them.
func realPresenter() presenter.Presenter { return presenter.NewReal(inmemory.FixedClock{}) }

func authRouter(t *testing.T, users fakeUsers, p presenter.Presenter) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/protege", middleware.Authenticate(secret, users, p), func(c *gin.Context) {
		caller := middleware.CallerFrom(c)
		c.JSON(http.StatusOK, gin.H{"operatorId": caller.OperatorID})
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, srv *httptest.Server, bearer string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/protege", nil)
	require.NoError(t, err)
	if bearer != "" {
		req.Header.Set("Authorization", bearer)
	}
	rep, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { rep.Body.Close() })
	return rep
}

// Outcome 1 — header absent, malformed, or an empty Bearer token: 401 with
// the ARTP ACCES_INTERDIT envelope, regardless of fidelity mode.

func TestJetonAbsentRendEnveloppeAccesInterdit(t *testing.T) {
	srv := authRouter(t, fakeUsers{}, realPresenter())
	rep := get(t, srv, "")

	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
	var corps map[string]any
	require.NoError(t, json.NewDecoder(rep.Body).Decode(&corps))
	require.Equal(t, false, corps["success"])
	require.Equal(t, "ACCES_INTERDIT", corps["code"])
	require.Equal(t,
		"Token JWT absent, invalide ou expiré. Veuillez vous authentifier à nouveau.",
		corps["message"])
	require.Nil(t, corps["data"])
}

func TestJetonPorteurVideRendEnveloppeAccesInterdit(t *testing.T) {
	srv := authRouter(t, fakeUsers{}, realPresenter())
	rep := get(t, srv, "Bearer ")

	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
	var corps map[string]any
	require.NoError(t, json.NewDecoder(rep.Body).Decode(&corps))
	require.Equal(t, "ACCES_INTERDIT", corps["code"])
}

func TestJetonMalFormeSansPrefixeBearerRendEnveloppeAccesInterdit(t *testing.T) {
	srv := authRouter(t, fakeUsers{}, realPresenter())
	rep := get(t, srv, "jeton-sans-prefixe")

	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
	var corps map[string]any
	require.NoError(t, json.NewDecoder(rep.Body).Decode(&corps))
	require.Equal(t, "ACCES_INTERDIT", corps["code"])
}

// Outcome 2 — token present but invalid (bad signature, expired, wrong
// algorithm): 401, empty body, no Content-Type (ANO-008), regardless of
// fidelity mode — this branch never reaches the presenter.

func TestJetonInvalideRendCorpsVideSansContentType(t *testing.T) {
	srv := authRouter(t, fakeUsers{}, realPresenter())
	rep := get(t, srv, "Bearer jeton.bidon.xxx")

	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
	require.Equal(t, "", rep.Header.Get("Content-Type"))
	require.Equal(t, int64(0), rep.ContentLength)
}

// Outcome 3 — token valid but its subject no longer resolves to a user: the
// legacy middleware called d.R.Fail(c, entity.OperatorNotFound()), so this
// is the one outcome that depends on fidelity mode. Real mode: every
// non-validation fault falls through to a 500 problem+json with the
// "RuntimeException: " prefix — not a 401 at all. Contract mode: the ARTP
// envelope, at the status entity.FaultAccess maps to (403), carrying
// OPERATEUR_NON_TROUVE.

func TestJetonValideUtilisateurIntrouvableEnReelRend500ProblemJSON(t *testing.T) {
	jeton, err := token.Issue(secret, time.Hour, "fantome", nil)
	require.NoError(t, err)

	srv := authRouter(t, fakeUsers{}, presenter.NewReal(inmemory.FixedClock{}))
	rep := get(t, srv, "Bearer "+jeton)

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "application/problem+json", rep.Header.Get("Content-Type"))

	var corps map[string]any
	require.NoError(t, json.NewDecoder(rep.Body).Decode(&corps))
	require.Equal(t, "https://www.jhipster.tech/problem/problem-with-message", corps["type"])
	require.Equal(t, "Internal Server Error", corps["title"])
	require.Equal(t, float64(http.StatusInternalServerError), corps["status"])
	require.Equal(t, "RuntimeException: Votre compte n'est pas associé à un opérateur", corps["detail"])
	require.Equal(t, "/protege", corps["path"])
	require.Equal(t, "error.http.500", corps["message"])
	require.NotContains(t, corps, "code")
	require.NotContains(t, corps, "success")
}

func TestJetonValideUtilisateurIntrouvableEnContratRendEnveloppe403(t *testing.T) {
	jeton, err := token.Issue(secret, time.Hour, "fantome", nil)
	require.NoError(t, err)

	srv := authRouter(t, fakeUsers{}, presenter.NewContract(inmemory.FixedClock{}))
	rep := get(t, srv, "Bearer "+jeton)

	require.Equal(t, http.StatusForbidden, rep.StatusCode)

	var corps map[string]any
	require.NoError(t, json.NewDecoder(rep.Body).Decode(&corps))
	require.Equal(t, false, corps["success"])
	require.Equal(t, "OPERATEUR_NON_TROUVE", corps["code"])
	require.Equal(t, "Votre compte n'est pas associé à un opérateur", corps["message"])
	require.Nil(t, corps["data"])
}

// Valid token whose subject does resolve: Authenticate calls c.Next(), and
// CallerFrom returns what the gateway resolved — the positive path, kept
// alongside the three failure outcomes above.

func TestJetonValideResoutAppelantEtAppelleSuivant(t *testing.T) {
	jeton, err := token.Issue(secret, time.Hour, "yas", []string{"ROLE_OPERATEUR_ADMIN"})
	require.NoError(t, err)

	users := fakeUsers{byUsername: map[string]entity.Caller{
		"yas": {UserID: "u1", Username: "yas", OperatorID: "op-yas", OperatorName: "YAS"},
	}}
	srv := authRouter(t, users, realPresenter())
	rep := get(t, srv, "Bearer "+jeton)

	require.Equal(t, http.StatusOK, rep.StatusCode)
	var corps map[string]any
	require.NoError(t, json.NewDecoder(rep.Body).Decode(&corps))
	require.Equal(t, "op-yas", corps["operatorId"])
}

func TestCallerFromSansAuthentificationRendValeurZero(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/libre", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"operatorId": middleware.CallerFrom(c).OperatorID})
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	rep, err := http.Get(srv.URL + "/libre")
	require.NoError(t, err)
	t.Cleanup(func() { rep.Body.Close() })

	var corps map[string]any
	require.NoError(t, json.NewDecoder(rep.Body).Decode(&corps))
	require.Equal(t, "", corps["operatorId"])
}
