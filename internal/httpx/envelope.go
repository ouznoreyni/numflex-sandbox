package httpx

// Envelope is the guide's §8 success envelope, identical in both fidelity modes.
type Envelope struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// EnvelopeSansData serves ANO-011: otp/send's response omits the data field —
// it is neither present nor null.
type EnvelopeSansData struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
