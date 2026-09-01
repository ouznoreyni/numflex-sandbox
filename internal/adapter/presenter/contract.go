package presenter

import (
	"net/http"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// Contract renders JHipster problem+json in error cases and the ARTP
// success envelope unchanged, mirroring internal/httpx.Renderer in
// config.FidelityContract mode.
type Contract struct {
	clock port.Clock
}

// NewContract returns a Contract presenter, its Rendered method backed by
// clock — see Real.Rendered.
func NewContract(clock port.Clock) *Contract {
	return &Contract{clock: clock}
}

func (p *Contract) Success(status int, message string, data any) ViewModel {
	return ViewModel{Status: status, Body: Envelope{
		Success: true, Code: "SUCCESS", Message: message, Data: data,
	}}
}

func (p *Contract) SuccessWithoutData(status int, message string) ViewModel {
	return ViewModel{Status: status, Body: Envelope{
		Success: true, Code: "SUCCESS", Message: message, Data: nil,
	}}
}

// Failure mirrors Renderer.failContrat. path is accepted to satisfy
// Presenter but unused: the contract envelope, unlike Real's problem+json,
// has no path field (mirroring failContrat, which never reads
// c.Request.URL.Path).
func (p *Contract) Failure(f *entity.Fault, _ string) ViewModel {
	if f == nil {
		f = entity.InternalError("erreur interne")
	}
	return ViewModel{Status: statutContrat(f.Kind), Body: Envelope{
		Success: false,
		Code:    f.Code,
		Message: f.Message,
		Data:    nil,
	}}
}

// Rendered — see Real.Rendered.
func (p *Contract) Rendered(t time.Time) time.Time {
	return p.clock.Rendered(t)
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
