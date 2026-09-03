package sandbox

import (
	"context"
	"strings"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// The two natures a range can have. They are read from the rows, never from
// internal/framework/seed's own declaration: what the sandbox meant to
// install and what the database holds are two different things as soon as
// the volume is persistent.
const (
	NatureNeverPorted   = "JAMAIS_PORTE"
	NatureAlreadyPorted = "DEJA_PORTE"
)

// RangeView is one three-digit range as it leaves the interactor.
type RangeView struct {
	Prefix string
	First  string
	Last   string
	Total  int64
	Nature string
}

// CountNumberRangesResult carries the operator the caller named, resolved
// against the operateur table, and its ranges.
type CountNumberRangesResult struct {
	OperatorID string
	Operator   string
	Ranges     []RangeView
	Total      int64
}

// CountNumberRangesBoundary is the interface a controller drives.
type CountNumberRangesBoundary interface {
	Execute(ctx context.Context, operator string) (CountNumberRangesResult, *entity.Fault)
}

// CountNumberRangesInteractor implements CountNumberRangesBoundary.
type CountNumberRangesInteractor struct {
	reference port.ReferenceGateway
	ranges    port.NumberRangeGateway
}

// NewCountNumberRanges wires an interactor against its two gateways.
func NewCountNumberRanges(reference port.ReferenceGateway, ranges port.NumberRangeGateway) *CountNumberRangesInteractor {
	return &CountNumberRangesInteractor{reference: reference, ranges: ranges}
}

// Execute resolves operator — the enum of the route's only parameter —
// against the operateur table rather than against a constant list, so the
// accepted values are exactly the operators the registry knows, then counts
// what that operator holds, range by range.
//
// An unknown or missing name is a FaultValidation carrying the field, hence
// a 400 naming the accepted values, in both fidelity modes: this route is
// outside the ARTP contract, so there is no ANO-003 to reproduce and
// nothing gained by answering 500 to a typo.
func (i *CountNumberRangesInteractor) Execute(ctx context.Context, operator string) (CountNumberRangesResult, *entity.Fault) {
	operators, err := i.reference.Operators(ctx)
	if err != nil {
		return CountNumberRangesResult{}, entity.InternalError("reading the operators")
	}

	wanted := strings.ToUpper(strings.TrimSpace(operator))
	var matched entity.Operator
	names := make([]string, 0, len(operators))
	for _, o := range operators {
		names = append(names, o.Name)
		if strings.ToUpper(o.Name) == wanted {
			matched = o
		}
	}
	if matched.ID == "" {
		return CountNumberRangesResult{}, entity.Validation(entity.FieldFault{
			ObjectName: "tranchesQuery",
			Field:      "operateur",
			Message:    "doit valoir l'un de : " + strings.Join(names, ", "),
		})
	}

	rows, err := i.ranges.RangesByOperator(ctx, matched.ID)
	if err != nil {
		return CountNumberRangesResult{}, entity.InternalError("counting the ranges")
	}

	out := CountNumberRangesResult{
		OperatorID: matched.ID,
		Operator:   matched.Name,
		Ranges:     make([]RangeView, 0, len(rows)),
	}
	for _, r := range rows {
		nature := NatureNeverPorted
		if r.Ported > 0 {
			nature = NatureAlreadyPorted
		}
		out.Ranges = append(out.Ranges, RangeView{
			Prefix: r.Prefix, First: r.First, Last: r.Last,
			Total: r.Total, Nature: nature,
		})
		out.Total += r.Total
	}
	return out, nil
}
