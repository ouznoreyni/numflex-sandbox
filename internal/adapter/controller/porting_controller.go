package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/presenter"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/acceptance"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/porting"
)

// PortingController is the interface-adapter for the three routes a request
// goes through once accepted: POST /demandes/a-confirmer,
// POST /demandes/traitement and POST /demandes/:id/annuler. Confirmation
// and traitement each check the frozen-market gate FIRST — before binding
// any JSON — via acceptance.MarketFrozen: the same pure function
// AcceptanceController already calls, exported precisely so that its own
// doc comment's "the same call opens /a-confirmer and /traitement" would
// hold. Annulation does not check it: [HYP], preserved unchanged from the
// deleted internal/api/annulation.go's own comment — cancelling a request
// still at ACCEPTATION withdraws it rather than processing a step.
type PortingController struct {
	confirm porting.ConfirmRequestBoundary
	process porting.ProcessStepBoundary
	cancel  porting.CancelRequestBoundary
	pres    presenter.Presenter
	// clock formats the two timestamps a rendered request carries — see
	// CreationController's own clock field for why this is the
	// controller's job, not the presenter's.
	clock port.Clock
	// engine backs the frozen-market check Confirmation and Traitement make
	// before binding their body (see acceptance.MarketFrozen's doc comment
	// for why that order matters, and why this lives here rather than
	// inside either Execute).
	engine port.Engine
}

// NewPortingController wires a controller against the three boundaries, a
// presenter, a clock and the engine port the frozen-market gate reads.
func NewPortingController(
	confirm porting.ConfirmRequestBoundary,
	process porting.ProcessStepBoundary,
	cancel porting.CancelRequestBoundary,
	p presenter.Presenter,
	clock port.Clock,
	engine port.Engine,
) *PortingController {
	return &PortingController{
		confirm: confirm, process: process, cancel: cancel,
		pres: p, clock: clock, engine: engine,
	}
}

// --- Confirmation ------------------------------------------------------------

type confirmationRequest struct {
	IDDemande   string `json:"idDemande"`
	Commentaire string `json:"commentaire"`
}

// Confirmation handles POST /demandes/a-confirmer.
func (ctl *PortingController) Confirmation(c *gin.Context) {
	if f := acceptance.MarketFrozen(c.Request.Context(), ctl.engine); f != nil {
		render(c, ctl.pres.Failure(f, c.Request.URL.Path))
		return
	}

	var req confirmationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		render(c, ctl.pres.Failure(entity.InvalidJSONFormat(), c.Request.URL.Path))
		return
	}
	if req.IDDemande == "" {
		render(c, ctl.pres.Failure(entity.Validation(entity.FieldFault{
			ObjectName: "confirmationDTO", Field: "idDemande",
			Message: "ne doit pas être vide",
		}), c.Request.URL.Path))
		return
	}

	view, fault := ctl.confirm.Execute(c.Request.Context(), porting.ConfirmRequestInput{
		RequestID: req.IDDemande, Comment: req.Commentaire,
	})
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	// Sans client : les captures des deux confirmations, orange puis
	// expresso, n'en portent pas, là où toutes les autres réponses en
	// portent un.
	render(c, ctl.pres.Success(http.StatusOK, "Étape traitée avec succès",
		sansClient(requestViewDTO(ctl.clock, view))))
}

// --- Traitement ------------------------------------------------------------

// traitementRequest ne déclare PAS de champ etape : retiré en v2. Un client
// v1 qui l'envoie encore n'est ni rejeté ni averti — le champ est simplement
// ignoré et l'étape courante est exécutée (ANO-018).
type traitementRequest struct {
	IDDemande   string `json:"idDemande"`
	Commentaire string `json:"commentaire"`
}

// Traitement handles POST /demandes/traitement.
func (ctl *PortingController) Traitement(c *gin.Context) {
	if f := acceptance.MarketFrozen(c.Request.Context(), ctl.engine); f != nil {
		render(c, ctl.pres.Failure(f, c.Request.URL.Path))
		return
	}

	var req traitementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		render(c, ctl.pres.Failure(entity.InvalidJSONFormat(), c.Request.URL.Path))
		return
	}
	if req.IDDemande == "" {
		render(c, ctl.pres.Failure(entity.Validation(entity.FieldFault{
			ObjectName: "traitementDTO", Field: "idDemande", Message: "ne doit pas être vide",
		}), c.Request.URL.Path))
		return
	}

	view, fault := ctl.process.Execute(c.Request.Context(), porting.ProcessStepInput{
		RequestID: req.IDDemande, Comment: req.Commentaire,
	})
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Étape traitée avec succès",
		requestViewDTO(ctl.clock, view)))
}

// --- Annulation --------------------------------------------------------------

// Annulation handles POST /demandes/:id/annuler. §7.11 : aucun corps n'est
// requis, et aucun n'est lu — le seul identifiant utile est déjà dans l'URL.
func (ctl *PortingController) Annulation(c *gin.Context) {
	view, fault := ctl.cancel.Execute(c.Request.Context(), c.Param("id"))
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demande annulée avec succès",
		requestViewDTO(ctl.clock, view)))
}
