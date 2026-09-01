package api

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/controller"
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/presenter"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/clock"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/otp"
)

// otpController wires the clean-architecture OTP stack — gateway, clock,
// interactors, presenter — behind the live router. NewRouter calls it once
// per request (from a closure), not once at router construction: the
// pieces it wires are cheap and stateless (they only hold a *pgxpool.Pool
// and configuration values), and building it lazily lets NewRouter stay
// callable with a Deps that carries no DB — internal/api/cors_test.go's
// TestLeCORSNAjouteAucuneRoute builds exactly such a Deps to count
// registered routes, without ever issuing a request.
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

// verifierOTP is the bridge internal/api/demandes_creation.go still calls
// (individual and fleet creation, Task 12 moves both) — kept per R1 so this
// task does not have to touch demandes_creation.go. It delegates to the
// VerifyOTP interactor rather than re-implementing the check: no SQL, no
// business rule left in this package.
func (d *Deps) verifierOTP(ctx context.Context, numero, code string) *entity.Fault {
	gw := postgres.NewOTPGateway(d.DB.Pool)
	clk := clock.New(d.Cfg.ClockSkew)
	interactor := otp.NewVerifyOTP(gw, clk, d.Cfg.OTPMaxAttempts)
	return interactor.Execute(ctx, otp.VerifyOTPInput{MSISDN: numero, Code: code})
}
