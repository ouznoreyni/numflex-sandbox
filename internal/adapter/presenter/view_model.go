// Package presenter turns a use case's outcome into a transport-agnostic
// ViewModel, in one of the sandbox's two fidelity modes (Real, Contract).
// internal/httpx.Renderer makes the same choice today by branching inline on
// config.Fidelity inside failReel/failContrat; this package is that branch
// turned into two types behind one interface.
package presenter

// ViewModel is the presenter's uniform output: an HTTP status and a body
// ready to be marshalled to JSON by whichever transport a controller uses.
// Unlike internal/httpx.Renderer, which writes straight to a *gin.Context, a
// ViewModel carries no framework dependency.
type ViewModel struct {
	Status int
	Body   any
}
