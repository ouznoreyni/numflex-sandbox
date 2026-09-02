package web

import (
	"github.com/gin-gonic/gin"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
)

// NewEngine builds the sandbox's Gin engine: release mode and panic
// recovery, with the caller-supplied middleware layered on top, in the
// order given. cfg is accepted so that callers already holding it need not
// thread engine-wide settings through some other seam as later tasks add
// them; NewEngine itself reads nothing from it today.
func NewEngine(cfg *config.Config, mw ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	for _, m := range mw {
		r.Use(m)
	}
	return r
}
