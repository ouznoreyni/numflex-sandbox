// Package reference holds the five read-only reference-data use cases
// behind /operateurs, /motifs-rejet, /types-demande, /processus and
// /types-incident. There is no business rule to apply to a reference list —
// no validation to invent, no filtering, no defensive re-checking of what
// the gateway returned — so each interactor here is a direct pass-through:
// it calls one port.ReferenceGateway method and turns a gateway error into
// an *entity.Fault, exactly as internal/api/referentiels.go's five handlers
// did before this task moved their SQL into
// internal/adapter/gateway/postgres/reference_gateway.go. Five files of
// ceremony that did nothing would be worse than the handlers they replace,
// so none of them is padded with logic that isn't there.
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
		return nil, entity.InternalError("lecture des opérateurs")
	}
	return out, nil
}
