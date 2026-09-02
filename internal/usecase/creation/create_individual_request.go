package creation

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/otp"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// CreateIndividualRequestInput carries POST /demandes/particulier's body,
// already bound and shape-validated by the controller.
type CreateIndividualRequestInput struct {
	MSISDN              string
	OTPCode             string
	SourceOperatorID    string
	RecipientOperatorID string
	Process             string // PREPAID or POSTPAID
	Client              ClientInput
}

// CreateIndividualRequestBoundary is the interface a controller drives.
type CreateIndividualRequestBoundary interface {
	Execute(ctx context.Context, in CreateIndividualRequestInput) (port.RequestView, *entity.Fault)
}

// CreateIndividualRequestInteractor implements CreateIndividualRequestBoundary.
type CreateIndividualRequestInteractor struct {
	verify   otp.VerifyOTPBoundary
	numbers  port.NumberGateway
	uow      port.UnitOfWork
	requests port.RequestGateway // pool-bound: only Get, after commit
	ids      port.IDGenerator
	clock    port.Clock
}

// NewCreateIndividualRequest wires an interactor against its dependencies.
// verify is the existing VerifyOTP interactor, reused rather than
// re-implemented: TC-021 (pre-verify without consuming) is its rule to own,
// not this package's to restate.
func NewCreateIndividualRequest(
	verify otp.VerifyOTPBoundary,
	numbers port.NumberGateway,
	uow port.UnitOfWork,
	requests port.RequestGateway,
	ids port.IDGenerator,
	clock port.Clock,
) *CreateIndividualRequestInteractor {
	return &CreateIndividualRequestInteractor{
		verify: verify, numbers: numbers, uow: uow,
		requests: requests, ids: ids, clock: clock,
	}
}

// Execute reproduces internal/api/demandes_creation.go's postDemandeParticulier
// order of checks exactly: caller must be the recipient, then OTP, then
// eligibility, then the single transaction that inserts the request, its
// number and its client, and consumes the OTP — all four or none.
func (i *CreateIndividualRequestInteractor) Execute(
	ctx context.Context, in CreateIndividualRequestInput,
) (port.RequestView, *entity.Fault) {
	caller := port.CallerFromContext(ctx)
	if in.RecipientOperatorID != caller.OperatorID {
		return port.RequestView{}, entity.RequestAccessDenied(
			"L'opérateur connecté doit être l'opérateur destinataire de la demande.")
	}

	if f := i.verify.Execute(ctx, otp.VerifyOTPInput{MSISDN: in.MSISDN, Code: in.OTPCode}); f != nil {
		return port.RequestView{}, f
	}

	state, found, err := i.numbers.State(ctx, in.MSISDN)
	if err != nil {
		return port.RequestView{}, entity.InternalError("lecture du numéro")
	}
	if !found {
		return port.RequestView{}, entity.IncorrectSourceOperator()
	}
	if f := entity.CheckPortingEligibility(state, in.SourceOperatorID,
		in.RecipientOperatorID, entity.DelayBetweenPortings); f != nil {
		return port.RequestView{}, f
	}

	id := i.ids.NewID()
	now := i.clock.Now()
	process := in.Process

	err = i.uow.Do(ctx, func(repos port.Repositories) error {
		prefix, err := repos.Requests.RoutingPrefix(ctx, in.SourceOperatorID)
		if err != nil {
			return entity.ValidationFailed("Opérateur source inconnu")
		}
		if err := repos.Requests.Create(ctx, port.CreateRequestInput{
			ID: id, MSISDN: in.MSISDN,
			SubscriberType:   string(entity.SubscriberIndividual),
			RequestType:      string(entity.RequestTypePorting),
			SourceOperatorID: in.SourceOperatorID, RecipientOperatorID: in.RecipientOperatorID,
			CreatorOperatorID: in.RecipientOperatorID,
			Process:           &process,
			RoutingInfo:       &prefix,
			RequestDate:       now,
		}); err != nil {
			return entity.InternalError("création de la demande")
		}
		if err := repos.Requests.AddNumber(ctx, port.RequestNumberInput{
			RequestID: id, MSISDN: in.MSISDN, RoutingInfo: &prefix,
		}); err != nil {
			return entity.InternalError("enregistrement du numéro")
		}
		if err := repos.Requests.AddClient(ctx, port.ClientInput{
			RequestID: id, LastName: in.Client.LastName, FirstName: in.Client.FirstName,
			BirthDate: in.Client.BirthDate, BirthPlace: in.Client.BirthPlace,
			IDType: in.Client.IDType, IDNumber: in.Client.IDNumber,
		}); err != nil {
			return entity.InternalError("enregistrement du client")
		}
		// Last call of the transaction: if any of the previous three failed,
		// this one is never reached and the OTP stays consumable.
		if err := repos.OTP.Consume(ctx, in.MSISDN); err != nil {
			return entity.InternalError("consommation de l'OTP")
		}
		return nil
	})
	if err != nil {
		return port.RequestView{}, entity.FaultFrom(err)
	}

	view, found, err := i.requests.Get(ctx, id)
	if err != nil || !found {
		return port.RequestView{}, entity.InternalError("relecture de la demande")
	}
	return view, nil
}
