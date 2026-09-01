package api

import (
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/controller"
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/clock"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/identifier"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/creation"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/otp"
)

// creationController wires the clean-architecture request-creation stack —
// two pool-bound gateways (NumberGateway for eligibility reads,
// RequestGateway for the post-commit read-back), a UnitOfWork (Task 5,
// carrying Repositories.Requests and Repositories.OTP bound to the same
// transaction inside Do), the reused VerifyOTP interactor, three
// interactors, a presenter — behind the three creation routes. NewRouter
// calls it once, at router construction, exactly as it does otpController,
// referenceController and authController: the same build-once rationale
// applies unchanged.
//
// This is the last stop of the strangler pattern for this capacity:
// internal/api/demandes_creation.go — the 468-line handler that opened a
// *pgx.Tx directly — is gone. port.UnitOfWork (Task 5) is what makes that
// possible: the interactors decide there is a transaction, this file's
// persistence.NewUnitOfWork decides what a transaction is, and no *pgx.Tx
// ever reaches internal/usecase/creation.
func (d *Deps) creationController() *controller.CreationController {
	numbers := postgres.NewNumberGateway(d.DB.Pool)
	requestsRead := postgres.NewRequestGateway(d.DB.Pool)
	uow := persistence.NewUnitOfWork(d.DB)
	ids := identifier.NewGenerator()
	clk := clock.New(d.Cfg.ClockSkew)

	verify := otp.NewVerifyOTP(postgres.NewOTPGateway(d.DB.Pool), clk, d.Cfg.OTPMaxAttempts)

	individual := creation.NewCreateIndividualRequest(verify, numbers, uow, requestsRead, ids, clk)
	enterprise := creation.NewCreateEnterpriseRequest(verify, numbers, uow, ids, clk)
	restitution := creation.NewCreateRestitutionRequest(numbers, uow, requestsRead, ids, clk)

	return controller.NewCreationController(individual, enterprise, restitution, d.presenter(), clk)
}
