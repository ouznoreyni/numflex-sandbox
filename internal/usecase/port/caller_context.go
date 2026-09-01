package port

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
)

// callerContextKey is an unexported type so no other package can collide
// with this context.Context key.
type callerContextKey struct{}

// WithCaller returns ctx carrying caller. It is the seam through which the
// authentication middleware (internal/framework/web/middleware, an outer
// layer) hands a resolved entity.Caller down to a controller
// (internal/adapter, an inner layer) without the adapter importing
// framework: context.Context — unlike *gin.Context — is a type both layers
// already depend on, since it flows through every use case boundary's
// Execute. It lives in internal/usecase/port rather than internal/entity
// because it is transport plumbing for carrying a request-scoped value
// across the layer boundary the dependency rule itself creates, not domain
// vocabulary — entity.Caller's own invariants have nothing to do with it.
func WithCaller(ctx context.Context, caller entity.Caller) context.Context {
	return context.WithValue(ctx, callerContextKey{}, caller)
}

// CallerFromContext returns the entity.Caller stored by WithCaller, or the
// zero value if ctx carries none.
func CallerFromContext(ctx context.Context) entity.Caller {
	caller, _ := ctx.Value(callerContextKey{}).(entity.Caller)
	return caller
}
