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
	courante := dm.CurrentStep

	// Une étape soldée par une action porte TERMINE, y compris la COMPLETION :
	// c'est ce que rendent les captures « in » et « 2_yas_confirmer-a
	// COMPLETION », et ce qu'ANO-013 décrivait déjà (TERMINE en nominal, EXPIRE
	// par expiration).
	closedStatus := entity.StepCompleted
	if origin == "EXPIRATION" {
		closedStatus = entity.StepExpired
	}
	if err := requests.CloseCurrentStep(ctx, id, closedStatus, origin, now); err != nil {
		return err
	}

	suivante, existe := entity.NextStep(courante)
	if !existe && courante != entity.StepCompletion {
		// etape_actuelle n'a pas de contrainte CHECK en base : une valeur
		// corrompue ne doit pas être traitée comme si COMPLETION était soldée
		// (ce qui clôturerait la demande et transférerait le numéro).
		return fmt.Errorf("étape inconnue %q sur la demande %s", courante, id)
	}
	if !existe {
		// COMPLETION soldée : la demande se termine.
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

	// Effets de bord attachés à la sortie de l'étape.
	if courante == entity.StepActivation && dm.RequestType == entity.RequestTypePorting {
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

	return requests.AdvanceStep(ctx, id, suivante, now)
}
