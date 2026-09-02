package creation

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// CreateRestitutionRequestInput carries POST /demandes/restitution's body —
// a bare MSISDN, no OTP (§7.5: the guide requires none).
type CreateRestitutionRequestInput struct {
	MSISDN string
}

// CreateRestitutionRequestBoundary is the interface a controller drives.
type CreateRestitutionRequestBoundary interface {
	Execute(ctx context.Context, in CreateRestitutionRequestInput) (port.RequestView, *entity.Fault)
}

// CreateRestitutionRequestInteractor implements CreateRestitutionRequestBoundary.
type CreateRestitutionRequestInteractor struct {
	numbers  port.NumberGateway
	uow      port.UnitOfWork
	requests port.RequestGateway // pool-bound: only Get, after commit
	ids      port.IDGenerator
	clock    port.Clock
}

// NewCreateRestitutionRequest wires an interactor against its dependencies.
func NewCreateRestitutionRequest(
	numbers port.NumberGateway,
	uow port.UnitOfWork,
	requests port.RequestGateway,
	ids port.IDGenerator,
	clock port.Clock,
) *CreateRestitutionRequestInteractor {
	return &CreateRestitutionRequestInteractor{
		numbers: numbers, uow: uow, requests: requests, ids: ids, clock: clock,
	}
}

// Execute reproduces internal/api/demandes_creation.go's postDemandeRestitution
// order of checks exactly: number state, then the [HYP] role choice — the
// caller must be the number's origin operator, who becomes the request's
// recipient — then restitution eligibility, then a two-insert transaction
// (no OTP consumption: a restitution never challenges one; no client row: a
// restitution carries no identity).
func (i *CreateRestitutionRequestInteractor) Execute(
	ctx context.Context, in CreateRestitutionRequestInput,
) (port.RequestView, *entity.Fault) {
	state, found, err := i.numbers.State(ctx, in.MSISDN)
	if err != nil {
		return port.RequestView{}, entity.InternalError("lecture du numéro")
	}
	if !found {
		return port.RequestView{}, entity.IncorrectSourceOperator()
	}

	// [HYP] The guide does not settle role assignment on a restitution; the
	// project chose that the caller must be the number's origin operator and
	// becomes the recipient (it gets the number back). See §9.4 of the spec.
	caller := port.CallerFromContext(ctx)
	if state.OriginOperatorID != caller.OperatorID {
		return port.RequestView{}, entity.RequestAccessDenied(
			"Seul l'opérateur d'origine du numéro peut demander sa restitution.")
	}

	if f := entity.CheckRestitutionEligibility(state, entity.DelayBeforeRestitution); f != nil {
		return port.RequestView{}, f
	}

	id := i.ids.NewID()
	now := i.clock.Now()

	err = i.uow.Do(ctx, func(repos port.Repositories) error {
		// operateur_source_id = current holder (it gives back the number);
		// operateur_destinataire_id = createur_operateur_id = origin
		// operator (the caller, it gets the number back). Process and
		// RoutingInfo stay nil: a restitution has neither a routing prefix
		// nor a PREPAID/POSTPAID dimension before its COMPLETION.
		if err := repos.Requests.Create(ctx, port.CreateRequestInput{
			ID: id, MSISDN: in.MSISDN,
			SubscriberType:   string(entity.SubscriberIndividual),
			RequestType:      string(entity.RequestTypeRestitution),
			SourceOperatorID: state.CurrentOperatorID, RecipientOperatorID: state.OriginOperatorID,
			CreatorOperatorID: state.OriginOperatorID,
			RequestDate:       now,
		}); err != nil {
			return entity.InternalError("création de la demande")
		}
		if err := repos.Requests.AddNumber(ctx, port.RequestNumberInput{
			RequestID: id, MSISDN: in.MSISDN,
		}); err != nil {
			return entity.InternalError("enregistrement du numéro")
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
