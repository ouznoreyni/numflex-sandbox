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
}
