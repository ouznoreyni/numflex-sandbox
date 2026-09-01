package port

import "context"

// Repositories is the set of gateways available inside one transaction.
// Gateways obtained here are bound to that transaction; gateways injected
// directly into an interactor are not.
type Repositories struct {
	OTP OTPGateway
	// Requests, Numbers, Reverse, Incidents… are added by later tasks.
}

// UnitOfWork owns the transaction boundary. The interactor decides that there
// is a transaction; the adapter decides what a transaction is. No pgx.Tx ever
// reaches this layer.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(Repositories) error) error
}
