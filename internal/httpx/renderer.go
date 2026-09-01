package httpx

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ouznoreyni/numflex-sandbox/internal/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
)

type Renderer struct {
	fid  config.Fidelity
	skew time.Duration
}

func NewRenderer(fid config.Fidelity, skew time.Duration) *Renderer {
	return &Renderer{fid: fid, skew: skew}
}

func (r *Renderer) Fidelity() config.Fidelity { return r.fid }

// Skew applique la dérive d'horloge serveur mesurée en recette (ANO-015, ~9 min),
// et tronque à la milliseconde. Elle ne touche que les horodatages rendus ; ceux
// stockés en base restent justes et à leur pleine précision.
//
// La troncature vient des captures du 2026-08-27 : la plateforme relit ses
// documents depuis Mongo, dont l'horodatage est milliseconde, et rend donc
// « 2026-08-27T22:39:23.583Z ». Postgres stocke à la microseconde ; sans
// troncature le sandbox rendrait un champ plus précis que l'original, et un
// client qui compare des horodatages au format exact verrait la différence.
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

// failReel reproduit ANO-001, ANO-003 et ANO-004 : aucune enveloppe, aucun champ
// code, les erreurs métier en 500, et le nom de la classe d'exception Java exposé.
func (r *Renderer) failReel(c *gin.Context, e *entity.Fault) {
	chemin := ""
	if c.Request != nil {
		chemin = c.Request.URL.Path
	}

	// KindValidation avec Fields : une vraie violation de bean validation Spring,
	// qui porte toujours au moins un fieldError.
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

	// KindValidation sans Fields : une validation métier échouée en dehors de la
	// couche bean validation (ex. FormatJSONInvalide, FlotteVide, ValidationEchouee).
	// Une pile Spring/JHipster ne répond jamais constraint-violation avec zéro
	// fieldErrors ; ce corps-là est un problem-with-message ordinaire en 400, et le
	// message métier reste lisible (pas de préfixe RuntimeException:, réservé aux 500).
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
