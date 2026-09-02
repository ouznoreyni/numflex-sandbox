package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/presenter"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/auth"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// AuthController is the interface-adapter for the two /api/authenticate
// routes: issuing a token (POST) and confirming one is still valid (GET —
// the authentication middleware, wired ahead of it on that route, is what
// actually rejects an invalid token; by the time this controller runs there
// is nothing left to check).
type AuthController struct {
	authenticate auth.AuthenticateBoundary
	describe     auth.DescribeCallerBoundary
	pres         presenter.Presenter
}

// NewAuthController wires a controller against the two auth boundaries and a
// presenter, exactly as NewOTPController does.
func NewAuthController(authenticate auth.AuthenticateBoundary, describe auth.DescribeCallerBoundary, p presenter.Presenter) *AuthController {
	return &AuthController{authenticate: authenticate, describe: describe, pres: p}
}

type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	// RememberMe is bound, like the legacy demandeAuth, but never read: the
	// sandbox issues a single fixed-TTL token regardless of its value.
	RememberMe bool `json:"rememberMe"`
}

// PostAuthenticate binds the login request and delegates to
// AuthenticateBoundary. A bad login (entity.BadCredentials) is rendered
// outside the presenter entirely, in a fixed JHipster problem+json body —
// ANO-016 — moved unchanged from the deleted internal/api/auth.go: it is
// the one outcome the real platform never varies by fidelity mode.
func (ctl *AuthController) PostAuthenticate(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		render(c, ctl.pres.Failure(entity.InvalidJSONFormat(), c.Request.URL.Path))
		return
	}

	out, fault := ctl.authenticate.Execute(c.Request.Context(), auth.AuthenticateInput{
		Username: req.Username, Password: req.Password,
	})
	if fault != nil {
		if fault.Code == entity.BadCredentials().Code {
			c.Header("Content-Type", "application/problem+json")
			c.JSON(http.StatusUnauthorized, gin.H{
				"type":    "https://www.jhipster.tech/problem/problem-with-message",
				"title":   "Unauthorized",
				"status":  http.StatusUnauthorized,
				"detail":  "Bad credentials",
				"path":    c.Request.URL.Path,
				"message": "error.http.401",
			})
			c.Abort()
			return
		}
		render(c, ctl.pres.Failure(fault, c.Request.URL.Path))
		return
	}

	c.JSON(http.StatusOK, gin.H{"id_token": out.Token})
}

// GetAuthenticate confirms authentication — 204 No Content, exactly as the
// legacy handler answered once middleware.Authenticate (registered ahead of
// this route in internal/api/router.go) had already let the request
// through. port.CallerFromContext reads the caller that middleware
// resolved: DescribeCaller's outcome never changes the response, but running
// it keeps this route driving a use case rather than a bare status write.
func (ctl *AuthController) GetAuthenticate(c *gin.Context) {
	ctl.describe.Execute(c.Request.Context(), auth.DescribeCallerInput{
		Caller: port.CallerFromContext(c.Request.Context()),
	})
	c.Status(http.StatusNoContent)
}
