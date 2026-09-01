package web

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// GatewayPrefix is the root of the 33 routes of the ARTP contract.
const GatewayPrefix = "/api/gateway/v1"

// GuardGatewayPrefix wires authenticate as a global middleware, guarded by
// prefix, rather than as a group middleware: Gin only runs a group's
// middleware for a route actually registered in that group, but a
// non-contract path under GatewayPrefix must answer the same 401 as a valid
// path — exactly like a Spring Security filter, which runs before route
// resolution rather than after it. The comparison is on whole segments —
// equal to GatewayPrefix, or GatewayPrefix followed by "/" — so that a
// neighbouring path like /api/gateway/v1extra, or a future
// /api/gateway/v10, is not captured: a Spring Security filter is declared
// "/api/gateway/v1/**", which requires the slash and would never match
// those paths either.
func GuardGatewayPrefix(authenticate gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path != GatewayPrefix && !strings.HasPrefix(path, GatewayPrefix+"/") {
			c.Next()
			return
		}
		authenticate(c)
	}
}
