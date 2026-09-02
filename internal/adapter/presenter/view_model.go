package presenter

// ViewModel is the presenter's uniform output: an HTTP status and a body
// ready to be marshalled to JSON by whichever transport a controller uses.
// Unlike internal/httpx.Renderer, which writes straight to a *gin.Context, a
// ViewModel carries no framework dependency.
type ViewModel struct {
	Status int
	Body   any
}
