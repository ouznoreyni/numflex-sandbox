package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/presenter"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/sandbox"
)

// SandboxController is the interface-adapter for the two
// /api/sandbox/v1 routes — hors gateway, hors contrat ARTP: the test-data
// purge, and the registry's ranges. Both are mounted unconditionally and
// both answer only to a token; nothing here is gated by configuration.
type SandboxController struct {
	purge  sandbox.PurgeTestDataBoundary
	ranges sandbox.CountNumberRangesBoundary
	pres   presenter.Presenter
}

// NewSandboxController wires a controller against its two boundaries and a
// presenter.
func NewSandboxController(
	purge sandbox.PurgeTestDataBoundary,
	ranges sandbox.CountNumberRangesBoundary,
	p presenter.Presenter,
) *SandboxController {
	return &SandboxController{purge: purge, ranges: ranges, pres: p}
}

// numberRangeDTO is one range of the registry, its json tags being the
// field names the route publishes.
type numberRangeDTO struct {
	Prefix string `json:"prefixe"`
	First  string `json:"premier"`
	Last   string `json:"dernier"`
	Total  int64  `json:"total"`
	Nature string `json:"nature"`
}

// Ranges handles GET /api/sandbox/v1/numeros/tranches?operateur=ORANGE.
// The parameter is the route's whole input: an enum of the operators the
// registry knows, resolved by the interactor, so a typo comes back as the
// 400 a bean validation would give rather than as the contract's 500.
func (ctl *SandboxController) Ranges(c *gin.Context) {
	out, fault := ctl.ranges.Execute(c.Request.Context(), c.Query("operateur"))
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}

	tranches := make([]numberRangeDTO, 0, len(out.Ranges))
	for _, r := range out.Ranges {
		tranches = append(tranches, numberRangeDTO{
			Prefix: r.Prefix, First: r.First, Last: r.Last,
			Total: r.Total, Nature: r.Nature,
		})
	}

	render(c, ctl.pres.Success(http.StatusOK,
		"Tranches de l'opérateur "+out.Operator, map[string]any{
			"operateur":      out.Operator,
			"operateurId":    out.OperatorID,
			"nombreTranches": len(tranches),
			"totalNumeros":   out.Total,
			"tranches":       tranches,
		}))
}

// Purge handles DELETE /api/sandbox/v1/demandes.
func (ctl *SandboxController) Purge(c *gin.Context) {
	result, fault := ctl.purge.Execute(c.Request.Context())
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demandes purgées avec succès", map[string]any{
		"demandesSupprimees": result.RequestsDeleted,
		"numerosRestaures":   result.NumbersRestored,
		"otpSupprimes":       result.OTPDeleted,
		"reverseSupprimees":  result.ReverseDeleted,
	}))
}
