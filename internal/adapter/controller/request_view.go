package controller

import (
	"context"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// requestViewDTO serializes a request into the guide §7.3 shape — the shape
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
//
// A fleet (ENTREPRISE) follows the same shape, with the two differences
// measured on 2026-09-18 against rec-numflex.artp.sn: numeros[] replaces
// numero, and the client carries raisonSociale and numRC instead of
// lieuNaissance.
func requestViewDTO(ctx context.Context, clk port.Clock, v port.RequestView) map[string]any {
	fleet := v.SubscriberType == string(entity.SubscriberEnterprise)
	out := map[string]any{
		"id":                    v.ID,
		"typeAbonne":            v.SubscriberType,
		"typeDemande":           v.RequestType,
		"statutDemande":         v.Status,
		"etapeActuelle":         v.CurrentStep,
		"statutEtapeActuel":     v.CurrentStepStatus,
		"operateurSource":       map[string]any{"id": v.SourceOperatorID, "nom": v.SourceOperatorName},
		"operateurDestinataire": map[string]any{"id": v.RecipientOperatorID, "nom": v.RecipientOperatorName},
		"dateDemande":           renderedInstant(ctx, clk, v.RequestDate),
		"processus":             nil,
		"routageInfo":           nil,
	}
	if v.Process != nil {
		out["processus"] = *v.Process
	}
	if v.RoutingInfo != nil {
		out["routageInfo"] = *v.RoutingInfo
	}
	if v.CompletionDate != nil {
		out["dateFinalisation"] = renderedInstant(ctx, clk, *v.CompletionDate)
	}
	// An individual carries its number; a fleet carries the retained numbers
	// in declared order, never the carrier alone (v.MSISDN stays internal:
	// it is the number that received the OTP).
	if fleet {
		numbers := v.Numbers
		if numbers == nil {
			numbers = []string{}
		}
		out["numeros"] = numbers
	} else {
		out["numero"] = v.MSISDN
	}
	// Individual: six fields, lieuNaissance included (2026-08-27). Fleet:
	// seven, raisonSociale and numRC in place of lieuNaissance (2026-09-18).
	if v.Client != nil {
		client := map[string]any{
			"nom":           v.Client.LastName,
			"prenom":        v.Client.FirstName,
			"dateNaissance": "",
			"typePiece":     v.Client.IDType,
			"numeroPiece":   v.Client.IDNumber,
		}
		if v.Client.BirthDate != nil {
			client["dateNaissance"] = v.Client.BirthDate.Format("2006-01-02")
		}
		if fleet {
			client["raisonSociale"] = v.Client.CompanyName
			client["numRC"] = v.Client.RCNumber
		} else {
			client["lieuNaissance"] = v.Client.BirthPlace
		}
		out["client"] = client
	}
	return out
}

// sansClient removes the client sub-object from a DTO. The confirmation
// endpoints — the queue, its detail and the POST — are the only ones that
// do not carry it; measured against four 2026-08-27 captures, not deduced.
// The single remaining copy, after the same consolidation as requestViewDTO
// above.
func sansClient(dto map[string]any) map[string]any {
	delete(dto, "client")
	return dto
}
