// Package controller is the inbound half of the interface-adapter layer:
// one type per capability, translating an HTTP request into a use case's
// input model and its outcome into a presenter.ViewModel. A controller
// binds JSON, applies the validation that is not the interactor's business
// (a field's shape, say), calls a use case boundary, and hands the result
// to a presenter.Presenter.
//
// May import: the standard library, Gin, internal/entity,
// internal/usecase/port, the internal/usecase/<capability> packages and
// internal/adapter/presenter.
//
// Must never know: SQL, pgx, a business rule, or the concrete engine and
// database behind its gateways. It depends on *gin.Context because that is
// the request type the module's single driving framework uses, but it must
// never import internal/framework — test/architecture_test.go's dependency
// rule refuses it.
package controller
