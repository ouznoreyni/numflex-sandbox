package entity

// This file holds the five read-only reference-data rows exposed by
// /operateurs, /motifs-rejet, /types-demande, /processus and /types-incident.
// None of them carries behaviour: a reference row is a fact looked up from a
// catalog table, not a rule to apply.

// Operator is one row of the /operateurs catalog (table operateur).
type Operator struct {
	ID   string
	Name string
}

// RejectionReason is one row of the /motifs-rejet catalog (table
// motif_rejet). ANO-009: the JSON field the guide documents is "motif", not
// "libelle" — see internal/adapter/controller/reference_controller.go.
type RejectionReason struct {
	ID     string
	Reason string
}

// RequestTypeRef is one row of the /types-demande catalog (table
// type_demande) — named distinctly from RequestType (porting_request.go),
// the enum PortingRequest.RequestType holds, to avoid a name collision in
// this package; the two happen to share the same PORTAGE/RESTITUTION/REVERSE
// values, but this struct is a catalog row, not the enum itself.
type RequestTypeRef struct {
	ID   string
	Type string
}

// Process is one row of the /processus catalog (table processus).
type Process struct {
	ID   string
	Type string
}

// IncidentType is one row of the /types-incident catalog (table
// type_incident).
type IncidentType struct {
	ID           string
	Label        string
	SystemLocked bool
}
