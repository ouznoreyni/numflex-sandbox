package entity

import "context"

// callerContextKey is an unexported type so no other package can collide
// with this context.Context key.
type callerContextKey struct{}

// WithCaller returns ctx carrying caller. It is the seam through which the
// authentication middleware (internal/framework/web/middleware, an outer
// layer) hands a resolved Caller down to a controller (internal/adapter, an
// inner layer) without the adapter importing framework:
// context.Context — unlike *gin.Context — is a type both layers already
// depend on, since it flows through every use case boundary's Execute.
func WithCaller(ctx context.Context, caller Caller) context.Context {
	return context.WithValue(ctx, callerContextKey{}, caller)
}

// CallerFromContext returns the Caller stored by WithCaller, or the zero
// value if ctx carries none.
func CallerFromContext(ctx context.Context) Caller {
	caller, _ := ctx.Value(callerContextKey{}).(Caller)
	return caller
}
