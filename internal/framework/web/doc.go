// Package web is the driving framework: the Gin engine, the gateway prefix
// guard, the middlewares under ./middleware (authentication, CORS), and the
// wiring of all 36 routes. One file per capability builds that capability's
// gateways, interactors, presenter and controller exactly once, then
// declares its routes; router.go assembles them in the order the guide
// lists them, with the SANDBOX_ADMIN group added only when configured.
//
// May import: anything — it is the outermost layer, with cmd/.
//
// Must never know: a business rule, or SQL. Nothing here decides an
// outcome; it only builds the objects that do and hands them a request.
package web
