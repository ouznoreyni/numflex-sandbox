// Package middleware holds the sandbox's global Gin middlewares:
// authentication (verifying a token, resolving the caller it names) and
// CORS.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AllowCORS does not belong to the ARTP contract: the real gateway is
// consumed server-to-server, and no SIT test ever measured its behaviour on
// a cross-origin request. This middleware exists purely for the sandbox's
// own comfort — letting a page served from anywhere else, the Swagger one
// or a back-office in development, call the API from a browser. Every
// origin is allowed, with nothing to configure: a local double holds
// nothing worth protecting from a cross-origin read, and one variable fewer
// is one less reason for a browser call to fail unexplained.
//
// The header goes out only when the request carries an Origin, which is to
// say only to a browser: a server-to-server call still receives exactly the
// three headers the platform sends, and the fidelity of those responses is
// untouched.
//
// It is deliberately written as a global middleware rather than as
// registered OPTIONS routes: constraint D-4 wants the sandbox to expose
// only the 33 contract routes, and an OPTIONS route per endpoint would
// double its surface.
func AllowCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("Origin") == "" {
			c.Next()
			return
		}

		c.Header("Access-Control-Allow-Origin", "*")

		// Preflight: the browser sends it without an Authorization header.
		// It must be settled here, before the authentication middleware, or
		// it would leave in 401 and the real request would never be issued.
		// DELETE is in the list for the sandbox purge, the one route of the
		// surface that uses it.
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", strings.Join([]string{
				http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodOptions,
			}, ", "))
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Max-Age", "600")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
