package controller

import (
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/presenter"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/otp"
)

// msisdnPattern is the ARTP MSISDN shape, moved unchanged from the deleted
// internal/api/otp.go.
var msisdnPattern = regexp.MustCompile(`^[0-9]{9}$`)

// OTPController is the interface-adapter for the two OTP routes.
type OTPController struct {
	send   otp.SendOTPBoundary
	verify otp.VerifyOTPBoundary
	pres   presenter.Presenter
}

// NewOTPController wires a controller against the two OTP boundaries and a
// presenter — the presenter chosen by the caller according to the
// configured fidelity mode.
func NewOTPController(send otp.SendOTPBoundary, verify otp.VerifyOTPBoundary, p presenter.Presenter) *OTPController {
	return &OTPController{send: send, verify: verify, pres: p}
}

type otpSendRequest struct {
	MSISDN string `json:"numero"`
}

type otpVerifyRequest struct {
	MSISDN  string `json:"numero"`
	OtpCode string `json:"otpCode"`
}

// render writes a presenter.ViewModel to the response — the controller's
// counterpart of internal/httpx.Renderer writing straight to *gin.Context.
func render(c *gin.Context, vm presenter.ViewModel) {
	c.JSON(vm.Status, vm.Body)
}

// Send binds the request, validates the MSISDN shape — the one rule that is
// not the interactor's business — then delegates to SendOTPBoundary.
func (ctl *OTPController) Send(c *gin.Context) {
	var req otpSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		render(c, ctl.pres.Failure(entity.InvalidJSONFormat(), c.Request.URL.Path))
		return
	}
	if !msisdnPattern.MatchString(req.MSISDN) {
		render(c, ctl.pres.Failure(entity.Validation(entity.FieldFault{
			ObjectName: "otpSendDTO", Field: "numero",
			Message: `doit correspondre à "^[0-9]{9}$"`,
		}), c.Request.URL.Path))
		return
	}

	if err := ctl.send.Execute(c.Request.Context(), otp.SendOTPInput{MSISDN: req.MSISDN}); err != nil {
		render(c, ctl.pres.Failure(entity.FaultFrom(err), c.Request.URL.Path))
		return
	}

	// The sandbox does not send an SMS: the code is static and logged. The
	// response acknowledges the submission, not delivery (ANO-021).
	render(c, ctl.pres.SuccessWithoutData(http.StatusOK, "OTP envoyé avec succès"))
}

// Verify binds the request and delegates to VerifyOTPBoundary, which owns
// every business rule (TC-021: pre-verifies without consuming).
func (ctl *OTPController) Verify(c *gin.Context) {
	var req otpVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		render(c, ctl.pres.Failure(entity.InvalidJSONFormat(), c.Request.URL.Path))
		return
	}

	if f := ctl.verify.Execute(c.Request.Context(), otp.VerifyOTPInput{
		MSISDN: req.MSISDN, Code: req.OtpCode,
	}); f != nil {
		render(c, ctl.pres.Failure(f, c.Request.URL.Path))
		return
	}

	render(c, ctl.pres.SuccessWithoutData(http.StatusOK, "Code OTP vérifié avec succès"))
}
