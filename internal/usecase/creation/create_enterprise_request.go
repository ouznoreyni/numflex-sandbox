package creation

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/otp"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// CreateEnterpriseRequestInput carries POST /demandes/entreprise's body,
// already bound and shape-validated by the controller. NumbersFleet's
// emptiness is deliberately not rejected by the controller: guide §9
// reserves FLOTTE_VIDE for exactly this case, and treating it as a bean-
// validation violation would make that code unreachable (see Execute).
type CreateEnterpriseRequestInput struct {
	FleetMSISDN         string // numeroPorteurFlotte — the single OTP challenge covers the whole fleet
	OTPCode             string
	SourceOperatorID    string
	RecipientOperatorID string
	Processus           string // PREPAID or POSTPAID
	FleetNumbers        []string
	Client              ClientInput
}

// ExcludedNumber names one fleet member left out of the request, and why —
// BR-006 / invariant 11: the fleet still succeeds with fewer numbers than
// requested, and the client must be told which and why.
type ExcludedNumber struct {
	MSISDN    string
	Reason    string
	ErrorCode string
}

// CreateEnterpriseRequestOutput needs no read-back: unlike the particulier
// and restitution endpoints, the fleet response is built entirely from what
// the interactor already knows, exactly as the legacy handler did.
type CreateEnterpriseRequestOutput struct {
	ID            string
	RetainedCount int
	Excluded      []ExcludedNumber
}

// CreateEnterpriseRequestBoundary is the interface a controller drives.
type CreateEnterpriseRequestBoundary interface {
	Execute(ctx context.Context, in CreateEnterpriseRequestInput) (CreateEnterpriseRequestOutput, *entity.Fault)
}

// CreateEnterpriseRequestInteractor implements CreateEnterpriseRequestBoundary.
type CreateEnterpriseRequestInteractor struct {
	verify  otp.VerifyOTPBoundary
	numbers port.NumberGateway
	uow     port.UnitOfWork
	ids     port.IDGenerator
	clock   port.Clock
}

// NewCreateEnterpriseRequest wires an interactor against its dependencies.
func NewCreateEnterpriseRequest(
	verify otp.VerifyOTPBoundary,
	numbers port.NumberGateway,
	uow port.UnitOfWork,
	ids port.IDGenerator,
	clock port.Clock,
) *CreateEnterpriseRequestInteractor {
	return &CreateEnterpriseRequestInteractor{
		verify: verify, numbers: numbers, uow: uow, ids: ids, clock: clock,
	}
}

// Execute reproduces internal/api/demandes_creation.go's postDemandeEntreprise
// order of checks exactly: fleet emptiness, then caller must be the
// recipient, then the single OTP (checked once, on the fleet's carrier
// number, covering every member), then per-number state, then the
// mixed-operator guard, then per-number eligibility (partitioning into
// retained/excluded rather than failing outright), then — only if at least
// one number survived — the single transaction.
func (i *CreateEnterpriseRequestInteractor) Execute(
	ctx context.Context, in CreateEnterpriseRequestInput,
) (CreateEnterpriseRequestOutput, *entity.Fault) {
	// §9 : le catalogue réserve un code à ce cas précis. Le traiter comme une
	// violation de bean validation le rendrait inatteignable.
	if len(in.FleetNumbers) == 0 {
		return CreateEnterpriseRequestOutput{}, entity.FleetEmpty()
	}

	caller := port.CallerFromContext(ctx)
	if in.RecipientOperatorID != caller.OperatorID {
		return CreateEnterpriseRequestOutput{}, entity.RequestAccessDenied(
			"L'opérateur connecté doit être l'opérateur destinataire de la demande.")
	}

	if f := i.verify.Execute(ctx, otp.VerifyOTPInput{MSISDN: in.FleetMSISDN, Code: in.OTPCode}); f != nil {
		return CreateEnterpriseRequestOutput{}, f
	}

	states := make(map[string]entity.NumberState, len(in.FleetNumbers))
	for _, numero := range in.FleetNumbers {
		state, found, err := i.numbers.State(ctx, numero)
		if err != nil {
			return CreateEnterpriseRequestOutput{}, entity.InternalError("lecture du numéro")
		}
		if !found {
			return CreateEnterpriseRequestOutput{}, entity.IncorrectSourceOperator()
		}
		states[numero] = state
	}
	for _, numero := range in.FleetNumbers {
		if states[numero].CurrentOperatorID != states[in.FleetNumbers[0]].CurrentOperatorID {
			return CreateEnterpriseRequestOutput{}, entity.FleetMixedOperators()
		}
	}

	var retained []string
	excluded := []ExcludedNumber{}
	for _, numero := range in.FleetNumbers {
		if f := entity.CheckPortingEligibility(states[numero], in.SourceOperatorID,
			in.RecipientOperatorID, entity.DelayBetweenPortings); f != nil {
			excluded = append(excluded, ExcludedNumber{MSISDN: numero, Reason: f.Message, ErrorCode: f.Code})
			continue
		}
		retained = append(retained, numero)
	}
	if len(retained) == 0 {
		return CreateEnterpriseRequestOutput{}, entity.NoEligibleNumber()
	}

	id := i.ids.NewID()
	now := i.clock.Now()
	processus := in.Processus
	var companyName, rcNumber *string
	if in.Client.CompanyName != "" {
		companyName = &in.Client.CompanyName
	}
	if in.Client.RCNumber != "" {
		rcNumber = &in.Client.RCNumber
	}

	err := i.uow.Do(ctx, func(repos port.Repositories) error {
		prefix, err := repos.Requests.RoutingPrefix(ctx, in.SourceOperatorID)
		if err != nil {
			return entity.ValidationFailed("Opérateur source inconnu")
		}
		if err := repos.Requests.Create(ctx, port.CreateRequestInput{
			ID: id, MSISDN: in.FleetMSISDN,
			SubscriberType:   string(entity.SubscriberEnterprise),
			RequestType:      string(entity.RequestTypePorting),
			SourceOperatorID: in.SourceOperatorID, RecipientOperatorID: in.RecipientOperatorID,
			CreatorOperatorID: in.RecipientOperatorID,
			Processus:         &processus,
			RoutingInfo:       &prefix,
			RequestDate:       now,
		}); err != nil {
			return entity.InternalError("création de la demande")
		}
		for _, numero := range retained {
			if err := repos.Requests.AddNumber(ctx, port.RequestNumberInput{
				RequestID: id, MSISDN: numero, RoutingInfo: &prefix,
			}); err != nil {
				return entity.InternalError("enregistrement du numéro")
			}
		}
		for _, ex := range excluded {
			if err := repos.Requests.AddExcludedNumber(ctx, port.ExcludedNumberInput{
				RequestID: id, MSISDN: ex.MSISDN, Reason: ex.Reason, ErrorCode: ex.ErrorCode,
			}); err != nil {
				return entity.InternalError("enregistrement du numéro exclu")
			}
		}
		if err := repos.Requests.AddClient(ctx, port.ClientInput{
			RequestID: id, LastName: in.Client.LastName, FirstName: in.Client.FirstName,
			BirthDate: in.Client.BirthDate, BirthPlace: in.Client.BirthPlace,
			IDType: in.Client.IDType, IDNumber: in.Client.IDNumber,
			CompanyName: companyName, RCNumber: rcNumber,
		}); err != nil {
			return entity.InternalError("enregistrement du client")
		}
		if err := repos.OTP.Consume(ctx, in.FleetMSISDN); err != nil {
			return entity.InternalError("consommation de l'OTP")
		}
		return nil
	})
	if err != nil {
		return CreateEnterpriseRequestOutput{}, entity.FaultFrom(err)
	}

	return CreateEnterpriseRequestOutput{ID: id, RetainedCount: len(retained), Excluded: excluded}, nil
}
