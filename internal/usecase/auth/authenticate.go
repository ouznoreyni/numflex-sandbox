package auth

import (
	"context"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// AuthenticateInput carries the credentials presented to POST /api/authenticate.
type AuthenticateInput struct {
	Username string
	Password string
}

// AuthenticateOutput carries the issued token.
type AuthenticateOutput struct {
	Token string
}

// AuthenticateBoundary is the interface a controller drives; it exists so
// the controller can depend on the use case's contract, not its struct.
type AuthenticateBoundary interface {
	Execute(context.Context, AuthenticateInput) (AuthenticateOutput, *entity.Fault)
}

// TokenIssuer signs a token for username, carrying roles as its claim — the
// one seam through which AuthenticateInteractor reaches a concrete token
// technology (internal/framework/token.Issue in production), without
// importing it directly.
type TokenIssuer func(username string, roles []string) (string, error)

// AuthenticateInteractor implements AuthenticateBoundary.
type AuthenticateInteractor struct {
	users port.UserGateway
	issue TokenIssuer
	// ttl is accepted for parity with the composition root, which also
	// threads config.JWTTTL into the TokenIssuer closure it builds — the
	// actual expiry is entirely that closure's business; Execute never reads
	// this field itself, exactly as the legacy demandeAuth.RememberMe field
	// was parsed but never acted upon.
	ttl time.Duration
}

// NewAuthenticate wires an interactor against the given gateway and token
// issuer, with the configured token time-to-live.
func NewAuthenticate(users port.UserGateway, issue TokenIssuer, ttl time.Duration) *AuthenticateInteractor {
	return &AuthenticateInteractor{users: users, issue: issue, ttl: ttl}
}

// Execute resolves the caller by credentials, then issues a token for it.
// A bad login — unknown username, or a password that does not match — and a
// gateway failure are both reported as *entity.Fault, never a Go error
// leaking through: BadCredentials for the former (ACCES_INTERDIT's sibling,
// rendered outside the envelope by the controller — see entity.BadCredentials),
// InternalError for the latter and for a token-issuance failure.
func (i *AuthenticateInteractor) Execute(ctx context.Context, in AuthenticateInput) (AuthenticateOutput, *entity.Fault) {
	caller, found, err := i.users.ByCredentials(ctx, in.Username, in.Password)
	if err != nil {
		return AuthenticateOutput{}, entity.InternalError("authentication failed")
	}
	if !found {
		return AuthenticateOutput{}, entity.BadCredentials()
	}

	token, err := i.issue(caller.Username, caller.Roles)
	if err != nil {
		return AuthenticateOutput{}, entity.InternalError("token issuance failed")
	}
	return AuthenticateOutput{Token: token}, nil
}
