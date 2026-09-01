package entity

// FaultKind classifies a Fault for HTTP status mapping. Rendering — the ARTP
// envelope or JHipster problem+json — is decided elsewhere, in
// internal/httpx, according to the fidelity mode.
type FaultKind int

const (
	FaultValidation FaultKind = iota
	FaultState
	FaultAccess
	FaultNotFound
	FaultInternal
)

// FieldFault names one invalid field, in the shape a bean-validation
// violation would report it.
type FieldFault struct {
	ObjectName string `json:"objectName"`
	Field      string `json:"field"`
	Message    string `json:"message"`
}

// Fault is the sandbox's single error type. Business rules know only this
// type.
type Fault struct {
	// Code from the guide's §9 catalogue. In real mode it is never emitted
	// (ANO-001), but it stays populated: it drives the contract mode.
	Code    string
	Message string
	Kind    FaultKind
	Fields  []FieldFault

	// RealDetail replaces the problem+json detail field in real mode, for
	// anomalies whose measured text differs from the business message
	// (ANO-002, ANO-020). Empty, the rendering uses "RuntimeException: " + Message.
	RealDetail string
}

func (f *Fault) Error() string { return f.Code + " : " + f.Message }

func New(kind FaultKind, code, message string) *Fault {
	return &Fault{Kind: kind, Code: code, Message: message}
}

func Validation(fields ...FieldFault) *Fault {
	return &Fault{
		Kind:    FaultValidation,
		Code:    "VALIDATION_ECHOUEE",
		Message: "Un ou plusieurs champs obligatoires sont manquants ou invalides",
		Fields:  fields,
	}
}
