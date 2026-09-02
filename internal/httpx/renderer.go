package httpx

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
)

type Renderer struct {
	fid  config.Fidelity
	skew time.Duration
}

func NewRenderer(fid config.Fidelity, skew time.Duration) *Renderer {
	return &Renderer{fid: fid, skew: skew}
}

func (r *Renderer) Fidelity() config.Fidelity { return r.fid }

// Skew applies the server clock drift measured in the ARTP staging environment
// (ANO-015, ~9 min) and truncates to the millisecond. It touches only rendered
// timestamps; those stored in the database stay accurate, at their full
// precision.
//
// The truncation comes from the captures of 2026-08-27: the platform reads its
// documents back from Mongo, whose timestamps are millisecond-precise, and so
// renders « 2026-08-27T22:39:23.583Z ». Postgres stores microseconds; without
// truncation the sandbox would render a field more precise than the original,
// and a client comparing timestamps on their exact format would see the
// difference.
func (r *Renderer) Skew(t time.Time) time.Time {
	return t.Add(r.skew).Truncate(time.Millisecond)
}

func (r *Renderer) OK(c *gin.Context, status int, message string, data any) {
	c.JSON(status, Envelope{Success: true, Code: "SUCCESS", Message: message, Data: data})
}

func (r *Renderer) OKSansData(c *gin.Context, status int, message string) {
	if r.fid == config.FidelityReal {
		c.JSON(status, EnvelopeSansData{Success: true, Code: "SUCCESS", Message: message})
		return
	}
	c.JSON(status, Envelope{Success: true, Code: "SUCCESS", Message: message, Data: nil})
}

type problem struct {
	Type        string              `json:"type"`
	Title       string              `json:"title"`
	Status      int                 `json:"status"`
	Detail      string              `json:"detail,omitempty"`
	Path        string              `json:"path"`
	Message     string              `json:"message"`
	FieldErrors []entity.FieldFault `json:"fieldErrors,omitempty"`
}

func (r *Renderer) Fail(c *gin.Context, err error) {
	e := entity.FaultFrom(err)
	if r.fid == config.FidelityContract {
		r.failContrat(c, e)
		return
	}
	r.failReel(c, e)
}

// failReel reproduces ANO-001, ANO-003 and ANO-004: no envelope, no code field,
// business errors as 500s, and the Java exception class name exposed.
func (r *Renderer) failReel(c *gin.Context, e *entity.Fault) {
	chemin := ""
	if c.Request != nil {
		chemin = c.Request.URL.Path
	}

	// KindValidation with Fields: a real Spring bean validation violation, which
	// always carries at least one fieldError.
	if e.Kind == entity.FaultValidation && len(e.Fields) > 0 {
		c.Header("Content-Type", "application/problem+json")
		c.JSON(http.StatusBadRequest, problem{
			Type:        "https://www.jhipster.tech/problem/constraint-violation",
			Title:       "Method argument not valid",
			Status:      http.StatusBadRequest,
			Path:        chemin,
			Message:     "error.validation",
			FieldErrors: e.Fields,
		})
		c.Abort()
		return
	}

	// KindValidation without Fields: a business validation that failed outside the
	// bean validation layer (FormatJSONInvalide, FlotteVide, ValidationEchouee, say).
	// A Spring/JHipster stack never answers constraint-violation with zero
	// fieldErrors; that body is an ordinary problem-with-message in 400, and the
	// business message stays readable (no RuntimeException: prefix, which is
	// reserved for 500s).
	if e.Kind == entity.FaultValidation {
		detail := e.RealDetail
		if detail == "" {
			detail = e.Message
		}
		c.Header("Content-Type", "application/problem+json")
		c.JSON(http.StatusBadRequest, problem{
			Type:    "https://www.jhipster.tech/problem/problem-with-message",
			Title:   "Bad Request",
			Status:  http.StatusBadRequest,
			Detail:  detail,
			Path:    chemin,
			Message: "error.http.400",
		})
		c.Abort()
		return
	}

	detail := e.RealDetail
	if detail == "" {
		detail = "RuntimeException: " + e.Message
	}
	c.Header("Content-Type", "application/problem+json")
	c.JSON(http.StatusInternalServerError, problem{
		Type:    "https://www.jhipster.tech/problem/problem-with-message",
		Title:   "Internal Server Error",
		Status:  http.StatusInternalServerError,
		Detail:  detail,
		Path:    chemin,
		Message: "error.http.500",
	})
	c.Abort()
}

func (r *Renderer) failContrat(c *gin.Context, e *entity.Fault) {
	c.JSON(statutContrat(e.Kind), Envelope{
		Success: false,
		Code:    e.Code,
		Message: e.Message,
		Data:    nil,
	})
	c.Abort()
}

func statutContrat(k entity.FaultKind) int {
	switch k {
	case entity.FaultValidation:
		return http.StatusBadRequest
	case entity.FaultNotFound:
		return http.StatusNotFound
	case entity.FaultAccess:
		return http.StatusForbidden
	case entity.FaultState:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
