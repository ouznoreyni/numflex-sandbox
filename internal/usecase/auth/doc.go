// Package auth holds the authentication use cases: issuing a token for a
// login (AuthenticateInteractor) and confirming an already-resolved caller
// (DescribeCallerInteractor, the GET /api/authenticate probe).
//
// May import: the standard library, internal/entity and
// internal/usecase/port.
//
// Must never know: pgx, Gin, and — critically — any JWT library. Issuing a
// token is delegated to an injected TokenIssuer, so this layer stays
// ignorant of the concrete token technology; internal/framework/token is
// the only package that knows it.
package auth
