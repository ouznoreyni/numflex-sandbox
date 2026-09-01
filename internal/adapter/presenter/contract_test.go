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

func TestContractErreurEtatSortEnveloppee(t *testing.T) {
	c := NewContract(inmemory.FixedClock{})

	vm := c.Failure(entity.RequestNotFound(), "/x")

	require.Equal(t, http.StatusNotFound, vm.Status)
	corps := bodyMap(t, vm)
	require.Equal(t, false, corps["success"])
	require.Equal(t, "DEMANDE_NON_TROUVEE", corps["code"])
	require.Equal(t, "Demande introuvable", corps["message"])
	require.NotContains(t, corps, "type")
}

func TestContractCorrespondanceKindStatut(t *testing.T) {
	cas := []struct {
		err    *entity.Fault
		statut int
		code   string
	}{
		{entity.Validation(entity.FieldFault{Field: "numero", Message: "obligatoire"}), 400, "VALIDATION_ECHOUEE"},
		{entity.RequestNotFound(), 404, "DEMANDE_NON_TROUVEE"},
		{entity.RequestAccessDenied("refusé"), 403, "DEMANDE_ACCES_REFUSE"},
		{entity.InvalidStep("mauvaise étape"), 409, "ETAPE_INVALIDE"},
		{entity.InternalError("boum"), 500, "ERREUR_INTERNE"},
	}
	for _, x := range cas {
		c := NewContract(inmemory.FixedClock{})
		vm := c.Failure(x.err, "/x")
		require.Equalf(t, x.statut, vm.Status, "code %s", x.code)
		corps := bodyMap(t, vm)
		require.Equal(t, x.code, corps["code"])
	}
}

func TestSuccesContrat(t *testing.T) {
	c := NewContract(inmemory.FixedClock{})
	vm := c.Success(http.StatusCreated, "Demande créée avec succès", map[string]string{"id": "abc"})

	require.Equal(t, http.StatusCreated, vm.Status)
	corps := bodyMap(t, vm)
	require.Equal(t, true, corps["success"])
	require.Equal(t, "SUCCESS", corps["code"])
	require.Equal(t, "Demande créée avec succès", corps["message"])
	require.Equal(t, map[string]any{"id": "abc"}, corps["data"])
}

func TestOKSansDataRendDataNullEnContrat(t *testing.T) {
	c := NewContract(inmemory.FixedClock{})
	vm := c.SuccessWithoutData(http.StatusOK, "OTP envoyé avec succès")

	corps := bodyMap(t, vm)
	require.Contains(t, corps, "data")
	require.Nil(t, corps["data"])
}

func TestFailErreurEmballeeConserveKindEtMessage(t *testing.T) {
	// Correction revue #3, déplacée dans la couche pure : errors.As doit
	// déballer une erreur enveloppée par fmt.Errorf("...: %w", ...). Fix
	// round 1 (finding 2) : exerce entity.FaultFrom sur l'erreur emballée
	// plutôt que sur le fault qu'elle produirait.
	f := entity.FaultFrom(fmt.Errorf("contexte : %w", entity.RequestNotFound()))
	require.Equal(t, entity.RequestNotFound(), f)

	c := NewContract(inmemory.FixedClock{})
	vm := c.Failure(f, "/x")

	require.Equal(t, http.StatusNotFound, vm.Status)
	corps := bodyMap(t, vm)
	require.Equal(t, "DEMANDE_NON_TROUVEE", corps["code"])
	require.Equal(t, "Demande introuvable", corps["message"])
}
