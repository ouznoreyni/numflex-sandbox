package presenter

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/inmemory"
	"github.com/stretchr/testify/require"
)

// bodyMap marshals a ViewModel's Body the same way a JSON transport layer
// would, and unmarshals the result into a generic map — so tests can assert
// on field presence or absence exactly as internal/httpx/renderer_test.go
// did against the HTTP response body. Field presence (encoding/json
// omitempty, or a struct that simply has no such field, like
// EnvelopeSansData) only shows up once the value is actually marshalled.
func bodyMap(t *testing.T, vm ViewModel) map[string]any {
	t.Helper()
	b, err := json.Marshal(vm.Body)
	require.NoError(t, err)
	var corps map[string]any
	require.NoError(t, json.Unmarshal(b, &corps))
	return corps
}

// R11: these tests duplicate (not move) internal/httpx/renderer_test.go's
// real-fidelity assertions, converted to assert on the returned ViewModel
// instead of a httptest.ResponseRecorder. internal/httpx/renderer_test.go
// stays untouched — httpx.Renderer remains the live production path until
// Task 18.

func TestRealErreurEtatSortEn500SansCode(t *testing.T) {
	r := NewReal(inmemory.FixedClock{})

	vm := r.Failure(entity.RequestNotFound())

	require.Equal(t, http.StatusInternalServerError, vm.Status)
	corps := bodyMap(t, vm)
	require.NotContains(t, corps, "code", "ANO-001 : aucune erreur ne porte de champ code en mode réel")
	require.NotContains(t, corps, "success")
	require.Equal(t, "https://www.jhipster.tech/problem/problem-with-message", corps["type"])
	require.Equal(t, "Internal Server Error", corps["title"])
	require.Equal(t, float64(500), corps["status"])
	require.Equal(t, "error.http.500", corps["message"])
	require.Equal(t, "RuntimeException: Demande introuvable", corps["detail"])
	// NOTE: httpx.Renderer.failReel fills "path" from the live *gin.Context.
	// Presenter.Failure takes only a *entity.Fault, with no request in scope
	// (see the Problem doc comment) — "path" is present but empty for now.
	require.Equal(t, "", corps["path"])
}

func TestRealErreurValidationSortEn400AvecFieldErrors(t *testing.T) {
	r := NewReal(inmemory.FixedClock{})

	vm := r.Failure(entity.Validation(entity.FieldFault{
		ObjectName: "demandeParticulierDTO",
		Field:      "client.lieuNaissance",
		Message:    "ne doit pas être vide",
	}))

	require.Equal(t, http.StatusBadRequest, vm.Status)
	corps := bodyMap(t, vm)
	require.NotContains(t, corps, "code")
	require.Equal(t, "https://www.jhipster.tech/problem/constraint-violation", corps["type"])
	require.Equal(t, "Method argument not valid", corps["title"])
	require.Equal(t, "error.validation", corps["message"])

	champs := corps["fieldErrors"].([]any)
	require.Len(t, champs, 1)
	premier := champs[0].(map[string]any)
	require.Equal(t, "demandeParticulierDTO", premier["objectName"])
	require.Equal(t, "client.lieuNaissance", premier["field"])
	require.Equal(t, "ne doit pas être vide", premier["message"])
}

func TestRealDetailPersonnalise(t *testing.T) {
	// ANO-002 : le refus de re-portage à moins de 3 mois se présente comme
	// une panne.
	r := NewReal(inmemory.FixedClock{})

	vm := r.Failure(entity.PortingDelayNotRespected())

	require.Equal(t, http.StatusInternalServerError, vm.Status)
	corps := bodyMap(t, vm)
	require.Equal(t, "Unexpected runtime exception", corps["detail"])
}

func TestSuccesReel(t *testing.T) {
	r := NewReal(inmemory.FixedClock{})
	vm := r.Success(http.StatusCreated, "Demande créée avec succès", map[string]string{"id": "abc"})

	require.Equal(t, http.StatusCreated, vm.Status)
	corps := bodyMap(t, vm)
	require.Equal(t, true, corps["success"])
	require.Equal(t, "SUCCESS", corps["code"])
	require.Equal(t, "Demande créée avec succès", corps["message"])
	require.Equal(t, map[string]any{"id": "abc"}, corps["data"])
}

func TestOKSansDataOmetLeChampEnReel(t *testing.T) {
	// ANO-011 : la réponse de otp/send ne porte pas de champ data du tout.
	r := NewReal(inmemory.FixedClock{})
	vm := r.SuccessWithoutData(http.StatusOK, "OTP envoyé avec succès")

	corps := bodyMap(t, vm)
	require.NotContains(t, corps, "data")
}

func TestRenduAppliqueLaDerive(t *testing.T) {
	base := time.Date(2026, 8, 21, 11, 0, 0, 0, time.UTC)

	r := NewReal(inmemory.FixedClock{Skew: 9 * time.Minute})
	require.Equal(t, base.Add(9*time.Minute), r.Rendered(base))

	sans := NewReal(inmemory.FixedClock{})
	require.Equal(t, base, sans.Rendered(base))
}

func TestRealValidationAvecChampsGardeConstraintViolation(t *testing.T) {
	r := NewReal(inmemory.FixedClock{})

	vm := r.Failure(entity.Validation(entity.FieldFault{
		ObjectName: "demandeParticulierDTO",
		Field:      "numero",
		Message:    "ne doit pas être vide",
	}))

	require.Equal(t, http.StatusBadRequest, vm.Status)
	corps := bodyMap(t, vm)
	require.Equal(t, "https://www.jhipster.tech/problem/constraint-violation", corps["type"])
	require.Equal(t, "Method argument not valid", corps["title"])
	require.Equal(t, "error.validation", corps["message"])
	require.NotContains(t, corps, "code")
	require.NotEmpty(t, corps["fieldErrors"])
}

func TestRealValidationSansChampsRendMessagePrecis(t *testing.T) {
	// Correction revue #1 : sans Fields, la forme constraint-violation ne
	// serait jamais produite par une pile Spring/JHipster (elle porte
	// toujours au moins un fieldError). FormatJSONInvalide, FlotteVide,
	// ValidationEchouee(msg) doivent donc rendre un problem-with-message en
	// 400 qui porte le message métier.
	r := NewReal(inmemory.FixedClock{})

	vm := r.Failure(entity.ValidationFailed("un message précis"))

	require.Equal(t, http.StatusBadRequest, vm.Status)
	corps := bodyMap(t, vm)
	require.Equal(t, "https://www.jhipster.tech/problem/problem-with-message", corps["type"])
	require.Equal(t, "Bad Request", corps["title"])
	require.Equal(t, float64(400), corps["status"])
	require.Equal(t, "un message précis", corps["detail"])
	require.Equal(t, "error.http.400", corps["message"])
	require.NotContains(t, corps, "code")
	require.NotContains(t, corps, "fieldErrors")
}

func TestFailureAvecFaultNilNePaniquePas(t *testing.T) {
	// Correction revue #2 (portée à la présentation) : httpx.Renderer.Fail
	// normalise un appelant qui fait `return r.Fail(c, err)` avec err nil en
	// entity.InternalError("erreur interne") avant que failReel ne
	// s'exécute. Presenter.Failure reçoit désormais un *entity.Fault déjà
	// normalisé, mais reste défensif : un Fault nil ne doit pas non plus
	// faire paniquer Failure.
	r := NewReal(inmemory.FixedClock{})

	var vm ViewModel
	require.NotPanics(t, func() {
		vm = r.Failure(nil)
	})

	require.Equal(t, http.StatusInternalServerError, vm.Status)
	corps := bodyMap(t, vm)
	require.Equal(t, "RuntimeException: erreur interne", corps["detail"])
}

func TestFailureAvecFaultDeNormalisationTypeNilNePaniquePas(t *testing.T) {
	// Correction revue #2 (portée à la présentation) : un *entity.Fault typé
	// nil, emballé dans un error, fait aussi réussir errors.As avec e ==
	// nil dans httpx.Renderer.Fail ; e.Kind y paniquerait sans sa garde, qui
	// normalise ce cas vers le même entity.InternalError("erreur interne").
	// Ce fault normalisé, transmis à Failure, rend le même corps que pour un
	// Fault nil direct.
	r := NewReal(inmemory.FixedClock{})

	vm := r.Failure(entity.InternalError("erreur interne"))

	require.Equal(t, http.StatusInternalServerError, vm.Status)
	corps := bodyMap(t, vm)
	require.Equal(t, "RuntimeException: erreur interne", corps["detail"])
}

func TestFailureErreurNueDevientRuntimeExceptionAvecSonTexte(t *testing.T) {
	// Correction revue #3 (portée à la présentation) : chemin de repli pour
	// une erreur qui n'est pas un *entity.Fault. httpx.Renderer.Fail la
	// normalise en entity.InternalError(err.Error()) avant de rendre ; ce
	// fault normalisé rend le même détail "RuntimeException: <texte>".
	r := NewReal(inmemory.FixedClock{})

	vm := r.Failure(entity.InternalError("panne imprévue"))

	require.Equal(t, http.StatusInternalServerError, vm.Status)
	corps := bodyMap(t, vm)
	require.Equal(t, "RuntimeException: panne imprévue", corps["detail"])
}
