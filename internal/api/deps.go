package api

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/httpx"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

type Deps struct {
	Cfg *config.Config
	DB  *persistence.DB
	R   *httpx.Renderer
	// Moteur is filled in by Task 9. Declared here as an interface so that the
	// api package does not depend on the order the tasks land in.
	Moteur Moteur
}

// Moteur — the part of the platform's behaviour that calls do not drive.
type Moteur interface {
	PlaceGelee(ctx context.Context) (bool, error)
	PlanifierTransition(ctx context.Context, demandeID string) error
}

// Appelant reads the Caller that middleware.Authenticate (Task 6, wired in
// Task 10) resolved and put on the request's context — not on *gin.Context's
// own store, as the former Authentifier did: port.CallerFromContext is the
// shared crossing point between the middleware (internal/framework) and this
// package, which is not bound by test/architecture_test.go's dependency rule
// (being neither adapter nor framework) and may therefore read what the
// middleware left there.
func Appelant(c *gin.Context) entity.Caller {
	return port.CallerFromContext(c.Request.Context())
}
