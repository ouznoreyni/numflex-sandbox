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
// this middleware's job. It reproduces the two behaviours measured against
// the real platform: token ABSENT → 401 with the ARTP ACCES_INTERDIT
// envelope (the only code it ever emits); token PRESENT but invalid → 401,
// empty body, no Content-Type (ANO-008).
func Authenticate(secret string, users port.UserGateway) gin.HandlerFunc {
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
			// A token whose subject no longer resolves to a user is, from
			// the caller's point of view, indistinguishable from an invalid
			// one: the same empty-body 401, no envelope.
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}

		c.Set(callerContextKey, caller)
		c.Next()
	}
}

// CallerFrom returns the entity.Caller resolved by Authenticate. Called on
// a request that never went through Authenticate, it returns the zero
// value.
func CallerFrom(c *gin.Context) entity.Caller {
	v, _ := c.Get(callerContextKey)
	caller, _ := v.(entity.Caller)
	return caller
}
