package api

import (
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/controller"
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/presenter"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/clock"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/otp"
)

// otpController wires the clean-architecture OTP stack — gateway, clock,
// interactors, presenter — behind the live router. NewRouter calls it once,
// at router construction, and registers its Send/Verify methods directly:
// the pieces it wires are cheap and stateless today, but there will be
// eleven of these, and some will not stay cheap, so the pattern is
// build-once from the start rather than a per-request closure. It reads
// d.DB.Pool eagerly, which is safe because every real Deps — the one
// cmd/server/main.go builds, and the ones tests build — carries a non-nil
// *persistence.DB; a test that only wants to count registered routes
// (internal/api/cors_test.go's TestLeCORSNAjouteAucuneRoute) passes an
// empty, unopened *persistence.DB rather than nil, since it never issues a
// request that would dereference the pool.
//
// This is the strangler pattern the remaining eight capabilities repeat: the
// live router (internal/api/router.go) keeps registering routes, but each
// route's handler is now a controller from internal/adapter/controller
// rather than a *Deps method with SQL inline.
func (d *Deps) otpController() *controller.OTPController {
	gw := postgres.NewOTPGateway(d.DB.Pool)
	clk := clock.New(d.Cfg.ClockSkew)

	send := otp.NewSendOTP(gw, clk, d.Cfg.OTPStaticCode, d.Cfg.OTPTTL)
	verify := otp.NewVerifyOTP(gw, clk, d.Cfg.OTPMaxAttempts)

	var pres presenter.Presenter
	if d.Cfg.Fidelity == config.FidelityContract {
		pres = presenter.NewContract(clk)
	} else {
		pres = presenter.NewReal(clk)
	}

	return controller.NewOTPController(send, verify, pres)
}
