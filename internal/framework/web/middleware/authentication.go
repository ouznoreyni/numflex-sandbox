package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/presenter"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/token"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// callerContextKey is the gin.Context key under which Authenticate stores
// the resolved entity.Caller, and CallerFrom reads it back.
const callerContextKey = "numflex.caller"

// Authenticate verifies a token — issuing one is a use case (Task 10), not
// this middleware's job. It reproduces the three behaviours of the legacy
// internal/api.Deps.Authentifier exactly:
//
//   - Header ABSENT, malformed, or an empty Bearer token → 401 with the ARTP
//     ACCES_INTERDIT envelope (the only code the real platform ever emits
//     for this case), regardless of fidelity mode.
//   - Token PRESENT but INVALID (bad signature, expired, wrong algorithm) →
//     401, empty body, no Content-Type (ANO-008) — again regardless of
//     fidelity mode: this branch never reaches the renderer.
//   - Token VALID but its subject no longer resolves to a user → rendered
//     through p with entity.OperatorNotFound(), exactly as
//     d.R.Fail(c, entity.OperatorNotFound()) did. This is the one outcome
//     that depends on fidelity: in real mode it is not a 401 at all — every
//     non-validation fault falls through to a 500 problem+json with the
//     "RuntimeException: " prefix — while contract mode answers with the
//     envelope carrying OPERATEUR_NON_TROUVE.
func Authenticate(secret string, users port.UserGateway, p presenter.Presenter) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") || strings.TrimSpace(header[7:]) == "" {
			f := entity.AccessForbidden()
			c.JSON(http.StatusUnauthorized, presenter.Envelope{
				Success: false, Code: f.Code, Message: f.Message, Data: nil,
			})
			c.Abort()
			return
		}

		username, err := token.Verify(secret, strings.TrimSpace(header[7:]))
		if err != nil {
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}

		caller, found, err := users.ByUsername(c, username)
		if err != nil || !found {
			render(c, p.Failure(entity.OperatorNotFound(), c.Request.URL.Path))
			return
		}

		c.Set(callerContextKey, caller)
		// Also carried on the request's context.Context, not just gin's own
		// store: an internal/adapter/controller cannot import this package
		// (test/architecture_test.go's dependency rule), but it can read
		// port.CallerFromContext(c.Request.Context()) — internal/usecase/port
		// is a layer both sides already depend on.
		c.Request = c.Request.WithContext(port.WithCaller(c.Request.Context(), caller))
		c.Next()
	}
}

// render writes a presenter.ViewModel to c, restoring the
// "Content-Type: application/problem+json" header that internal/httpx.Renderer
// set alongside a presenter.Problem body — the presenter package itself
// carries no framework dependency, so this middleware is where that header
// belongs until a controller layer takes over the job (Tasks 9 onward).
func render(c *gin.Context, vm presenter.ViewModel) {
	if _, ok := vm.Body.(presenter.Problem); ok {
		c.Header("Content-Type", "application/problem+json")
	}
	c.JSON(vm.Status, vm.Body)
	c.Abort()
}

// CallerFrom returns the entity.Caller resolved by Authenticate. Called on
// a request that never went through Authenticate, it returns the zero
// value.
func CallerFrom(c *gin.Context) entity.Caller {
	v, _ := c.Get(callerContextKey)
	caller, _ := v.(entity.Caller)
	return caller
}
