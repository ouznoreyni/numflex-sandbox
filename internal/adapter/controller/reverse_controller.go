package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/presenter"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/reverse"
)

// ReverseController is the interface-adapter for guide §6's two routes:
// POST /reverse-requests and GET /reverse-requests/mes-demandes.
type ReverseController struct {
	submit  reverse.SubmitReverseRequestBoundary
	listOwn reverse.ListOwnReverseRequestsBoundary
	pres    presenter.Presenter
	// clock formats dateDemande — see CreationController's own clock field
	// for why this is the controller's job, not the presenter's.
	clock port.Clock
}

// NewReverseController wires a controller against the two boundaries, a
// presenter and a clock.
func NewReverseController(
	submit reverse.SubmitReverseRequestBoundary,
	listOwn reverse.ListOwnReverseRequestsBoundary,
	p presenter.Presenter,
	clock port.Clock,
) *ReverseController {
	return &ReverseController{submit: submit, listOwn: listOwn, pres: p, clock: clock}
}

// reverseViewDTO serializes a reverse request per guide §6:
// {id, numero, statut, dateDemande, operateur{id,nom}}.
func reverseViewDTO(clk port.Clock, v port.ReverseView) map[string]any {
	return map[string]any{
		"id":          v.ID,
		"numero":      v.MSISDN,
		"statut":      v.Status,
		"dateDemande": clk.Rendered(v.RequestDate),
		"operateur":   map[string]any{"id": v.OperatorID, "nom": v.OperatorName},
	}
}

type reverseRequestBody struct {
	Numero string `json:"numero"`
}

// Soumission handles POST /reverse-requests.
func (ctl *ReverseController) Soumission(c *gin.Context) {
	var req reverseRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		render(c, ctl.pres.Failure(entity.InvalidJSONFormat(), c.Request.URL.Path))
		return
	}
	if !motifMSISDN.MatchString(req.Numero) {
		render(c, ctl.pres.Failure(entity.Validation(entity.FieldFault{
			ObjectName: "reverseRequestDTO", Field: "numero",
			Message: "doit correspondre à \"^[0-9]{9}$\"",
		}), c.Request.URL.Path))
		return
	}

	view, fault := ctl.submit.Execute(c.Request.Context(), reverse.SubmitReverseRequestInput{
		MSISDN: req.Numero,
	})
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusCreated, "Demande de reverse soumise avec succès",
		reverseViewDTO(ctl.clock, view)))
}

// MesDemandes handles GET /reverse-requests/mes-demandes.
func (ctl *ReverseController) MesDemandes(c *gin.Context) {
	page := parseQueryInt(c, "page", 0)
	size := parseQueryInt(c, "size", 20)

	appelant := port.CallerFromContext(c.Request.Context())
	views, fault := ctl.listOwn.Execute(c.Request.Context(), appelant.OperatorID, page, size)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}

	out := make([]map[string]any, 0, len(views))
	for _, v := range views {
		out = append(out, reverseViewDTO(ctl.clock, v))
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demandes de reverse récupérées avec succès", out))
}
