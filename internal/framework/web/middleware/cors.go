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
// own comfort — letting the Swagger page, served on another port, call the
// API from a browser. It allows every origin by default; setting origins to
// empty makes it inert, restoring the real gateway's silence.
//
// It is deliberately written as a global middleware rather than as
// registered OPTIONS routes: constraint D-4 wants the sandbox to expose
// only the 33 contract routes, and an OPTIONS route per endpoint would
// double its surface.
func AllowCORS(origins []string) gin.HandlerFunc {
	allowAll := false
	for _, o := range origins {
		if o == "*" {
			allowAll = true
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" || len(origins) == 0 {
			c.Next()
			return
		}

		allowed := allowAll
		for _, o := range origins {
			if o == origin {
				allowed = true
				break
			}
		}
		if !allowed {
			// No header: the browser will block reading the response, which
			// is the expected behaviour for a disallowed origin.
			c.Next()
			return
		}

		value := origin
		if allowAll {
			value = "*"
		}
		c.Header("Access-Control-Allow-Origin", value)
		c.Header("Vary", "Origin")

		// Preflight: the browser sends it without an Authorization header.
		// It must be settled here, before the authentication middleware, or
		// it would leave in 401 and the real request would never be issued.
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", strings.Join([]string{
				http.MethodGet, http.MethodPost, http.MethodOptions,
			}, ", "))
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Max-Age", "600")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
