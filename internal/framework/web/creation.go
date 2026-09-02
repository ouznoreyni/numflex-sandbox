package web

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
// RequestGateway for the post-commit read-back), a UnitOfWork (carrying
// Repositories.Requests and Repositories.OTP bound to the same transaction
// inside Do), the reused VerifyOTP interactor, three interactors, a
// presenter — behind the three creation routes. NewRouter calls it once, at
// router construction. Moved from internal/api/creation.go (Task 18).
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
