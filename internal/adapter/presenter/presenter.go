package presenter

import "github.com/ouznoreyni/numflex-sandbox/internal/entity"

// Envelope is the success envelope of guide §8, identical in both fidelity
// modes.
type Envelope struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// EnvelopeSansData serves ANO-011: the otp/send response omits the data
// field entirely — it is neither present nor null.
type EnvelopeSansData struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Problem is the JHipster problem+json shape used by Real's error rendering.
//
// NOTE on Path: internal/httpx.Renderer.failReel fills Path from the live
// *gin.Context (c.Request.URL.Path). Presenter.Failure takes only a
// *entity.Fault — it has no HTTP request in scope — so Real renders Path
// present but empty for now. Threading the real request path through is
// deferred to the task that wires internal/api onto this package (Task 18).
type Problem struct {
	Type        string              `json:"type"`
	Title       string              `json:"title"`
	Status      int                 `json:"status"`
	Detail      string              `json:"detail,omitempty"`
	Path        string              `json:"path"`
	Message     string              `json:"message"`
	FieldErrors []entity.FieldFault `json:"fieldErrors,omitempty"`
}

// Presenter renders a use case's outcome into a ViewModel. Real and Contract
// are its two implementations, one per fidelity mode of the guide.
type Presenter interface {
	Success(status int, message string, data any) ViewModel
	SuccessWithoutData(status int, message string) ViewModel
	Failure(f *entity.Fault) ViewModel
}

var _ Presenter = (*Real)(nil)
var _ Presenter = (*Contract)(nil)
