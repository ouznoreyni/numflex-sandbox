// Package sandbox holds the one use case behind
// DELETE /api/sandbox/v1/demandes — hors gateway, hors contrat ARTP. The
// route only exists when config.SandboxAdmin is true; internal/api decides
// that, not this package.
package sandbox

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// PurgeTestDataResult carries the four counts the deleted
// internal/api/sandbox.go's deletePurgeDemandes rendered:
// {demandesSupprimees, numerosRestaures, otpSupprimes, reverseSupprimees}.
type PurgeTestDataResult struct {
	RequestsDeleted int64
	NumbersRestored int64
	OTPDeleted      int64
	ReverseDeleted  int64
}

// PurgeTestDataBoundary is the interface a controller drives.
type PurgeTestDataBoundary interface {
	Execute(ctx context.Context) (PurgeTestDataResult, *entity.Fault)
}

// PurgeTestDataInteractor implements PurgeTestDataBoundary.
type PurgeTestDataInteractor struct {
	uow port.UnitOfWork
}

// NewPurgeTestData wires an interactor against its dependency.
func NewPurgeTestData(uow port.UnitOfWork) *PurgeTestDataInteractor {
	return &PurgeTestDataInteractor{uow: uow}
}

// Execute reproduces the deleted internal/api/sandbox.go's
// deletePurgeDemandes: it wipes the caller's own test data and restores the
// national registry for every number involved. The scope is
// createur_operateur_id, never the /mes-demandes filter — a request belongs
// to two operators at once, and only its creator made it; a Port-IN created
// by a partner may not be purged with this caller's token.
//
// This is the strongest case for port.UnitOfWork in the whole project: five
// tables (demande, demande_numero, demande_client, etape_historique and
// confirmation via cascade, plus reverse_request, otp and numero directly),
// six statements, one Do. The restoration is what makes the purge useful:
// without it a number already ported would stay blocked by
// DELAI_PORTAGE_NON_RESPECTE for three months, and purging the request
// alone would not let the scenario be replayed — so every read and write
// below runs inside the same transaction, or none of them survive.
func (i *PurgeTestDataInteractor) Execute(ctx context.Context) (PurgeTestDataResult, *entity.Fault) {
	caller := port.CallerFromContext(ctx)

	var result PurgeTestDataResult
	err := i.uow.Do(ctx, func(repos port.Repositories) error {
		ids, err := repos.Sandbox.RequestIDsToPurge(ctx, caller.OperatorID)
		if err != nil {
			return entity.InternalError("lecture des demandes à purger")
		}

		numeros, err := repos.Sandbox.NumbersToRestore(ctx, ids)
		if err != nil {
			return entity.InternalError("lecture des numéros à restaurer")
		}

		// Ahead of the demande DELETE: reverse_request's foreign key carries
		// no ON DELETE CASCADE and would block it.
		reverseCount, err := repos.Sandbox.DeleteReverseRequests(ctx, caller.OperatorID, ids)
		if err != nil {
			return entity.InternalError("purge des demandes de reverse")
		}

		otpCount, err := repos.Sandbox.DeleteOTP(ctx, numeros)
		if err != nil {
			return entity.InternalError("purge des OTP")
		}

		requestCount, err := repos.Sandbox.DeleteRequests(ctx, ids)
		if err != nil {
			return entity.InternalError("purge des demandes")
		}

		numberCount, err := repos.Sandbox.RestoreNumbers(ctx, numeros)
		if err != nil {
			return entity.InternalError("restauration du registre")
		}

		result = PurgeTestDataResult{
			RequestsDeleted: requestCount, NumbersRestored: numberCount,
			OTPDeleted: otpCount, ReverseDeleted: reverseCount,
		}
		return nil
	})
	if err != nil {
		return PurgeTestDataResult{}, entity.FaultFrom(err)
	}
	return result, nil
}
