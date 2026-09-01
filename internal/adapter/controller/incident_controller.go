package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/presenter"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/incident"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// IncidentController is the interface-adapter for guide §7.12's six routes
// — two families, gateway and interne, sharing the same three handlers
// parameterized by systemLocked, the only dimension where they diverge for
// real. Declared once per family in NewRouter, exactly like every other
// controller's routes.
type IncidentController struct {
	declare incident.DeclareIncidentBoundary
	resolve incident.ResolveIncidentBoundary
	listOwn incident.ListOwnIncidentsBoundary
	pres    presenter.Presenter
	// clock formats dateOuverture — see CreationController's own clock
	// field for why this is the controller's job, not the presenter's.
	clock port.Clock
}

// NewIncidentController wires a controller against the three boundaries, a
// presenter and a clock.
func NewIncidentController(
	declare incident.DeclareIncidentBoundary,
	resolve incident.ResolveIncidentBoundary,
	listOwn incident.ListOwnIncidentsBoundary,
	p presenter.Presenter,
	clock port.Clock,
) *IncidentController {
	return &IncidentController{declare: declare, resolve: resolve, listOwn: listOwn, pres: p, clock: clock}
}

// incidentViewDTO serializes an incident per guide §7.12: {id,
// typeIncidentId, type, figeSysteme, description, statut, dateOuverture,
// operateur{id,nom}}.
func incidentViewDTO(clk port.Clock, v port.IncidentView) map[string]any {
	return map[string]any{
		"id":             v.ID,
		"typeIncidentId": v.TypeID,
		"type":           v.TypeLabel,
		"figeSysteme":    v.SystemLocked,
		"description":    v.Description,
		"statut":         v.Status,
		"dateOuverture":  clk.Rendered(v.OpenedAt),
		"operateur":      map[string]any{"id": v.OperatorID, "nom": v.OperatorName},
	}
}

// incidentBody is the sole shape POST /incidents/{gateway,interne} and its
// resoudre counterpart accept. A typeIncidentId sent alongside is not
// decoded, hence silently ignored (§7.12): it is the URL segment that
// decides the category, never the body.
type incidentBody struct {
	Commentaire string `json:"commentaire"`
}

func (ctl *IncidentController) declarer(c *gin.Context, systemLocked bool) {
	var req incidentBody
	if err := c.ShouldBindJSON(&req); err != nil {
		render(c, ctl.pres.Failure(entity.InvalidJSONFormat(), c.Request.URL.Path))
		return
	}

	view, fault := ctl.declare.Execute(c.Request.Context(), incident.DeclareIncidentInput{
		SystemLocked: systemLocked, Comment: req.Commentaire,
	})
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusCreated, "Incident déclaré avec succès",
		incidentViewDTO(ctl.clock, view)))
}

// DeclarerGateway handles POST /incidents/gateway.
func (ctl *IncidentController) DeclarerGateway(c *gin.Context) { ctl.declarer(c, false) }

// DeclarerInterne handles POST /incidents/interne.
func (ctl *IncidentController) DeclarerInterne(c *gin.Context) { ctl.declarer(c, true) }

func (ctl *IncidentController) resoudre(c *gin.Context, systemLocked bool) {
	var req incidentBody
	if err := c.ShouldBindJSON(&req); err != nil {
		render(c, ctl.pres.Failure(entity.InvalidJSONFormat(), c.Request.URL.Path))
		return
	}

	view, fault := ctl.resolve.Execute(c.Request.Context(), incident.ResolveIncidentInput{
		IncidentID: c.Param("id"), SystemLocked: systemLocked, Comment: req.Commentaire,
	})
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Incident résolu avec succès",
		incidentViewDTO(ctl.clock, view)))
}

// ResoudreGateway handles POST /incidents/gateway/:id/resoudre.
func (ctl *IncidentController) ResoudreGateway(c *gin.Context) { ctl.resoudre(c, false) }

// ResoudreInterne handles POST /incidents/interne/:id/resoudre.
func (ctl *IncidentController) ResoudreInterne(c *gin.Context) { ctl.resoudre(c, true) }

func (ctl *IncidentController) mesIncidents(c *gin.Context, systemLocked bool) {
	page := parseQueryInt(c, "page", 0)
	size := parseQueryInt(c, "size", 20)

	appelant := port.CallerFromContext(c.Request.Context())
	views, fault := ctl.listOwn.Execute(c.Request.Context(), appelant.OperatorID, systemLocked, page, size)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}

	out := make([]map[string]any, 0, len(views))
	for _, v := range views {
		out = append(out, incidentViewDTO(ctl.clock, v))
	}
	render(c, ctl.pres.Success(http.StatusOK, "Incidents récupérés avec succès", out))
}

// MesIncidentsGateway handles GET /incidents/gateway/mes-incidents.
func (ctl *IncidentController) MesIncidentsGateway(c *gin.Context) { ctl.mesIncidents(c, false) }

// MesIncidentsInterne handles GET /incidents/interne/mes-incidents.
func (ctl *IncidentController) MesIncidentsInterne(c *gin.Context) { ctl.mesIncidents(c, true) }
