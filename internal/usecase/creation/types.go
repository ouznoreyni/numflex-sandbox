// Package creation holds the three request-creation use cases behind
// POST /demandes/particulier, POST /demandes/entreprise and
// POST /demandes/restitution — the largest capability migrated so far, and
// the one carrying the sandbox's one load-bearing transaction: a request
// insert and the OTP consumption it depends on must commit or fail
// together (commit 643415f). Each interactor here orchestrates
// port.NumberGateway (read before the transaction), the otp package's
// VerifyOTPBoundary (also read-only, pre-transaction), entity's eligibility
// rules, and port.UnitOfWork (the transaction itself, via
// Repositories.Requests and Repositories.OTP) — never a *pgx.Tx, never SQL.
//
// A creation interactor returns port.RequestView directly rather than a
// package-local copy: port.RequestView already lives in this module's
// use-case layer (internal/usecase/port), so a controller reading it incurs
// no extra dependency the layering doesn't already allow.
package creation

// ClientInput is the identity carried by a particulier or entreprise
// request. CompanyName and RCNumber are read only by the entreprise
// interactor; a restitution has no client at all.
type ClientInput struct {
	LastName    string
	FirstName   string
	BirthDate   string // yyyy-mm-dd, exactly as bound from the request JSON
	BirthPlace  string
	IDType      string
	IDNumber    string
	CompanyName string
	RCNumber    string
}
