// Package apperr porte le type d'erreur unique du sandbox. Les règles métier ne
// connaissent que ce type ; le rendu HTTP — enveloppe ARTP ou problem+json JHipster —
// est décidé ailleurs, dans internal/httpx, selon le mode de fidélité.
package apperr

type Kind int

const (
	KindValidation Kind = iota
	KindEtat
	KindAcces
	KindIntrouvable
	KindInterne
)

type FieldError struct {
	ObjectName string `json:"objectName"`
	Field      string `json:"field"`
	Message    string `json:"message"`
}

type Error struct {
	// Code du catalogue §9 du guide. En mode real il n'est jamais émis (ANO-001),
	// mais il reste renseigné : c'est lui qui pilote le mode contract.
	Code    string
	Message string
	Kind    Kind
	Fields  []FieldError

	// RealDetail remplace le champ detail du problem+json en mode real, pour les
	// anomalies dont le texte mesuré diffère du message métier (ANO-002, ANO-020).
	// Vide, le rendu utilise "RuntimeException: " + Message.
	RealDetail string
}

func (e *Error) Error() string { return e.Code + " : " + e.Message }

func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

func Validation(fields ...FieldError) *Error {
	return &Error{
		Kind:    KindValidation,
		Code:    "VALIDATION_ECHOUEE",
		Message: "Un ou plusieurs champs obligatoires sont manquants ou invalides",
		Fields:  fields,
	}
}
