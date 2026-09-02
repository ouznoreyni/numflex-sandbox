package entity

import "fmt"

// Incident is the authorization-relevant shape of a declared incident — the
// three columns POST /incidents/{gateway,interne}/:id/resoudre needs to
// decide whether this caller, on this segment, may resolve it. Moved here
// from the deleted internal/api/incidents.go's own inline query.
type Incident struct {
	ID           string
	OperatorID   string
	SystemLocked bool
}

// IncidentResolutionEndpoint names the endpoint that resolves an incident of
// the given segment ("gateway" or "interne") — the same shape
// entity.StepEndpoint already builds for a request's own ETAPE_INVALIDE
// message, reused here for §7.12's own VALIDATION_ECHOUEE one.
func IncidentResolutionEndpoint(segment string) string {
	return fmt.Sprintf("/api/gateway/v1/incidents/%s/{id}/resoudre", segment)
}

// CanResolveIncident decides whether operatorID may resolve inc through the
// segment named by systemLocked (false: gateway, true: interne) — moved
// unchanged from the deleted internal/api/incidents.go's own
// resoudreIncident: the segment mismatch is checked first (§7.12 wants the
// right endpoint named even when the caller has no business resolving this
// incident at all), only then ownership.
func CanResolveIncident(inc Incident, systemLocked bool, operatorID string) *Fault {
	if inc.SystemLocked != systemLocked {
		expected := "gateway"
		if inc.SystemLocked {
			expected = "interne"
		}
		return ValidationFailed(
			"Cet incident se résout via POST " + IncidentResolutionEndpoint(expected) + ".")
	}
	if inc.OperatorID != operatorID {
		return RequestAccessDenied(
			"Seul l'opérateur ayant déclaré l'incident peut le résoudre.")
	}
	return nil
}
