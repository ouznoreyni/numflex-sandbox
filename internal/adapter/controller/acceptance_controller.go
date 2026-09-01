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
// POST /demandes/:id/acceptation (entreprise/flotte). Each handler binds
// JSON, applies the one shape validation that is not the interactor's
// business (idDemande must not be empty), delegates to a boundary, and
// renders the result — the same division CreationController and
// QueryController already establish.
type AcceptanceController struct {
	individual acceptance.AcceptRequestBoundary
	fleet      acceptance.AcceptFleetRequestBoundary
	pres       presenter.Presenter
	// clock formats the two timestamps a rendered request carries — see
	// CreationController's own clock field for why this is the
	// controller's job, not the presenter's.
	clock port.Clock
}

// NewAcceptanceController wires a controller against the two boundaries, a
// presenter and a clock.
func NewAcceptanceController(
	individual acceptance.AcceptRequestBoundary,
	fleet acceptance.AcceptFleetRequestBoundary,
	p presenter.Presenter,
	clock port.Clock,
) *AcceptanceController {
	return &AcceptanceController{individual: individual, fleet: fleet, pres: p, clock: clock}
}

// requestViewDTO sérialise une demande au format du guide §7.3 — a third
// deliberate duplicate of CreationController's and QueryController's own
// (ruling R28): internal/api/dto.go's Deps.demandeDTO still serves
// annulation.go and traitement.go, so this task cannot consolidate the
// three copies into one without breaking their compilation. Task 15, which
// migrates the last of those two callers, is where that consolidation is
// due.
func (ctl *AcceptanceController) requestViewDTO(v port.RequestView) map[string]any {
	out := map[string]any{
		"id":                    v.ID,
		"numero":                v.MSISDN,
		"typeAbonne":            v.SubscriberType,
		"typeDemande":           v.RequestType,
		"statutDemande":         v.Status,
		"etapeActuelle":         v.CurrentStep,
		"statutEtapeActuel":     v.CurrentStepStatus,
		"operateurSource":       map[string]any{"id": v.SourceOperatorID, "nom": v.SourceOperatorName},
		"operateurDestinataire": map[string]any{"id": v.RecipientOperatorID, "nom": v.RecipientOperatorName},
		"dateDemande":           ctl.clock.Rendered(v.RequestDate),
		"processus":             nil,
		"routageInfo":           nil,
	}
	if v.Processus != nil {
		out["processus"] = *v.Processus
	}
	if v.RoutingInfo != nil {
		out["routageInfo"] = *v.RoutingInfo
	}
	if v.CompletionDate != nil {
		out["dateFinalisation"] = ctl.clock.Rendered(*v.CompletionDate)
	}
	if v.Client != nil {
		client := map[string]any{
			"nom":           v.Client.LastName,
			"prenom":        v.Client.FirstName,
			"dateNaissance": "",
			"lieuNaissance": v.Client.BirthPlace,
			"typePiece":     v.Client.IDType,
			"numeroPiece":   v.Client.IDNumber,
		}
		if v.Client.BirthDate != nil {
			client["dateNaissance"] = v.Client.BirthDate.Format("2006-01-02")
		}
		out["client"] = client
	}
	return out
}

// repondre renders the response common to both handlers: guide §7.3's
// shape, under the exact message the contract carries.
func (ctl *AcceptanceController) repondre(c *gin.Context, view port.RequestView) {
	render(c, ctl.pres.Success(http.StatusOK, "Décision d'acceptation enregistrée",
		ctl.requestViewDTO(view)))
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
