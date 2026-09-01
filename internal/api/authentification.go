package api

import (
	"github.com/gin-gonic/gin"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/controller"
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/presenter"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/clock"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/token"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/web/middleware"
	authuc "github.com/ouznoreyni/numflex-sandbox/internal/usecase/auth"
)

// authController wires the clean-architecture authentication stack — a
// Postgres user gateway, the two auth interactors, a presenter — behind the
// two /api/authenticate routes. NewRouter calls it once, at router
// construction, exactly as it does otpController (internal/api/otp.go):
// this is the strangler pattern Task 9 established, repeated capability by
// capability.
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

// authentifier wires Task 6's middleware.Authenticate against a Postgres
// UserGateway — the piece it was missing until this task made
// postgres.NewUserGateway exist. Built once in NewRouter and reused at all
// three call sites the legacy d.Authentifier() served: the GET
// /api/authenticate route, the gateway prefix guard, and the sandbox group.
func (d *Deps) authentifier() gin.HandlerFunc {
	users := postgres.NewUserGateway(d.DB.Pool)
	return middleware.Authenticate(d.Cfg.JWTSecret, users, d.presenter())
}

// presenter picks Real or Contract according to the configured fidelity
// mode — the same choice otpController makes, factored out because this
// file now makes it twice.
func (d *Deps) presenter() presenter.Presenter {
	clk := clock.New(d.Cfg.ClockSkew)
	if d.Cfg.Fidelity == config.FidelityContract {
		return presenter.NewContract(clk)
	}
	return presenter.NewReal(clk)
}
