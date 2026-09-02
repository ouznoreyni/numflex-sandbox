package reference

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// ListOperatorsBoundary is the interface a controller drives.
type ListOperatorsBoundary interface {
	Execute(context.Context) ([]entity.Operator, *entity.Fault)
}

// ListOperatorsInteractor implements ListOperatorsBoundary.
type ListOperatorsInteractor struct {
	gateway port.ReferenceGateway
}

// NewListOperators wires an interactor against the given gateway.
func NewListOperators(g port.ReferenceGateway) *ListOperatorsInteractor {
	return &ListOperatorsInteractor{gateway: g}
}

func (i *ListOperatorsInteractor) Execute(ctx context.Context) ([]entity.Operator, *entity.Fault) {
	out, err := i.gateway.Operators(ctx)
	if err != nil {
		return nil, entity.InternalError("reading the operators")
	}
	return out, nil
}
