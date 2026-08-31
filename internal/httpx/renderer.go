package httpx

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yas/numflex-sandbox/internal/apperr"
	"github.com/yas/numflex-sandbox/internal/config"
)

type Renderer struct {
	fid  config.Fidelity
	skew time.Duration
}

func NewRenderer(fid config.Fidelity, skew time.Duration) *Renderer {
	return &Renderer{fid: fid, skew: skew}
}

func (r *Renderer) Fidelity() config.Fidelity { return r.fid }

// Skew applique la dérive d'horloge serveur mesurée en recette (ANO-015, ~9 min).
// Elle ne touche que les horodatages rendus ; ceux stockés en base restent justes.
func (r *Renderer) Skew(t time.Time) time.Time { return t.Add(r.skew) }

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
	FieldErrors []apperr.FieldError `json:"fieldErrors,omitempty"`
}

func (r *Renderer) Fail(c *gin.Context, err error) {
	var e *apperr.Error
	if !errors.As(err, &e) {
		e = apperr.ErreurInterne(err.Error())
	}
	if r.fid == config.FidelityContract {
		r.failContrat(c, e)
		return
	}
	r.failReel(c, e)
}

// failReel reproduit ANO-001, ANO-003 et ANO-004 : aucune enveloppe, aucun champ
// code, les erreurs métier en 500, et le nom de la classe d'exception Java exposé.
func (r *Renderer) failReel(c *gin.Context, e *apperr.Error) {
	chemin := ""
	if c.Request != nil {
		chemin = c.Request.URL.Path
	}

	if e.Kind == apperr.KindValidation {
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

func (r *Renderer) failContrat(c *gin.Context, e *apperr.Error) {
	c.JSON(statutContrat(e.Kind), Envelope{
		Success: false,
		Code:    e.Code,
		Message: e.Message,
		Data:    nil,
	})
	c.Abort()
}

func statutContrat(k apperr.Kind) int {
	switch k {
	case apperr.KindValidation:
		return http.StatusBadRequest
	case apperr.KindIntrouvable:
		return http.StatusNotFound
	case apperr.KindAcces:
		return http.StatusForbidden
	case apperr.KindEtat:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
