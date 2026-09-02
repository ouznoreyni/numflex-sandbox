package web

import (
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/controller"
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/clock"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/otp"
)

// otpController wires the clean-architecture OTP stack — gateway, clock,
// interactors, presenter — behind the live router. NewRouter calls it once,
// at router construction, and registers its Send/Verify methods directly.
// It reads d.DB.Pool eagerly, which is safe because every real Deps — the
// one cmd/server/main.go builds, and the ones tests build — carries a
// non-nil *persistence.DB. Moved from internal/api/otp.go (Task 18).
func (d *Deps) otpController() *controller.OTPController {
	gw := postgres.NewOTPGateway(d.DB.Pool)
	clk := clock.New(d.Cfg.ClockSkew)

	send := otp.NewSendOTP(gw, clk, d.Cfg.OTPStaticCode, d.Cfg.OTPTTL)
	verify := otp.NewVerifyOTP(gw, clk, d.Cfg.OTPMaxAttempts)

	return controller.NewOTPController(send, verify, d.presenter())
}
