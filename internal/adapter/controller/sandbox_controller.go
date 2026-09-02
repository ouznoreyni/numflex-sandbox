package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/presenter"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/sandbox"
)

// SandboxController is the interface-adapter for
// DELETE /api/sandbox/v1/demandes — hors gateway, hors contrat ARTP. Its
// route is registered only when config.SandboxAdmin is true (internal/api's
// own concern, not this package's): unlike every other controller here, an
// unauthenticated or invalid caller against a disabled purge never reaches
// this type at all — the group simply is not mounted, and gin answers 404.
type SandboxController struct {
	purge sandbox.PurgeTestDataBoundary
	pres  presenter.Presenter
}

// NewSandboxController wires a controller against the one boundary and a
// presenter.
func NewSandboxController(purge sandbox.PurgeTestDataBoundary, p presenter.Presenter) *SandboxController {
	return &SandboxController{purge: purge, pres: p}
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
