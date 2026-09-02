package entity

// stepSequence is the fixed order a request's steps follow.
var stepSequence = []Step{
	StepAcceptance, StepDeactivation, StepActivation,
	StepConfirmation, StepCompletion,
}

// NextStep returns the step that follows s, or false if s is terminal.
func NextStep(s Step) (Step, bool) {
	for i, x := range stepSequence {
		if x == s && i+1 < len(stepSequence) {
			return stepSequence[i+1], true
		}
	}
	return "", false
}

// StepOwner returns the role responsible for a given step.
func StepOwner(s Step, rt RequestType) Role {
	switch s {
	case StepAcceptance, StepDeactivation:
		return RoleSource
	case StepActivation:
		return RoleRecipient
	case StepConfirmation:
		return RoleAll
	case StepCompletion:
		if rt == RequestTypeReverse {
			return RoleARTP
		}
		return RoleRecipient
	}
	return RoleARTP
}

// StepEndpoint names the endpoint that processes a step, to build the
// ETAPE_INVALIDE message the way the guide writes it (§7.10).
func StepEndpoint(s Step) string {
	switch s {
	case StepAcceptance:
		return "POST /api/gateway/v1/demandes/acceptation"
	case StepConfirmation:
		return "POST /api/gateway/v1/demandes/a-confirmer"
	default:
		return "POST /api/gateway/v1/demandes/traitement"
	}
}
