// Package entity holds the enterprise rules of number portability. It knows
// nothing about HTTP, SQL, or the fidelity mode: a business rule is the same
// in both modes, only its rendering differs.
package entity

type Step string

const (
	StepAcceptance   Step = "ACCEPTATION"
	StepDeactivation Step = "DESACTIVATION"
	StepActivation   Step = "ACTIVATION"
	StepConfirmation Step = "CONFIRMATION"
	StepCompletion   Step = "COMPLETION"
)

type RequestStatus string

const (
	RequestInProgress RequestStatus = "EN_COURS"
	RequestCompleted  RequestStatus = "TERMINE"
	RequestCancelled  RequestStatus = "ANNULE"
	// RequestRejected — [HYP] neither documented in the guide nor observed in SIT.
	RequestRejected RequestStatus = "REJETE"
)

type StepStatus string

const (
	StepInProgress StepStatus = "EN_COURS"
	StepCompleted  StepStatus = "TERMINE"
	StepExpired    StepStatus = "EXPIRE"
)

type RequestType string

const (
	RequestTypePorting     RequestType = "PORTAGE"
	RequestTypeRestitution RequestType = "RESTITUTION"
	RequestTypeReverse     RequestType = "REVERSE"
)

type SubscriberType string

const (
	SubscriberIndividual SubscriberType = "PARTICULIER"
	SubscriberEnterprise SubscriberType = "ENTREPRISE"
)

type Role int

const (
	RoleSource Role = iota
	RoleRecipient
	RoleAll
	RoleARTP
)

type PortingRequest struct {
	ID                  string
	MSISDN              string
	RequestType         RequestType
	SubscriberType      SubscriberType
	Status              RequestStatus
	CurrentStep         Step
	CurrentStepStatus   StepStatus
	SourceOperatorID    string
	RecipientOperatorID string
	CreatorOperatorID   string
	// PendingTransition is true between the processing of a step and its
	// effective convergence (R-10).
	PendingTransition bool
}
