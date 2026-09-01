package entity

import "fmt"

// Caller describes the operator behind the token presented on a request.
type Caller struct {
	UserID       string
	Username     string
	OperatorID   string
	OperatorName string
}

// CanProcess decides whether an operator may call /demandes/traitement now.
func CanProcess(r PortingRequest, operatorID string) *Fault {
	// The refusal of a REVERSE's COMPLETION precedes the status check, and
	// that is deliberate: guide §7.9 states this message as the answer to
	// any attempt, without conditioning it on the request's state. Since the
	// engine plays the ARTP, it completes the REVERSE as soon as the last
	// confirmation lands — placed after the status check, this refusal would
	// only have a one-tick window and the operator would almost always
	// receive a generic ETAPE_INVALIDE instead.
	if StepOwner(r.CurrentStep, r.RequestType) == RoleARTP {
		return RequestAccessDenied(
			"La complétion (COMPLETION) d'une demande REVERSE est réservée à l'ARTP, une fois que tous les opérateurs ont confirmé.")
	}

	if r.Status != RequestInProgress {
		return InvalidStep(fmt.Sprintf(
			"Cette demande n'est plus en cours (statut : %s).", r.Status))
	}
	if r.PendingTransition {
		return InvalidStep(fmt.Sprintf(
			"L'étape %s a déjà été traitée pour cette demande.", r.CurrentStep))
	}

	switch r.CurrentStep {
	case StepAcceptance, StepConfirmation:
		return InvalidStep(fmt.Sprintf("L'étape %s se traite via %s.",
			r.CurrentStep, StepEndpoint(r.CurrentStep)))
	}

	switch StepOwner(r.CurrentStep, r.RequestType) {
	case RoleSource:
		if operatorID != r.SourceOperatorID {
			return RequestAccessDenied(fmt.Sprintf(
				"L'étape %s incombe à l'opérateur source.", r.CurrentStep))
		}
	case RoleRecipient:
		if operatorID != r.RecipientOperatorID {
			return RequestAccessDenied(fmt.Sprintf(
				"L'étape %s incombe à l'opérateur destinataire.", r.CurrentStep))
		}
	}
	return nil
}

// ExpectedConfirmers lists the operators whose confirmation is required.
// PORTAGE: every operator on the market except the recipient, who is
// auto-confirmed once the others have validated — verified by measurement at
// SIT, a third operator neither source nor recipient having to confirm to
// settle the step.
// RESTITUTION and REVERSE: everyone, recipient included.
func ExpectedConfirmers(r PortingRequest, allOperators []string) []string {
	out := make([]string, 0, len(allOperators))
	for _, op := range allOperators {
		if r.RequestType == RequestTypePorting && op == r.RecipientOperatorID {
			continue
		}
		out = append(out, op)
	}
	return out
}

// CanAccept decides whether an operator may accept or reject a request.
func CanAccept(r PortingRequest, operatorID string) *Fault {
	if r.Status != RequestInProgress {
		return InvalidStep(fmt.Sprintf(
			"Cette demande n'est plus en cours (statut : %s).", r.Status))
	}
	if r.CurrentStep != StepAcceptance || r.PendingTransition {
		return InvalidStep(fmt.Sprintf(
			"Cette demande n'est plus à l'étape ACCEPTATION (étape actuelle : %s).",
			r.CurrentStep))
	}
	if operatorID != r.SourceOperatorID {
		return RequestAccessDenied(
			"Seul l'opérateur source peut accepter ou rejeter cette demande.")
	}
	return nil
}

// CanCancel decides whether an operator may cancel a request.
func CanCancel(r PortingRequest, operatorID string) *Fault {
	if operatorID != r.CreatorOperatorID {
		return RequestAccessDenied(
			"Seul l'opérateur ayant créé la demande (opérateur destinataire) peut l'annuler.")
	}
	if r.Status != RequestInProgress || r.CurrentStep != StepAcceptance {
		return InvalidStep(fmt.Sprintf(
			"Cette demande ne peut plus être annulée (étape actuelle : %s).", r.CurrentStep))
	}
	return nil
}
