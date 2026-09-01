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

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/token"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/web/middleware"
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

func authRouter(t *testing.T, users fakeUsers) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/protege", middleware.Authenticate(secret, users), func(c *gin.Context) {
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

func TestJetonAbsentRendEnveloppeAccesInterdit(t *testing.T) {
	srv := authRouter(t, fakeUsers{})
	rep := get(t, srv, "")

	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
	var corps map[string]any
	require.NoError(t, json.NewDecoder(rep.Body).Decode(&corps))
	require.Equal(t, false, corps["success"])
	require.Equal(t, "ACCES_INTERDIT", corps["code"])
	require.Equal(t,
		"Token JWT absent, invalide ou expiré. Veuillez vous authentifier à nouveau.",
		corps["message"])
}

func TestJetonPorteurVideRendEnveloppeAccesInterdit(t *testing.T) {
	srv := authRouter(t, fakeUsers{})
	rep := get(t, srv, "Bearer ")

	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
	var corps map[string]any
	require.NoError(t, json.NewDecoder(rep.Body).Decode(&corps))
	require.Equal(t, "ACCES_INTERDIT", corps["code"])
}

func TestJetonInvalideRendCorpsVideSansContentType(t *testing.T) {
	// ANO-008 : jeton invalide → 401, corps vide, aucun Content-Type.
	srv := authRouter(t, fakeUsers{})
	rep := get(t, srv, "Bearer jeton.bidon.xxx")

	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
	require.Equal(t, "", rep.Header.Get("Content-Type"))
	require.Equal(t, int64(0), rep.ContentLength)
}

func TestJetonValideResoutAppelantEtAppelleSuivant(t *testing.T) {
	jeton, err := token.Issue(secret, time.Hour, "yas", []string{"ROLE_OPERATEUR_ADMIN"})
	require.NoError(t, err)

	users := fakeUsers{byUsername: map[string]entity.Caller{
		"yas": {UserID: "u1", Username: "yas", OperatorID: "op-yas", OperatorName: "YAS"},
	}}
	srv := authRouter(t, users)
	rep := get(t, srv, "Bearer "+jeton)

	require.Equal(t, http.StatusOK, rep.StatusCode)
	var corps map[string]any
	require.NoError(t, json.NewDecoder(rep.Body).Decode(&corps))
	require.Equal(t, "op-yas", corps["operatorId"])
}

// Un jeton valide dont le sujet ne résout plus à un utilisateur — compte
// supprimé entre l'émission et l'appel — est, du point de vue de l'appelant,
// indistinguable d'un jeton invalide : même 401 à corps vide.
func TestJetonValideUtilisateurIntrouvableRendCorpsVide(t *testing.T) {
	jeton, err := token.Issue(secret, time.Hour, "fantome", nil)
	require.NoError(t, err)

	srv := authRouter(t, fakeUsers{})
	rep := get(t, srv, "Bearer "+jeton)

	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
	require.Equal(t, "", rep.Header.Get("Content-Type"))
}

func TestJetonMalFormeSansPrefixeBearerRendEnveloppeAccesInterdit(t *testing.T) {
	srv := authRouter(t, fakeUsers{})
	rep := get(t, srv, "jeton-sans-prefixe")

	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
	var corps map[string]any
	require.NoError(t, json.NewDecoder(rep.Body).Decode(&corps))
	require.Equal(t, "ACCES_INTERDIT", corps["code"])
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
