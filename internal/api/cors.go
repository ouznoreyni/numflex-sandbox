package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS is no part of the ARTP contract: the real gateway is consumed server to
// server, and no SIT test measured its behaviour on a cross-origin request.
// This middleware exists only for the sandbox's comfort — letting the Swagger
// page, served on another port, call the API from a browser. It allows every
// origin by default; setting CORS_ALLOWED_ORIGINS to empty makes it inert, and
// gives the sandbox back the real gateway's silence.
//
// It is deliberately written as a global middleware rather than as registered
// OPTIONS routes: constraint D-4 wants the sandbox to expose the 33 contract
// routes and nothing else, and one OPTIONS route per endpoint would double that
// surface.
func (d *Deps) autoriserCORS() gin.HandlerFunc {
	origines := d.Cfg.CORSAllowedOrigins
	toutes := false
	for _, o := range origines {
		if o == "*" {
			toutes = true
		}
	}

	return func(c *gin.Context) {
		origine := c.GetHeader("Origin")
		if origine == "" || len(origines) == 0 {
			c.Next()
			return
		}

		autorisee := toutes
		for _, o := range origines {
			if o == origine {
				autorisee = true
				break
			}
		}
		if !autorisee {
			// No header at all: the browser will block reading the response,
			// which is what an unauthorised origin is meant to get.
			c.Next()
			return
		}

		valeur := origine
		if toutes {
			valeur = "*"
		}
		c.Header("Access-Control-Allow-Origin", valeur)
		c.Header("Vary", "Origin")

		// Preflight: the browser sends it without an Authorization header. It must
		// be settled here, before the authentication middleware, or it comes back
		// 401 and the real request is never sent.
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
