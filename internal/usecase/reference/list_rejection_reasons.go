package reference

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ListRejectionReasonsBoundary is the interface a controller drives.
type ListRejectionReasonsBoundary interface {
	Execute(context.Context) ([]entity.RejectionReason, *entity.Fault)
}

// ListRejectionReasonsInteractor implements ListRejectionReasonsBoundary.
type ListRejectionReasonsInteractor struct {
	gateway port.ReferenceGateway
}

// NewListRejectionReasons wires an interactor against the given gateway.
func NewListRejectionReasons(g port.ReferenceGateway) *ListRejectionReasonsInteractor {
	return &ListRejectionReasonsInteractor{gateway: g}
}

func (i *ListRejectionReasonsInteractor) Execute(ctx context.Context) ([]entity.RejectionReason, *entity.Fault) {
	out, err := i.gateway.RejectionReasons(ctx)
	if err != nil {
		return nil, entity.InternalError("reading the rejection reasons")
	}
	return out, nil
}
