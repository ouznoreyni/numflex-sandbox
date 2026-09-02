package identifier

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

var pattern = regexp.MustCompile(`^[0-9a-f]{24}$`)

func TestNewFormat(t *testing.T) {
	for i := 0; i < 100; i++ {
		id := New()
		require.Truef(t, pattern.MatchString(id), "identifiant non conforme : %q", id)
	}
}

func TestNewUniqueness(t *testing.T) {
	seen := make(map[string]bool, 10000)
	for i := 0; i < 10000; i++ {
		id := New()
		require.Falsef(t, seen[id], "collision sur %q", id)
		seen[id] = true
	}
}

// TestGeneratorNewIDDelegatesToNew pins that Generator, the port.IDGenerator
// implementation, produces the same shape as the free function.
func TestGeneratorNewIDDelegatesToNew(t *testing.T) {
	id := NewGenerator().NewID()
	require.Truef(t, pattern.MatchString(id), "identifiant non conforme : %q", id)
}
