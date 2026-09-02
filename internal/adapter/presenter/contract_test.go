package presenter

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/inmemory"
	"github.com/stretchr/testify/require"
)

// R11: these tests duplicate (not move) internal/httpx/renderer_test.go's
// contract-fidelity assertions, converted to assert on the returned
// ViewModel instead of a httptest.ResponseRecorder. See real_test.go's
// header comment and bodyMap helper — both files share this package.

func TestContractStateErrorRendersEnveloped(t *testing.T) {
	c := NewContract(inmemory.FixedClock{})

	vm := c.Failure(entity.RequestNotFound(), "/x")

	require.Equal(t, http.StatusNotFound, vm.Status)
	body := bodyMap(t, vm)
	require.Equal(t, false, body["success"])
	require.Equal(t, "DEMANDE_NON_TROUVEE", body["code"])
	require.Equal(t, "Demande introuvable", body["message"])
	require.NotContains(t, body, "type")
}

func TestContractKindStatusMapping(t *testing.T) {
	cases := []struct {
		err    *entity.Fault
		status int
		code   string
	}{
		{entity.Validation(entity.FieldFault{Field: "numero", Message: "obligatoire"}), 400, "VALIDATION_ECHOUEE"},
		{entity.RequestNotFound(), 404, "DEMANDE_NON_TROUVEE"},
		{entity.RequestAccessDenied("denied"), 403, "DEMANDE_ACCES_REFUSE"},
		{entity.InvalidStep("wrong step"), 409, "ETAPE_INVALIDE"},
		{entity.InternalError("boom"), 500, "ERREUR_INTERNE"},
	}
	for _, x := range cases {
		c := NewContract(inmemory.FixedClock{})
		vm := c.Failure(x.err, "/x")
		require.Equalf(t, x.status, vm.Status, "code %s", x.code)
		body := bodyMap(t, vm)
		require.Equal(t, x.code, body["code"])
	}
}

func TestSuccessContract(t *testing.T) {
	c := NewContract(inmemory.FixedClock{})
	vm := c.Success(http.StatusCreated, "Demande créée avec succès", map[string]string{"id": "abc"})

	require.Equal(t, http.StatusCreated, vm.Status)
	body := bodyMap(t, vm)
	require.Equal(t, true, body["success"])
	require.Equal(t, "SUCCESS", body["code"])
	require.Equal(t, "Demande créée avec succès", body["message"])
	require.Equal(t, map[string]any{"id": "abc"}, body["data"])
}

func TestOKWithoutDataRendersDataNullInContract(t *testing.T) {
	c := NewContract(inmemory.FixedClock{})
	vm := c.SuccessWithoutData(http.StatusOK, "OTP envoyé avec succès")

	body := bodyMap(t, vm)
	require.Contains(t, body, "data")
	require.Nil(t, body["data"])
}

func TestFailWrappedErrorKeepsKindAndMessage(t *testing.T) {
	// Review correction #3, moved into the pure layer: errors.As must
	// unwrap an error wrapped by fmt.Errorf("...: %w", ...). Fix round 1
	// (finding 2): exercises entity.FaultFrom on the wrapped error rather
	// than on the fault it would produce.
	f := entity.FaultFrom(fmt.Errorf("context: %w", entity.RequestNotFound()))
	require.Equal(t, entity.RequestNotFound(), f)

	c := NewContract(inmemory.FixedClock{})
	vm := c.Failure(f, "/x")

	require.Equal(t, http.StatusNotFound, vm.Status)
	body := bodyMap(t, vm)
	require.Equal(t, "DEMANDE_NON_TROUVEE", body["code"])
	require.Equal(t, "Demande introuvable", body["message"])
}
