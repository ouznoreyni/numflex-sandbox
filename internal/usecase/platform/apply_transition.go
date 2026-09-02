package platform

import (
	"context"
	"fmt"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ApplyTransition closes id's current step and moves it to the next one —
// moved in substance from the deleted internal/engine/transitions.go's
// AppliquerTransition, now expressed through port.RequestGateway rather
// than a raw *pgx.Tx: the three helpers that used to take a pgx.Tx directly
// (transfererAuRegistre, recalculerRoutage, effetsFinDeDemande) are now
// port.RequestGateway methods (TransferToRegistry, ApplyRouting,
// ApplyEndOfRequestRestitution), so no pgx type crosses into this package.
// origin is "ACTION" (nominal processing, immediate or converged) or
// "EXPIRATION" (ANO-006). Runs inside one port.UnitOfWork.Do: the FOR
// UPDATE read, the historique write and every side effect commit or roll
// back together — the same atomicity guarantee AppliquerTransition's own
// *pgx.Tx gave.
func ApplyTransition(ctx context.Context, uow port.UnitOfWork, clock port.Clock, id, origin string) error {
	return uow.Do(ctx, func(repos port.Repositories) error {
		return applyTransitionLocked(ctx, repos.Requests, clock, id, origin)
	})
}

func applyTransitionLocked(ctx context.Context, requests port.RequestGateway, clock port.Clock, id, origin string) error {
	dm, found, err := requests.LockForTransition(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if dm.Status != entity.RequestInProgress {
		return nil
	}

	now := clock.Now()
	current := dm.CurrentStep

	// A step closed by an action carries TERMINE, COMPLETION included: that
	// is what the "in" and "2_yas_confirmer-a COMPLETION" captures render,
	// and what ANO-013 already described (TERMINE on the nominal path,
	// EXPIRE on expiry).
	closedStatus := entity.StepCompleted
	if origin == "EXPIRATION" {
		closedStatus = entity.StepExpired
	}
	if err := requests.CloseCurrentStep(ctx, id, closedStatus, origin, now); err != nil {
		return err
	}

	next, exists := entity.NextStep(current)
	if !exists && current != entity.StepCompletion {
		// etape_actuelle carries no CHECK constraint in the database: a
		// corrupted value must not be treated as if COMPLETION were closed
		// (which would close the request and transfer the number).
		return fmt.Errorf("étape inconnue %q sur la demande %s", current, id)
	}
	if !exists {
		// COMPLETION closed: the request ends.
		if dm.RequestType != entity.RequestTypePorting {
			prefix, err := requests.RoutingPrefix(ctx, dm.RecipientOperatorID)
			if err != nil {
				return err
			}
			if err := requests.ApplyEndOfRequestRestitution(ctx, id, dm.MSISDN, dm.RecipientOperatorID, prefix); err != nil {
				return err
			}
		}
		return requests.CompleteRequest(ctx, id, closedStatus, now)
	}

	// Side effects attached to leaving the step.
	if current == entity.StepActivation && dm.RequestType == entity.RequestTypePorting {
		if err := requests.TransferToRegistry(ctx, id, dm.RecipientOperatorID); err != nil {
			return err
		}
		destPrefix, err := requests.RoutingPrefix(ctx, dm.RecipientOperatorID)
		if err != nil {
			return err
		}
		srcPrefix, err := requests.RoutingPrefix(ctx, dm.SourceOperatorID)
		if err != nil {
			return err
		}
		if err := requests.ApplyRouting(ctx, id, srcPrefix, destPrefix); err != nil {
			return err
		}
	}

	return requests.AdvanceStep(ctx, id, next, now)
}
