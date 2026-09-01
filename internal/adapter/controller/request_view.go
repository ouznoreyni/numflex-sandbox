package controller

import "github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"

// requestViewDTO sérialise une demande au format du guide §7.3 — the shape
// every capability that renders a request shares: CreationController,
// QueryController, AcceptanceController and now PortingController's three
// routes (Task 15). It used to be duplicated once per controller (ruling
// R28), kept alive across four tasks purely because internal/api/dto.go's
// own copy, Deps.demandeDTO, still had callers outside clean architecture —
// annulation.go, confirmation.go and traitement.go. Task 15, migrating the
// last of those callers, is where that duplication was always meant to
// collapse: all four copies (three controllers' own plus Deps.demandeDTO)
// were compared field by field before being merged into this one, and found
// behaviourally identical — no drift. Every rendered timestamp passes
// through clk.Rendered: the clock skew exists only at render time, never in
// what is written to the database.
func requestViewDTO(clk port.Clock, v port.RequestView) map[string]any {
	out := map[string]any{
		"id":                    v.ID,
		"numero":                v.MSISDN,
		"typeAbonne":            v.SubscriberType,
		"typeDemande":           v.RequestType,
		"statutDemande":         v.Status,
		"etapeActuelle":         v.CurrentStep,
		"statutEtapeActuel":     v.CurrentStepStatus,
		"operateurSource":       map[string]any{"id": v.SourceOperatorID, "nom": v.SourceOperatorName},
		"operateurDestinataire": map[string]any{"id": v.RecipientOperatorID, "nom": v.RecipientOperatorName},
		"dateDemande":           clk.Rendered(v.RequestDate),
		"processus":             nil,
		"routageInfo":           nil,
	}
	if v.Processus != nil {
		out["processus"] = *v.Processus
	}
	if v.RoutingInfo != nil {
		out["routageInfo"] = *v.RoutingInfo
	}
	if v.CompletionDate != nil {
		out["dateFinalisation"] = clk.Rendered(*v.CompletionDate)
	}
	if v.Client != nil {
		client := map[string]any{
			"nom":           v.Client.LastName,
			"prenom":        v.Client.FirstName,
			"dateNaissance": "",
			"lieuNaissance": v.Client.BirthPlace,
			"typePiece":     v.Client.IDType,
			"numeroPiece":   v.Client.IDNumber,
		}
		if v.Client.BirthDate != nil {
			client["dateNaissance"] = v.Client.BirthDate.Format("2006-01-02")
		}
		out["client"] = client
	}
	return out
}

// sansClient retire le sous-objet client d'un DTO. Les endpoints de
// confirmation — la file, son détail et le POST — sont les seuls à ne pas le
// porter ; mesuré sur quatre captures du 2026-08-27, pas déduit. The single
// remaining copy, after the same consolidation as requestViewDTO above.
func sansClient(dto map[string]any) map[string]any {
	delete(dto, "client")
	return dto
}
