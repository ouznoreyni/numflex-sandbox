package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// TrackWrittenTimestamps gives every request its registry of the timestamps
// it writes, so that its response renders them to the nanosecond as the
// platform does (see port.WrittenAt). Installed before authentication: the
// caller that middleware.Authenticate adds to the context derives from this
// one and keeps the registry.
func TrackWrittenTimestamps() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(port.WithWrittenTimestamps(c.Request.Context()))
		c.Next()
	}
}
