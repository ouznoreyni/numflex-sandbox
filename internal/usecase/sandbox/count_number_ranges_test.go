package sandbox_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/inmemory"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/sandbox"
)

// rangeGateway is a map-backed port.NumberRangeGateway. It stays here
// rather than in internal/testsupport/inmemory because this interactor is
// its only consumer: one route, one aggregate, no second use case to share
// it with.
type rangeGateway struct {
	byOperator map[string][]port.NumberRange
	err        error
}

func (g rangeGateway) RangesByOperator(_ context.Context, operatorID string) ([]port.NumberRange, error) {
	if g.err != nil {
		return nil, g.err
	}
	return g.byOperator[operatorID], nil
}

func newInteractor(gw rangeGateway) *sandbox.CountNumberRangesInteractor {
	ref := inmemory.NewReferenceGateway()
	ref.SeedOperator("op-orange", "ORANGE")
	ref.SeedOperator("op-yas", "YAS")
	ref.SeedOperator("op-expresso", "EXPRESSO")
	return sandbox.NewCountNumberRanges(ref, gw)
}

var orangeRanges = rangeGateway{byOperator: map[string][]port.NumberRange{
	"op-orange": {
		{Prefix: "771", First: "771000000", Last: "771999999", Total: 1_000_000},
		{Prefix: "779", First: "779000000", Last: "779003999", Total: 4000, Ported: 4000},
	},
}}

// The nature of a range is read from the rows — a range whose numbers carry
// a porting date is rejection material — never from what the seed meant to
// install: a persistent database seeded at another volume must still
// describe itself truthfully.
func TestRangeNatureComesFromTheRows(t *testing.T) {
	out, fault := newInteractor(orangeRanges).Execute(context.Background(), "ORANGE")

	require.Nil(t, fault)
	require.Equal(t, "op-orange", out.OperatorID)
	require.Len(t, out.Ranges, 2)
	require.Equal(t, sandbox.NatureNeverPorted, out.Ranges[0].Nature)
	require.Equal(t, sandbox.NatureAlreadyPorted, out.Ranges[1].Nature)
	require.Equal(t, int64(1_004_000), out.Total, "the total sums the ranges")
}

// The enum is resolved against the operateur table, so its accepted values
// are exactly the operators the registry knows — and a name typed in a
// browser bar is not rejected on its case alone.
func TestOperatorNameIsCaseInsensitive(t *testing.T) {
	out, fault := newInteractor(orangeRanges).Execute(context.Background(), "  orange ")

	require.Nil(t, fault)
	require.Equal(t, "ORANGE", out.Operator)
	require.Len(t, out.Ranges, 2)
}

// Outside the ARTP contract there is no ANO-003 to reproduce: a typo
// deserves the 400 a bean validation would give, naming what is accepted.
func TestUnknownOperatorIsAValidationFault(t *testing.T) {
	for _, operator := range []string{"", "SONATEL"} {
		_, fault := newInteractor(orangeRanges).Execute(context.Background(), operator)

		require.NotNil(t, fault, "operator %q", operator)
		require.Equal(t, entity.FaultValidation, fault.Kind)
		require.Len(t, fault.Fields, 1)
		require.Equal(t, "operateur", fault.Fields[0].Field)
		require.Contains(t, fault.Fields[0].Message, "ORANGE")
		require.Contains(t, fault.Fields[0].Message, "EXPRESSO")
	}
}

// An operator that holds nothing is not an error: the registry simply has
// no range for it, and the envelope carries an empty list rather than null.
func TestOperatorWithoutNumbersAnswersEmpty(t *testing.T) {
	out, fault := newInteractor(orangeRanges).Execute(context.Background(), "EXPRESSO")

	require.Nil(t, fault)
	require.NotNil(t, out.Ranges)
	require.Empty(t, out.Ranges)
	require.Zero(t, out.Total)
}

func TestGatewayFailureIsInternal(t *testing.T) {
	gw := rangeGateway{err: errors.New("boom counting the ranges")}

	_, fault := newInteractor(gw).Execute(context.Background(), "ORANGE")

	require.NotNil(t, fault)
	require.Equal(t, entity.FaultInternal, fault.Kind)
}
