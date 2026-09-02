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
// Path is filled by Real.Failure from the path argument the caller passes
// it — the future controller's equivalent of httpx.Renderer.failReel reading
// c.Request.URL.Path.
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
//
// Failure takes the request path alongside the fault: Real's problem+json
// body carries it (mirroring httpx.Renderer.failReel, which reads
// c.Request.URL.Path); Contract's envelope has no such field and ignores it,
// but the interface stays common to both so a controller need not know which
// fidelity it is talking to.
type Presenter interface {
	Success(status int, message string, data any) ViewModel
	SuccessWithoutData(status int, message string) ViewModel
	Failure(f *entity.Fault, path string) ViewModel
}

var _ Presenter = (*Real)(nil)
var _ Presenter = (*Contract)(nil)
