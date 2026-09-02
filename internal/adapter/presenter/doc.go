// Package presenter is the outbound half of the interface-adapter layer:
// it turns a use case's outcome into a transport-agnostic ViewModel, in one
// of the sandbox's two fidelity modes. Real reproduces what the platform
// actually returns — JHipster problem+json, business errors in HTTP 500
// (ANO-003), no code field (ANO-001) — and Contract returns what the guide
// promises. The mode is the only thing that separates the two.
//
// May import: the standard library, internal/entity and
// internal/usecase/port. Its tests additionally import
// internal/testsupport/inmemory for the port doubles they present from —
// a test-only edge the dependency rule does not classify, since
// testsupport belongs to no layer.
//
// Must never know: Gin, HTTP writers, SQL, or a business rule. It receives
// the request path as an argument rather than reading it from a request.
package presenter
