package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/presenter"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/reference"
)

// ReferenceController is the interface-adapter for the five reference-data
// routes: /operateurs, /motifs-rejet, /types-demande, /processus and
// /types-incident. Each handler below does the same three things — call its
// boundary, map the returned entity slice onto the DTO whose json tags are
// the contract's exact field names, hand the result to the presenter — and
// nothing else: there is no field to bind and no rule to enforce on a GET
// with no body.
type ReferenceController struct {
	operators        reference.ListOperatorsBoundary
	rejectionReasons reference.ListRejectionReasonsBoundary
	requestTypes     reference.ListRequestTypesBoundary
	processes        reference.ListProcessesBoundary
	incidentTypes    reference.ListIncidentTypesBoundary
	pres             presenter.Presenter
}

// NewReferenceController wires a controller against the five boundaries and
// a presenter, exactly as NewOTPController and NewAuthController do.
func NewReferenceController(
	operators reference.ListOperatorsBoundary,
	rejectionReasons reference.ListRejectionReasonsBoundary,
	requestTypes reference.ListRequestTypesBoundary,
	processes reference.ListProcessesBoundary,
	incidentTypes reference.ListIncidentTypesBoundary,
	p presenter.Presenter,
) *ReferenceController {
	return &ReferenceController{
		operators:        operators,
		rejectionReasons: rejectionReasons,
		requestTypes:     requestTypes,
		processes:        processes,
		incidentTypes:    incidentTypes,
		pres:             p,
	}
}

type operatorDTO struct {
	ID  string `json:"id"`
	Nom string `json:"nom"`
}

// Operators handles GET /operateurs.
func (ctl *ReferenceController) Operators(c *gin.Context) {
	out, fault := ctl.operators.Execute(c.Request.Context())
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	dto := make([]operatorDTO, 0, len(out))
	for _, o := range out {
		dto = append(dto, operatorDTO{ID: o.ID, Nom: o.Name})
	}
	render(c, ctl.pres.Success(http.StatusOK, "Opérateurs récupérés avec succès", dto))
}

// motifRejetDTO's field is "motif", not "libelle" — ANO-009, the v2 guide
// documents it this way.
type motifRejetDTO struct {
	ID    string `json:"id"`
	Motif string `json:"motif"`
}

// RejectionReasons handles GET /motifs-rejet.
func (ctl *ReferenceController) RejectionReasons(c *gin.Context) {
	out, fault := ctl.rejectionReasons.Execute(c.Request.Context())
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	dto := make([]motifRejetDTO, 0, len(out))
	for _, m := range out {
		dto = append(dto, motifRejetDTO{ID: m.ID, Motif: m.Reason})
	}
	render(c, ctl.pres.Success(http.StatusOK, "Motifs de rejet récupérés avec succès", dto))
}

type typeDemandeDTO struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// RequestTypes handles GET /types-demande.
func (ctl *ReferenceController) RequestTypes(c *gin.Context) {
	out, fault := ctl.requestTypes.Execute(c.Request.Context())
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	dto := make([]typeDemandeDTO, 0, len(out))
	for _, t := range out {
		dto = append(dto, typeDemandeDTO{ID: t.ID, Type: t.Type})
	}
	render(c, ctl.pres.Success(http.StatusOK, "Types de demande récupérés avec succès", dto))
}

type processusDTO struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// Processes handles GET /processus.
func (ctl *ReferenceController) Processes(c *gin.Context) {
	out, fault := ctl.processes.Execute(c.Request.Context())
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	dto := make([]processusDTO, 0, len(out))
	for _, p := range out {
		dto = append(dto, processusDTO{ID: p.ID, Type: p.Type})
	}
	render(c, ctl.pres.Success(http.StatusOK, "Processus récupérés avec succès", dto))
}

type typeIncidentDTO struct {
	ID          string `json:"id"`
	Libelle     string `json:"libelle"`
	FigeSysteme bool   `json:"figeSysteme"`
}

// IncidentTypes handles GET /types-incident.
func (ctl *ReferenceController) IncidentTypes(c *gin.Context) {
	out, fault := ctl.incidentTypes.Execute(c.Request.Context())
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	dto := make([]typeIncidentDTO, 0, len(out))
	for _, ti := range out {
		dto = append(dto, typeIncidentDTO{ID: ti.ID, Libelle: ti.Label, FigeSysteme: ti.SystemLocked})
	}
	render(c, ctl.pres.Success(http.StatusOK, "Types d'incident récupérés avec succès", dto))
}
