package httpx

// Enveloppe de succès du §8 du guide, identique dans les deux modes de fidélité.
type Envelope struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// EnvelopeSansData sert ANO-011 : la réponse de otp/send omet le champ data —
// il n'est ni présent, ni null.
type EnvelopeSansData struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
