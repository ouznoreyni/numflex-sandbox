package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/presenter"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/acceptance"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// AcceptanceController is the interface-adapter for the two acceptance
// routes: POST /demandes/acceptation (particulier/restitution) and
// POST /demandes/:id/acceptation (entreprise/flotte). Each handler checks
// the frozen-market gate FIRST — before binding any JSON — then binds,
// applies the one shape validation that is not the interactor's business
// (idDemande must not be empty), delegates to a boundary, and renders the
// result — the same division CreationController and QueryController
// already establish for everything after the frozen-market gate.
type AcceptanceController struct {
	individual acceptance.AcceptRequestBoundary
	fleet      acceptance.AcceptFleetRequestBoundary
	pres       presenter.Presenter
	// clock formats the two timestamps a rendered request carries — see
	// CreationController's own clock field for why this is the
	// controller's job, not the presenter's.
	clock port.Clock
	// engine backs the frozen-market check both handlers make before
	// binding their body (see acceptance.MarketFrozen's doc comment for
	// why that order matters, and why this lives here rather than inside
	// each Execute).
	engine port.Engine
}

// NewAcceptanceController wires a controller against the two boundaries, a
// presenter, a clock and the engine port the frozen-market gate reads.
func NewAcceptanceController(
	individual acceptance.AcceptRequestBoundary,
	fleet acceptance.AcceptFleetRequestBoundary,
	p presenter.Presenter,
	clock port.Clock,
	engine port.Engine,
) *AcceptanceController {
	return &AcceptanceController{
		individual: individual, fleet: fleet, pres: p, clock: clock, engine: engine,
	}
}

// repondre renders the response common to both handlers: guide §7.3's
// shape, via the package-level requestViewDTO (request_view.go) — the one
// shared implementation Task 15 consolidated ruling R28's deliberate
// duplicates into — under the exact message the contract carries.
func (ctl *AcceptanceController) repondre(c *gin.Context, view port.RequestView) {
	render(c, ctl.pres.Success(http.StatusOK, "Décision d'acceptation enregistrée",
		requestViewDTO(ctl.clock, view)))
}

// --- Particulier / restitution ----------------------------------------------

type acceptationRequest struct {
	IDDemande    string `json:"idDemande"`
	Accepte      bool   `json:"accepte"`
	MotifRejetID string `json:"motifRejetId"`
	Commentaire  string `json:"commentaire"`
}

// Acceptation handles POST /demandes/acceptation.
func (ctl *AcceptanceController) Acceptation(c *gin.Context) {
	if f := acceptance.MarketFrozen(c.Request.Context(), ctl.engine); f != nil {
		render(c, ctl.pres.Failure(f, c.Request.URL.Path))
		return
	}

	var req acceptationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		render(c, ctl.pres.Failure(entity.InvalidJSONFormat(), c.Request.URL.Path))
		return
	}
	// Rupture v1 → v2 : la demande n'est plus identifiée par numero (R-10 §8).
	// Un corps qui n'envoie que numero laisse idDemande vide et échoue ici.
	if req.IDDemande == "" {
		render(c, ctl.pres.Failure(entity.Validation(entity.FieldFault{
			ObjectName: "acceptationDTO", Field: "idDemande",
			Message: "ne doit pas être vide",
		}), c.Request.URL.Path))
		return
	}

	view, fault := ctl.individual.Execute(c.Request.Context(), acceptance.AcceptRequestInput{
		RequestID: req.IDDemande, Accept: req.Accepte,
		RejectionReasonID: req.MotifRejetID, Comment: req.Commentaire,
	})
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	ctl.repondre(c, view)
}

// --- Entreprise (flotte) ------------------------------------------------------

type numeroRejeteFlotteDTO struct {
	Numero       string `json:"numero"`
	MotifRejetID string `json:"motifRejetId"`
}

type acceptationFlotteRequest struct {
	Accepte        bool                    `json:"accepte"`
	NumerosRejetes []numeroRejeteFlotteDTO `json:"numerosRejetes"`
	MotifRejetID   string                  `json:"motifRejetId"`
	Commentaire    string                  `json:"commentaire"`
}

// AcceptationFlotte handles POST /demandes/:id/acceptation.
func (ctl *AcceptanceController) AcceptationFlotte(c *gin.Context) {
	if f := acceptance.MarketFrozen(c.Request.Context(), ctl.engine); f != nil {
		render(c, ctl.pres.Failure(f, c.Request.URL.Path))
		return
	}

	var req acceptationFlotteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		render(c, ctl.pres.Failure(entity.InvalidJSONFormat(), c.Request.URL.Path))
		return
	}

	rejetes := make([]acceptance.RejectedNumberInput, 0, len(req.NumerosRejetes))
	for _, nr := range req.NumerosRejetes {
		rejetes = append(rejetes, acceptance.RejectedNumberInput{
			MSISDN: nr.Numero, RejectionReasonID: nr.MotifRejetID,
		})
	}

	view, fault := ctl.fleet.Execute(c.Request.Context(), acceptance.AcceptFleetRequestInput{
		RequestID: c.Param("id"), Accept: req.Accepte,
		RejectedNumbers: rejetes, RejectionReasonID: req.MotifRejetID, Comment: req.Commentaire,
	})
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	ctl.repondre(c, view)
}
