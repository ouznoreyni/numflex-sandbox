package port_test

import (
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/inmemory"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// TestInMemoryDoublesSatisfyPorts fails to compile if a double drifts from
// its interface. It is the cheapest guard against a half-updated port.
func TestInMemoryDoublesSatisfyPorts(t *testing.T) {
	var _ port.OTPGateway = (*inmemory.OTPGateway)(nil)
	var _ port.Clock = inmemory.FixedClock{}
	var _ port.NumberGateway = (*inmemory.NumberGateway)(nil)
	var _ port.RequestGateway = (*inmemory.RequestGateway)(nil)
	var _ port.UnitOfWork = (*inmemory.UnitOfWork)(nil)
	var _ port.IDGenerator = (*inmemory.IDGenerator)(nil)
	var _ port.ReferenceGateway = (*inmemory.ReferenceGateway)(nil)
	var _ port.Engine = (*inmemory.Engine)(nil)
	var _ port.ConfirmationGateway = (*inmemory.ConfirmationGateway)(nil)
}
