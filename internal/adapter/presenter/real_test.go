package presenter

import (
	"encoding/json"
	"errors"
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
	var body map[string]any
	require.NoError(t, json.Unmarshal(b, &body))
	return body
}

// R11: these tests duplicate (not move) internal/httpx/renderer_test.go's
// real-fidelity assertions, converted to assert on the returned ViewModel
// instead of a httptest.ResponseRecorder. internal/httpx/renderer_test.go
// stays untouched — httpx.Renderer remains the live production path until
// Task 18.

func TestRealStateErrorRendersAs500WithoutCode(t *testing.T) {
	r := NewReal(inmemory.FixedClock{})

	vm := r.Failure(entity.RequestNotFound(), "/api/gateway/v1/demandes/traitement")

	require.Equal(t, http.StatusInternalServerError, vm.Status)
	body := bodyMap(t, vm)
	require.NotContains(t, body, "code", "ANO-001: no error carries a code field in real mode")
	require.NotContains(t, body, "success")
	require.Equal(t, "https://www.jhipster.tech/problem/problem-with-message", body["type"])
	require.Equal(t, "Internal Server Error", body["title"])
	require.Equal(t, float64(500), body["status"])
	require.Equal(t, "error.http.500", body["message"])
	require.Equal(t, "RuntimeException: Demande introuvable", body["detail"])
	// Finding 1 (fix round 1): the request path, passed as an argument, does
	// reach the rendered body — problem-with-message 500 branch.
	require.Equal(t, "/api/gateway/v1/demandes/traitement", body["path"])
}

func TestRealValidationErrorRendersAs400WithFieldErrors(t *testing.T) {
	r := NewReal(inmemory.FixedClock{})

	vm := r.Failure(entity.Validation(entity.FieldFault{
		ObjectName: "demandeParticulierDTO",
		Field:      "client.lieuNaissance",
		Message:    "ne doit pas être vide",
	}), "/api/gateway/v1/demandes/particulier")

	require.Equal(t, http.StatusBadRequest, vm.Status)
	body := bodyMap(t, vm)
	require.NotContains(t, body, "code")
	require.Equal(t, "https://www.jhipster.tech/problem/constraint-violation", body["type"])
	require.Equal(t, "Method argument not valid", body["title"])
	require.Equal(t, "error.validation", body["message"])
	// Finding 1 (fix round 1): constraint-violation branch.
	require.Equal(t, "/api/gateway/v1/demandes/particulier", body["path"])

	fields := body["fieldErrors"].([]any)
	require.Len(t, fields, 1)
	first := fields[0].(map[string]any)
	require.Equal(t, "demandeParticulierDTO", first["objectName"])
	require.Equal(t, "client.lieuNaissance", first["field"])
	require.Equal(t, "ne doit pas être vide", first["message"])
}

func TestRealCustomDetail(t *testing.T) {
	// ANO-002: refusing a re-porting less than 3 months later presents as a
	// failure.
	r := NewReal(inmemory.FixedClock{})

	vm := r.Failure(entity.PortingDelayNotRespected(), "/api/gateway/v1/demandes/particulier")

	require.Equal(t, http.StatusInternalServerError, vm.Status)
	body := bodyMap(t, vm)
	require.Equal(t, "Unexpected runtime exception", body["detail"])
}

func TestSuccessReal(t *testing.T) {
	r := NewReal(inmemory.FixedClock{})
	vm := r.Success(http.StatusCreated, "Demande créée avec succès", map[string]string{"id": "abc"})

	require.Equal(t, http.StatusCreated, vm.Status)
	body := bodyMap(t, vm)
	require.Equal(t, true, body["success"])
	require.Equal(t, "SUCCESS", body["code"])
	require.Equal(t, "Demande créée avec succès", body["message"])
	require.Equal(t, map[string]any{"id": "abc"}, body["data"])
}

func TestOKWithoutDataOmitsTheFieldInReal(t *testing.T) {
	// ANO-011: otp/send's response carries no data field at all.
	r := NewReal(inmemory.FixedClock{})
	vm := r.SuccessWithoutData(http.StatusOK, "OTP envoyé avec succès")

	body := bodyMap(t, vm)
	require.NotContains(t, body, "data")
}

func TestRenderedAppliesTheSkew(t *testing.T) {
	base := time.Date(2026, 8, 21, 11, 0, 0, 0, time.UTC)

	r := NewReal(inmemory.FixedClock{Skew: 9 * time.Minute})
	require.Equal(t, base.Add(9*time.Minute), r.Rendered(base))

	withoutSkew := NewReal(inmemory.FixedClock{})
	require.Equal(t, base, withoutSkew.Rendered(base))
}

func TestRealValidationWithFieldsKeepsConstraintViolation(t *testing.T) {
	r := NewReal(inmemory.FixedClock{})

	vm := r.Failure(entity.Validation(entity.FieldFault{
		ObjectName: "demandeParticulierDTO",
		Field:      "numero",
		Message:    "ne doit pas être vide",
	}), "/api/gateway/v1/demandes/particulier")

	require.Equal(t, http.StatusBadRequest, vm.Status)
	body := bodyMap(t, vm)
	require.Equal(t, "https://www.jhipster.tech/problem/constraint-violation", body["type"])
	require.Equal(t, "Method argument not valid", body["title"])
	require.Equal(t, "error.validation", body["message"])
	require.NotContains(t, body, "code")
	require.NotEmpty(t, body["fieldErrors"])
}

func TestRealValidationWithoutFieldsRendersAPreciseMessage(t *testing.T) {
	// Review correction #1: without Fields, the constraint-violation shape
	// would never be produced by a real Spring/JHipster stack (it always
	// carries at least one fieldError). FormatJSONInvalide, FlotteVide,
	// ValidationEchouee(msg) must therefore render a problem-with-message in
	// 400 carrying the business message.
	r := NewReal(inmemory.FixedClock{})

	vm := r.Failure(entity.ValidationFailed("a precise message"), "/api/gateway/v1/demandes/flotte")

	require.Equal(t, http.StatusBadRequest, vm.Status)
	body := bodyMap(t, vm)
	require.Equal(t, "https://www.jhipster.tech/problem/problem-with-message", body["type"])
	require.Equal(t, "Bad Request", body["title"])
	require.Equal(t, float64(400), body["status"])
	require.Equal(t, "a precise message", body["detail"])
	require.Equal(t, "error.http.400", body["message"])
	require.NotContains(t, body, "code")
	require.NotContains(t, body, "fieldErrors")
	// Finding 1 (fix round 1): problem-with-message 400 branch.
	require.Equal(t, "/api/gateway/v1/demandes/flotte", body["path"])
}

func TestFailWithNilErrorDoesNotPanic(t *testing.T) {
	// Review correction #2, moved into the pure layer: a caller that does
	// `return httpx.Renderer.Fail(c, err)` with a nil err must not turn the
	// request into a panic. Fix round 1 (finding 2): entity.FaultFrom now
	// carries this normalization; this test exercises it directly, rather
	// than reconstructing its result by hand.
	var f *entity.Fault
	require.NotPanics(t, func() {
		f = entity.FaultFrom(nil)
	})
	require.Equal(t, entity.InternalError("internal error"), f)

	r := NewReal(inmemory.FixedClock{})
	vm := r.Failure(f, "/x")

	require.Equal(t, http.StatusInternalServerError, vm.Status)
	body := bodyMap(t, vm)
	require.Equal(t, "RuntimeException: internal error", body["detail"])
}

func TestFailWithNilApperrErrorTypeDoesNotPanic(t *testing.T) {
	// Review correction #2, moved into the pure layer: a *entity.Fault typed
	// nil, wrapped in an error, makes errors.As succeed with e == nil;
	// e.Kind would panic without the guard. Fix round 1 (finding 2):
	// exercises entity.FaultFrom on this exact case rather than on its
	// already-built result.
	var e *entity.Fault
	var err error = e

	var f *entity.Fault
	require.NotPanics(t, func() {
		f = entity.FaultFrom(err)
	})
	require.Equal(t, entity.InternalError("internal error"), f)

	r := NewReal(inmemory.FixedClock{})
	vm := r.Failure(f, "/x")

	require.Equal(t, http.StatusInternalServerError, vm.Status)
	body := bodyMap(t, vm)
	require.Equal(t, "RuntimeException: internal error", body["detail"])
}

func TestFailBareErrorBecomes500WithItsText(t *testing.T) {
	// Review correction #3, moved into the pure layer: the fallback path for
	// an error that is not a *entity.Fault. Fix round 1 (finding 2):
	// exercises entity.FaultFrom on a bare error rather than on the fault it
	// would produce.
	f := entity.FaultFrom(errors.New("unexpected failure"))
	require.Equal(t, entity.InternalError("unexpected failure"), f)

	r := NewReal(inmemory.FixedClock{})
	vm := r.Failure(f, "/x")

	require.Equal(t, http.StatusInternalServerError, vm.Status)
	body := bodyMap(t, vm)
	require.Equal(t, "RuntimeException: unexpected failure", body["detail"])
}
