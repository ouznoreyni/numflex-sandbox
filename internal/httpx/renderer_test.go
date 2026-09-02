package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/stretchr/testify/require"
)

func contexte(fid config.Fidelity, chemin string) (*gin.Context, *httptest.ResponseRecorder, *Renderer) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, chemin, nil)
	return c, rec, NewRenderer(fid, 0)
}

func TestRealErreurEtatSortEn500SansCode(t *testing.T) {
	c, rec, r := contexte(config.FidelityReal, "/api/gateway/v1/demandes/traitement")

	r.Fail(c, entity.RequestNotFound())

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))

	var corps map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &corps))
	require.NotContains(t, corps, "code", "ANO-001 : aucune erreur ne porte de champ code en mode réel")
	require.NotContains(t, corps, "success")
	require.Equal(t, "https://www.jhipster.tech/problem/problem-with-message", corps["type"])
	require.Equal(t, "Internal Server Error", corps["title"])
	require.Equal(t, float64(500), corps["status"])
	require.Equal(t, "error.http.500", corps["message"])
	require.Equal(t, "/api/gateway/v1/demandes/traitement", corps["path"])
	require.Equal(t, "RuntimeException: Demande introuvable", corps["detail"])
}

func TestRealErreurValidationSortEn400AvecFieldErrors(t *testing.T) {
	c, rec, r := contexte(config.FidelityReal, "/api/gateway/v1/demandes/particulier")

	r.Fail(c, entity.Validation(entity.FieldFault{
		ObjectName: "demandeParticulierDTO",
		Field:      "client.lieuNaissance",
		Message:    "ne doit pas être vide",
	}))

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var corps map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &corps))
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
	// ANO-002: refusing a re-porting inside 3 months presents itself as a failure.
	c, rec, r := contexte(config.FidelityReal, "/api/gateway/v1/demandes/particulier")

	r.Fail(c, entity.PortingDelayNotRespected())

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	var corps map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &corps))
	require.Equal(t, "Unexpected runtime exception", corps["detail"])
}

func TestContractErreurEtatSortEnveloppee(t *testing.T) {
	c, rec, r := contexte(config.FidelityContract, "/api/gateway/v1/demandes/traitement")

	r.Fail(c, entity.RequestNotFound())

	require.Equal(t, http.StatusNotFound, rec.Code)

	var corps map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &corps))
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
		c, rec, r := contexte(config.FidelityContract, "/x")
		r.Fail(c, x.err)
		require.Equalf(t, x.statut, rec.Code, "code %s", x.code)
		var corps map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &corps))
		require.Equal(t, x.code, corps["code"])
	}
}

func TestSucces(t *testing.T) {
	for _, fid := range []config.Fidelity{config.FidelityReal, config.FidelityContract} {
		c, rec, r := contexte(fid, "/x")
		r.OK(c, http.StatusCreated, "Demande créée avec succès", map[string]string{"id": "abc"})

		require.Equal(t, http.StatusCreated, rec.Code)
		var corps map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &corps))
		require.Equal(t, true, corps["success"])
		require.Equal(t, "SUCCESS", corps["code"])
		require.Equal(t, "Demande créée avec succès", corps["message"])
		require.Equal(t, map[string]any{"id": "abc"}, corps["data"])
	}
}

func TestOKSansDataOmetLeChampEnReel(t *testing.T) {
	// ANO-011: otp/send's response carries no data field at all.
	c, rec, r := contexte(config.FidelityReal, "/x")
	r.OKSansData(c, http.StatusOK, "OTP envoyé avec succès")

	var corps map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &corps))
	require.NotContains(t, corps, "data")
}

func TestOKSansDataRendDataNullEnContrat(t *testing.T) {
	c, rec, r := contexte(config.FidelityContract, "/x")
	r.OKSansData(c, http.StatusOK, "OTP envoyé avec succès")

	require.Contains(t, rec.Body.String(), `"data":null`)
}

func TestSkew(t *testing.T) {
	r := NewRenderer(config.FidelityReal, 540*time.Second)
	base := time.Date(2026, 8, 21, 11, 0, 0, 0, time.UTC)
	require.Equal(t, base.Add(9*time.Minute), r.Skew(base))

	sans := NewRenderer(config.FidelityReal, 0)
	require.Equal(t, base, sans.Skew(base))
}

func TestRealValidationAvecChampsGardeConstraintViolation(t *testing.T) {
	c, rec, r := contexte(config.FidelityReal, "/api/gateway/v1/demandes/particulier")

	r.Fail(c, entity.Validation(entity.FieldFault{
		ObjectName: "demandeParticulierDTO",
		Field:      "numero",
		Message:    "ne doit pas être vide",
	}))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var corps map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &corps))
	require.Equal(t, "https://www.jhipster.tech/problem/constraint-violation", corps["type"])
	require.Equal(t, "Method argument not valid", corps["title"])
	require.Equal(t, "error.validation", corps["message"])
	require.NotContains(t, corps, "code")
	require.NotEmpty(t, corps["fieldErrors"])
}

func TestRealValidationSansChampsRendMessagePrecis(t *testing.T) {
	// Review round 1 fix: without Fields, the constraint-violation shape would
	// never be produced by a Spring/JHipster stack (it always carries at least one
	// fieldError). FormatJSONInvalide, FlotteVide and ValidationEchouee(msg) must
	// therefore render a problem-with-message in 400 carrying the business message.
	c, rec, r := contexte(config.FidelityReal, "/api/gateway/v1/demandes/flotte")

	r.Fail(c, entity.ValidationFailed("un message précis"))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var corps map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &corps))
	require.Equal(t, "https://www.jhipster.tech/problem/problem-with-message", corps["type"])
	require.Equal(t, "Bad Request", corps["title"])
	require.Equal(t, float64(400), corps["status"])
	require.Equal(t, "un message précis", corps["detail"])
	require.Equal(t, "error.http.400", corps["message"])
	require.NotContains(t, corps, "code")
	require.NotContains(t, corps, "fieldErrors")
}

func TestFailAvecErreurNilNePaniquePas(t *testing.T) {
	// Review round 2 fix: a caller doing `return r.Fail(c, err)` with a nil err
	// must not turn the request into a panic recovered by gin.
	c, rec, r := contexte(config.FidelityReal, "/x")

	require.NotPanics(t, func() {
		r.Fail(c, nil)
	})

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))
}

func TestFailAvecApperrErrorTypeNilNePaniquePas(t *testing.T) {
	// Review round 2 fix: a typed-nil *entity.Fault wrapped in an error makes
	// errors.As succeed with e == nil; e.Kind would panic without the guard.
	c, rec, r := contexte(config.FidelityReal, "/x")

	var e *entity.Fault
	var err error = e

	require.NotPanics(t, func() {
		r.Fail(c, err)
	})

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))
}

func TestFailErreurNueDevient500AvecSonTexte(t *testing.T) {
	// Review round 3 fix: the fallback path for an error that is not an
	// *entity.Fault.
	c, rec, r := contexte(config.FidelityReal, "/x")

	r.Fail(c, errors.New("panne imprévue"))

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	var corps map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &corps))
	require.Equal(t, "RuntimeException: panne imprévue", corps["detail"])
}

func TestFailErreurEmballeeConserveKindEtMessage(t *testing.T) {
	// Review round 3 fix: errors.As must unwrap an error wrapped by
	// fmt.Errorf("...: %w", ...), as the downstream tasks will do.
	c, rec, r := contexte(config.FidelityContract, "/x")

	err := fmt.Errorf("contexte : %w", entity.RequestNotFound())
	r.Fail(c, err)

	require.Equal(t, http.StatusNotFound, rec.Code)
	var corps map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &corps))
	require.Equal(t, "DEMANDE_NON_TROUVEE", corps["code"])
	require.Equal(t, "Demande introuvable", corps["message"])
}
