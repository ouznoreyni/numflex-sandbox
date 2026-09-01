package presenter

import (
	"net/http"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// Real renders the ARTP envelope and its problem+json error shape, mirroring
// internal/httpx.Renderer in config.FidelityReal mode.
type Real struct {
	clock port.Clock
}

// NewReal returns a Real presenter, its Rendered method backed by clock —
// the single seam through which CLOCK_SKEW_SECONDS is applied to any
// timestamp that leaves through this presenter.
func NewReal(clock port.Clock) *Real {
	return &Real{clock: clock}
}

func (p *Real) Success(status int, message string, data any) ViewModel {
	return ViewModel{Status: status, Body: Envelope{
		Success: true, Code: "SUCCESS", Message: message, Data: data,
	}}
}

// SuccessWithoutData mirrors Renderer.OKSansData in real mode: the data
// field is entirely omitted (ANO-011), never rendered as null.
func (p *Real) SuccessWithoutData(status int, message string) ViewModel {
	return ViewModel{Status: status, Body: EnvelopeSansData{
		Success: true, Code: "SUCCESS", Message: message,
	}}
}

// Failure mirrors Renderer.failReel: it reproduces ANO-001, ANO-003 and
// ANO-004 — no envelope, no code field, business errors surfacing as a 500,
// and the Java exception class name exposed via the "RuntimeException: "
// prefix (overridden by RealDetail for ANO-002 and ANO-020). path is
// rendered verbatim into Problem.Path, exactly as failReel reads
// c.Request.URL.Path into chemin (including "" when there is no request).
func (p *Real) Failure(f *entity.Fault, path string) ViewModel {
	if f == nil {
		f = entity.InternalError("erreur interne")
	}

	// FaultValidation with Fields: a real bean-validation violation from
	// Spring, which always carries at least one fieldError.
	if f.Kind == entity.FaultValidation && len(f.Fields) > 0 {
		return ViewModel{Status: http.StatusBadRequest, Body: Problem{
			Type:        "https://www.jhipster.tech/problem/constraint-violation",
			Title:       "Method argument not valid",
			Status:      http.StatusBadRequest,
			Path:        path,
			Message:     "error.validation",
			FieldErrors: f.Fields,
		}}
	}

	// FaultValidation without Fields: a business validation that failed
	// outside the bean-validation layer (e.g. FormatJSONInvalide,
	// FlotteVide, ValidationEchouee). A Spring/JHipster stack never answers
	// constraint-violation with zero fieldErrors; this body is an ordinary
	// problem-with-message in 400, and the business message stays readable
	// (no "RuntimeException: " prefix, reserved for 500s).
	if f.Kind == entity.FaultValidation {
		detail := f.RealDetail
		if detail == "" {
			detail = f.Message
		}
		return ViewModel{Status: http.StatusBadRequest, Body: Problem{
			Type:    "https://www.jhipster.tech/problem/problem-with-message",
			Title:   "Bad Request",
			Status:  http.StatusBadRequest,
			Detail:  detail,
			Path:    path,
			Message: "error.http.400",
		}}
	}

	detail := f.RealDetail
	if detail == "" {
		detail = "RuntimeException: " + f.Message
	}
	return ViewModel{Status: http.StatusInternalServerError, Body: Problem{
		Type:    "https://www.jhipster.tech/problem/problem-with-message",
		Title:   "Internal Server Error",
		Status:  http.StatusInternalServerError,
		Detail:  detail,
		Path:    path,
		Message: "error.http.500",
	}}
}

// Rendered applies the configured clock skew, as internal/httpx.Renderer.Skew
// did before it. It exists only when a timestamp leaves through this
// presenter; the value stored in the database is never touched.
func (p *Real) Rendered(t time.Time) time.Time {
	return p.clock.Rendered(t)
}
