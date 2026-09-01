package port

import "context"

// Repositories is the set of gateways available inside one transaction.
// Gateways obtained here are bound to that transaction; gateways injected
// directly into an interactor are not.
type Repositories struct {
	OTP OTPGateway
	// Requests carries the writes of request creation — RoutingPrefix,
	// Create, AddNumber, AddExcludedNumber, AddClient — bound to the same
	// transaction as OTP, so a failed request insert leaves OTP.Consume
	// uncalled and the challenge unconsumed (the guarantee established at
	// commit 643415f). NumberGateway is deliberately absent here: every
	// number-state read a creation interactor needs happens before the
	// transaction opens, against the plain pool.
	Requests RequestGateway
	// Confirmations carries the single write Task 15's ConfirmRequest makes
	// — one confirmation row, bound to the same transaction boundary as
	// every other capability's writes here, though today it coordinates
	// with nothing else inside the same Do.
	Confirmations ConfirmationGateway
	// Reverse, Incidents… are added by later tasks.
}

// UnitOfWork owns the transaction boundary. The interactor decides that there
// is a transaction; the adapter decides what a transaction is. No pgx.Tx ever
// reaches this layer.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(Repositories) error) error
}
