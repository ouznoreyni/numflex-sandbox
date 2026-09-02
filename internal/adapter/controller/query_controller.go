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
// port.RequestView for a detail route) into the guide §7.3 shape, via the
// package-level requestViewDTO and sansClient (request_view.go) — the one
// shared implementation Task 15 consolidated ruling R28's deliberate
// duplicates into.
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

// dtoList maps a slice of views onto their DTOs. It never returns nil: even
// an empty views slice yields a non-nil, zero-length slice, so the caller's
// JSON "data" field renders as [] — never null — exactly as
// Deps.rendreListe was written to guarantee.
func (ctl *QueryController) dtoList(views []port.RequestView, transform func(map[string]any) map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(views))
	for _, v := range views {
		out = append(out, transform(requestViewDTO(ctl.clock, v)))
	}
	return out
}

func identity(dto map[string]any) map[string]any { return dto }

// --- mes-demandes ------------------------------------------------------------

// Own handles GET /demandes/mes-demandes.
func (ctl *QueryController) Own(c *gin.Context) {
	caller := port.CallerFromContext(c.Request.Context())
	views, fault := ctl.own.Execute(c.Request.Context(), caller.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demandes récupérées avec succès",
		ctl.dtoList(views, identity)))
}

// --- a-accepter ---------------------------------------------------------------

// ToAccept handles GET /demandes/a-accepter.
func (ctl *QueryController) ToAccept(c *gin.Context) {
	caller := port.CallerFromContext(c.Request.Context())
	views, fault := ctl.toAccept.Execute(c.Request.Context(), caller.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demandes à accepter récupérées avec succès",
		ctl.dtoList(views, identity)))
}

// ToAcceptDetail handles GET /demandes/a-accepter/:id.
func (ctl *QueryController) ToAcceptDetail(c *gin.Context) {
	caller := port.CallerFromContext(c.Request.Context())
	view, fault := ctl.toAccept.Detail(c.Request.Context(), c.Param("id"), caller.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demande récupérée avec succès",
		requestViewDTO(ctl.clock, view)))
}

// --- a-traiter ------------------------------------------------------------

// ToProcess handles GET /demandes/a-traiter.
func (ctl *QueryController) ToProcess(c *gin.Context) {
	caller := port.CallerFromContext(c.Request.Context())
	views, fault := ctl.toProcess.Execute(c.Request.Context(), caller.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demandes à traiter récupérées avec succès",
		ctl.dtoList(views, identity)))
}

// ToProcessDetail handles GET /demandes/a-traiter/:id.
func (ctl *QueryController) ToProcessDetail(c *gin.Context) {
	caller := port.CallerFromContext(c.Request.Context())
	view, fault := ctl.toProcess.Detail(c.Request.Context(), c.Param("id"), caller.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demande récupérée avec succès",
		requestViewDTO(ctl.clock, view)))
}

// --- a-confirmer ------------------------------------------------------------

// ToConfirm handles GET /demandes/a-confirmer.
func (ctl *QueryController) ToConfirm(c *gin.Context) {
	caller := port.CallerFromContext(c.Request.Context())
	views, fault := ctl.toConfirm.Execute(c.Request.Context(), caller.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demandes à confirmer récupérées avec succès",
		ctl.dtoList(views, sansClient)))
}

// ToConfirmDetail handles GET /demandes/a-confirmer/:id.
func (ctl *QueryController) ToConfirmDetail(c *gin.Context) {
	caller := port.CallerFromContext(c.Request.Context())
	view, fault := ctl.toConfirm.Detail(c.Request.Context(), c.Param("id"), caller.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demande récupérée avec succès",
		sansClient(requestViewDTO(ctl.clock, view))))
}

// --- deja-confirmees --------------------------------------------------------

// AlreadyConfirmed handles GET /demandes/deja-confirmees.
func (ctl *QueryController) AlreadyConfirmed(c *gin.Context) {
	caller := port.CallerFromContext(c.Request.Context())
	views, fault := ctl.alreadyConfirmed.Execute(c.Request.Context(), caller.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demandes déjà confirmées récupérées avec succès",
		ctl.dtoList(views, identity)))
}

// --- in / out -----------------------------------------------------------------

// In handles GET /demandes/in.
func (ctl *QueryController) In(c *gin.Context) {
	caller := port.CallerFromContext(c.Request.Context())
	views, fault := ctl.incoming.Execute(c.Request.Context(), caller.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demandes IN récupérées avec succès",
		ctl.dtoList(views, identity)))
}

// Out handles GET /demandes/out.
func (ctl *QueryController) Out(c *gin.Context) {
	caller := port.CallerFromContext(c.Request.Context())
	views, fault := ctl.outgoing.Execute(c.Request.Context(), caller.OperatorID)
	if fault != nil {
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}
	render(c, ctl.pres.Success(http.StatusOK, "Demandes OUT récupérées avec succès",
		ctl.dtoList(views, identity)))
}
