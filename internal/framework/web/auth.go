package web

import (
	"github.com/gin-gonic/gin"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/controller"
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/token"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/web/middleware"
	authuc "github.com/ouznoreyni/numflex-sandbox/internal/usecase/auth"
)

// authController wires the clean-architecture authentication stack — a
// Postgres user gateway, the two auth interactors, a presenter — behind the
// two /api/authenticate routes. NewRouter calls it once, at router
// construction. Moved from internal/api/authentification.go (Task 18).
//
// The TokenIssuer closure is where internal/framework/token.Issue is bound
// to d.Cfg.JWTSecret and d.Cfg.JWTTTL — AuthenticateInteractor itself never
// imports the JWT library, which is what keeps the use case layer clean.
func (d *Deps) authController() *controller.AuthController {
	users := postgres.NewUserGateway(d.DB.Pool)

	issue := func(username string, roles []string) (string, error) {
		return token.Issue(d.Cfg.JWTSecret, d.Cfg.JWTTTL, username, roles)
	}
	authenticate := authuc.NewAuthenticate(users, issue, d.Cfg.JWTTTL)
	describe := authuc.NewDescribeCaller()

	return controller.NewAuthController(authenticate, describe, d.presenter())
}

// authenticator wires middleware.Authenticate against a Postgres
// UserGateway. Built once in NewRouter and reused at all three call sites
// the legacy internal/api.Deps.authentifier served: the GET
// /api/authenticate route, the gateway prefix guard, and the sandbox group.
// Moved from internal/api/authentification.go's authentifier (Task 18),
// renamed to match this package's English-language convention.
func (d *Deps) authenticator() gin.HandlerFunc {
	users := postgres.NewUserGateway(d.DB.Pool)
	return middleware.Authenticate(d.Cfg.JWTSecret, users, d.presenter())
}
