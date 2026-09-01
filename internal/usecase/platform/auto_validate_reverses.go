package platform

import (
	"context"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ValidateReverse est un acte de l'ARTP, hors périmètre de l'API gateway
// (§6). Il crée une Demande de type REVERSE directement à l'étape
// CONFIRMATION : ni ACCEPTATION, ni DESACTIVATION/ACTIVATION. Moved in
// substance from the deleted internal/engine/reverse.go's own
// ValiderReverse: internal/framework/engine.ValiderReverse (cmd/artp's own
// caller) and AutoValidateReversesInteractor's own auto-validation loop
// both call this same function, so the ARTP's manual act and the sandbox's
// automatic stand-in for it never drift apart.
func ValidateReverse(ctx context.Context, uow port.UnitOfWork, ids port.IDGenerator, clock port.Clock, reverseID string) error {
	return uow.Do(ctx, func(repos port.Repositories) error {
		msisdn, operatorID, statut, err := repos.Reverse.LockPending(ctx, reverseID)
		if err != nil {
			return err
		}
		if statut != "EN_ATTENTE" {
			return nil
		}

		detenteurActuel, err := repos.Reverse.CurrentOperatorFor(ctx, msisdn)
		if err != nil {
			return err
		}

		id := ids.NewID()
		now := clock.Now()
		if err := repos.Requests.CreateAtConfirmation(ctx, port.CreateRequestInput{
			ID:                  id,
			MSISDN:              msisdn,
			SubscriberType:      string(entity.SubscriberIndividual),
			RequestType:         string(entity.RequestTypeReverse),
			SourceOperatorID:    detenteurActuel,
			RecipientOperatorID: operatorID,
			CreatorOperatorID:   operatorID,
			RequestDate:         now,
		}); err != nil {
			return err
		}
		if err := repos.Requests.AddNumber(ctx, port.RequestNumberInput{
			RequestID: id, MSISDN: msisdn,
		}); err != nil {
			return err
		}
		return repos.Reverse.MarkValidated(ctx, reverseID, id, now)
	})
}

// RejectReverse est également un acte de l'ARTP : rejeter la demande sans
// jamais créer de Demande. Moved verbatim in substance from the deleted
// internal/engine/reverse.go's own RejeterReverse.
func RejectReverse(ctx context.Context, uow port.UnitOfWork, reverseID string) error {
	return uow.Do(ctx, func(repos port.Repositories) error {
		return repos.Reverse.Reject(ctx, reverseID)
	})
}

// AutoValidateReversesInteractor covers the reverse lifecycle's two
// tick-driven behaviours, in the fixed order internal/engine.Tick already
// ran them in: automatic validation of an overdue reverse request, then
// automatic completion of a confirmed one. Moved in substance from the
// deleted internal/engine/reverse.go's own validerReversesAutomatiquement
// and completerReversesConfirmes.
type AutoValidateReversesInteractor struct {
	reverse             port.ReverseGateway
	requests            port.RequestGateway
	uow                 port.UnitOfWork
	ids                 port.IDGenerator
	clock               port.Clock
	autoValidationDelay time.Duration
}

// NewAutoValidateReverses wires an interactor against its dependencies.
// autoValidationDelay is REVERSE_AUTO_VALIDATION_SECONDS
// (internal/framework/config.Config), a plain time.Duration crossing the
// same boundary ExpireOverdueStepsInteractor's own timeout already does.
func NewAutoValidateReverses(
	reverse port.ReverseGateway, requests port.RequestGateway, uow port.UnitOfWork,
	ids port.IDGenerator, clock port.Clock, autoValidationDelay time.Duration,
) *AutoValidateReversesInteractor {
	return &AutoValidateReversesInteractor{
		reverse: reverse, requests: requests, uow: uow, ids: ids, clock: clock,
		autoValidationDelay: autoValidationDelay,
	}
}

// Execute runs the reverse lifecycle's two steps in order — see this type's
// own doc comment for why the order is fixed.
func (i *AutoValidateReversesInteractor) Execute(ctx context.Context) error {
	if err := i.autoValidate(ctx); err != nil {
		return err
	}
	return i.autoComplete(ctx)
}

// autoValidate rejoue ValidateReverse pour toute demande EN_ATTENTE depuis
// plus de REVERSE_AUTO_VALIDATION_SECONDS. Désactivé par défaut (0 =
// jamais) : dans le monde réel, la validation est un acte humain de
// l'ARTP, hors API ; ce délai n'existe que pour permettre au sandbox de
// simuler l'aval du régulateur sans intervention du CLI.
func (i *AutoValidateReversesInteractor) autoValidate(ctx context.Context) error {
	if i.autoValidationDelay <= 0 {
		return nil
	}
	ids, err := i.reverse.OverdueForAutoValidation(ctx, i.autoValidationDelay.Seconds())
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := ValidateReverse(ctx, i.uow, i.ids, i.clock, id); err != nil {
			return err
		}
	}
	return nil
}

// autoComplete : la COMPLETION d'un REVERSE est réservée à l'ARTP. Aucun
// endpoint ne l'expose ; c'est le moteur qui la prononce une fois que tous
// les opérateurs ont confirmé.
//
// Cette fonction doit aussi rattraper une demande REVERSE déjà à COMPLETION :
// postAConfirmer est agnostique du type de demande, et quand la dernière
// confirmation tombe, il planifie une transition générique via
// port.Engine.ScheduleTransition. Au tick suivant, la convergence générique
// s'exécute avant cette fonction et fait passer la demande de CONFIRMATION à
// COMPLETION par le chemin commun, en remettant transition_prevue_a à NULL —
// puisque la COMPLETION d'un REVERSE n'appartient à aucun opérateur, plus
// aucun endpoint ne peut la faire avancer ensuite. Sans ce rattrapage, la
// demande reste figée à COMPLETION/EN_COURS pour toujours. La branche
// CONFIRMATION reste nécessaire : elle sert quand autoValidate amène une
// demande jusqu'ici sans jamais passer par postAConfirmer (toutes les
// confirmations peuvent avoir été enregistrées avant que la dernière ne
// déclenche la planification, ou la demande peut n'avoir encore aucune
// transition planifiée).
func (i *AutoValidateReversesInteractor) autoComplete(ctx context.Context) error {
	candidats, err := i.requests.PendingReverseCompletions(ctx)
	if err != nil {
		return err
	}
	for _, c := range candidats {
		// Depuis CONFIRMATION : CONFIRMATION → COMPLETION, puis COMPLETION →
		// TERMINE. Depuis COMPLETION (déjà atteinte par la convergence
		// générique) : une seule transition suffit, COMPLETION → TERMINE.
		if c.CurrentStep == entity.StepConfirmation {
			if err := ApplyTransition(ctx, i.uow, i.clock, c.RequestID, "ACTION"); err != nil {
				return err
			}
		}
		if err := ApplyTransition(ctx, i.uow, i.clock, c.RequestID, "ACTION"); err != nil {
			return err
		}
	}
	return nil
}
