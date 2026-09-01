package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/presenter"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/query"
)

// QueryController is the interface-adapter for the seven read-only routes:
// /demandes/mes-demandes, /demandes/a-accepter (+detail),
// /demandes/a-traiter (+detail), /demandes/a-confirmer (+detail),
// /demandes/deja-confirmees, /demandes/in, /demandes/out. Each handler
// drives one boundary and renders []port.RequestView (or a single
// port.RequestView for a detail route) into the guide §7.3 shape — the
// requestViewDTO assembly below is a deliberate duplicate of
// CreationController's, not a new third shape (ruling R28): both exist only
// until acceptation.go, annulation.go, confirmation.go and traitement.go
// migrate off internal/api/dto.go's Deps.demandeDTO, their last remaining
// caller.
type QueryController struct {
	own              query.OwnBoundary
	toAccept         query.ToAcceptBoundary
	toProcess        query.ToProcessBoundary
	toConfirm        query.ToConfirmBoundary
	alreadyConfirmed query.AlreadyConfirmedBoundary
	incoming         query.IncomingBoundary
	outgoing         query.OutgoingBoundary
	pres             presenter.Presenter
	// clock formats the two timestamps a rendered request carries
	// (dateDemande, dateFinalisation) — see CreationController's own clock
	// field for why this is the controller's job, not the presenter's.
	clock port.Clock
}

// NewQueryController wires a controller against the seven boundaries, a
// presenter and a clock.
func NewQueryController(
	own query.OwnBoundary,
	toAccept query.ToAcceptBoundary,
	toProcess query.ToProcessBoundary,
	toConfirm query.ToConfirmBoundary,
	alreadyConfirmed query.AlreadyConfirmedBoundary,
	incoming query.IncomingBoundary,
	outgoing query.OutgoingBoundary,
	p presenter.Presenter,
	clock port.Clock,
) *QueryController {
	return &QueryController{
		own: own, toAccept: toAccept, toProcess: toProcess, toConfirm: toConfirm,
		alreadyConfirmed: alreadyConfirmed, incoming: incoming, outgoing: outgoing,
		pres: p, clock: clock,
	}
}

// requestViewDTO sérialise une demande au format du guide §7.3 — the same
// duplicate of the pre-migration Deps.demandeDTO that
// CreationController.requestViewDTO already carries, field for field.
func (ctl *QueryController) requestViewDTO(v port.RequestView) map[string]any {
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

// sansClient retire le sous-objet client d'un DTO. Les trois endpoints de
// confirmation — la file, son détail et le POST — sont les seuls à ne pas le
// porter ; c'est mesuré sur quatre captures du 2026-08-27, pas déduit. Local
// duplicate of internal/api/dto.go's sansClient, same reasoning as
// requestViewDTO above.
func sansClient(dto map[string]any) map[string]any {
	delete(dto, "client")
	return dto
}

// dtoList maps a slice of views onto their DTOs. It never returns nil: even
// an empty views slice yields a non-nil, zero-length slice, so the caller's
// JSON "data" field renders as [] — never null — exactly as
// Deps.rendreListe was written to guarantee.
func (ctl *QueryController) dtoList(views []port.RequestView, transforme func(map[string]any) map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(views))
	for _, v := range views {
		out = append(out, transforme(ctl.requestViewDTO(v)))
	}
	return out
}

func identite(dto map[string]any) map[string]any { return dto }

// --- mes-demandes ------------------------------------------------------------

// MesDemandes handles GET /demandes/mes-demandes.
func (ctl *QueryController) MesDemandes(c *gin.Context) {
	appelant := port.CallerFromContext(c.Request.Context())
	views, fault := ctl.own.Execute(c.Request.Context(), appelant.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demandes récupérées avec succès",
		ctl.dtoList(views, identite)))
}

// --- a-accepter ---------------------------------------------------------------

// AAccepter handles GET /demandes/a-accepter.
func (ctl *QueryController) AAccepter(c *gin.Context) {
	appelant := port.CallerFromContext(c.Request.Context())
	views, fault := ctl.toAccept.Execute(c.Request.Context(), appelant.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demandes à accepter récupérées avec succès",
		ctl.dtoList(views, identite)))
}

// AAccepterDetail handles GET /demandes/a-accepter/:id.
func (ctl *QueryController) AAccepterDetail(c *gin.Context) {
	appelant := port.CallerFromContext(c.Request.Context())
	view, fault := ctl.toAccept.Detail(c.Request.Context(), c.Param("id"), appelant.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demande récupérée avec succès",
		ctl.requestViewDTO(view)))
}

// --- a-traiter ------------------------------------------------------------

// ATraiter handles GET /demandes/a-traiter.
func (ctl *QueryController) ATraiter(c *gin.Context) {
	appelant := port.CallerFromContext(c.Request.Context())
	views, fault := ctl.toProcess.Execute(c.Request.Context(), appelant.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demandes à traiter récupérées avec succès",
		ctl.dtoList(views, identite)))
}

// ATraiterDetail handles GET /demandes/a-traiter/:id.
func (ctl *QueryController) ATraiterDetail(c *gin.Context) {
	appelant := port.CallerFromContext(c.Request.Context())
	view, fault := ctl.toProcess.Detail(c.Request.Context(), c.Param("id"), appelant.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demande récupérée avec succès",
		ctl.requestViewDTO(view)))
}

// --- a-confirmer ------------------------------------------------------------

// AConfirmer handles GET /demandes/a-confirmer.
func (ctl *QueryController) AConfirmer(c *gin.Context) {
	appelant := port.CallerFromContext(c.Request.Context())
	views, fault := ctl.toConfirm.Execute(c.Request.Context(), appelant.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demandes à confirmer récupérées avec succès",
		ctl.dtoList(views, sansClient)))
}

// AConfirmerDetail handles GET /demandes/a-confirmer/:id.
func (ctl *QueryController) AConfirmerDetail(c *gin.Context) {
	appelant := port.CallerFromContext(c.Request.Context())
	view, fault := ctl.toConfirm.Detail(c.Request.Context(), c.Param("id"), appelant.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demande récupérée avec succès",
		sansClient(ctl.requestViewDTO(view))))
}

// --- deja-confirmees --------------------------------------------------------

// DejaConfirmees handles GET /demandes/deja-confirmees.
func (ctl *QueryController) DejaConfirmees(c *gin.Context) {
	appelant := port.CallerFromContext(c.Request.Context())
	views, fault := ctl.alreadyConfirmed.Execute(c.Request.Context(), appelant.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demandes déjà confirmées récupérées avec succès",
		ctl.dtoList(views, identite)))
}

// --- in / out -----------------------------------------------------------------

// In handles GET /demandes/in.
func (ctl *QueryController) In(c *gin.Context) {
	appelant := port.CallerFromContext(c.Request.Context())
	views, fault := ctl.incoming.Execute(c.Request.Context(), appelant.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demandes IN récupérées avec succès",
		ctl.dtoList(views, identite)))
}

// Out handles GET /demandes/out.
func (ctl *QueryController) Out(c *gin.Context) {
	appelant := port.CallerFromContext(c.Request.Context())
	views, fault := ctl.outgoing.Execute(c.Request.Context(), appelant.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demandes OUT récupérées avec succès",
		ctl.dtoList(views, identite)))
}
