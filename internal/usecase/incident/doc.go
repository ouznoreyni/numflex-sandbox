// Package incident holds the three use cases behind guide §7.12's six
// routes — two families, gateway and interne, sharing the same logic
// parameterized by systemLocked (fige_systeme), the only dimension where
// they really diverge: the interne segment alone carries the rule "one
// open internal incident per operator", and freezes the whole market
// (BR-012, FR-028).
//
// May import: the standard library, internal/entity and
// internal/usecase/port.
//
// Must never know: pgx, Gin, or that anything reads the frozen state it
// writes. port.Engine.MarketFrozen simply reads the same incident table,
// so no interactor here drives that state directly.
package incident
