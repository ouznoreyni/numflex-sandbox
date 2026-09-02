// Package entity holds the enterprise rules of number portability: what a
// porting request is, which step follows which, who may act at each step,
// what makes a number eligible, which incident freezes the market
// (BR-012), and the fault catalog every outer layer translates. A rule
// here is true whoever asks and however the answer is rendered.
//
// May import: the standard library, and nothing else. Every instant it
// reasons about is passed in as an argument.
//
// Must never know: HTTP, Gin, SQL, pgx, the fidelity mode, the wall clock,
// or that a use case, a controller or a presenter exists. It is the
// innermost layer, so it cannot import any package of this module — a rule
// test/architecture_test.go's TestEntityIsPure enforces literally.
package entity
