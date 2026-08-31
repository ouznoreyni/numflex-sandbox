package domain

import (
	"fmt"

	"github.com/yas/numflex-sandbox/internal/apperr"
)

var sequence = []Etape{
	EtapeAcceptation, EtapeDesactivation, EtapeActivation,
	EtapeConfirmation, EtapeCompletion,
}

func EtapeSuivante(e Etape) (Etape, bool) {
	for i, x := range sequence {
		if x == e && i+1 < len(sequence) {
			return sequence[i+1], true
		}
	}
	return "", false
}

func ResponsableEtape(e Etape, td TypeDemande) Role {
	switch e {
	case EtapeAcceptation, EtapeDesactivation:
		return RoleSource
	case EtapeActivation:
		return RoleDestinataire
	case EtapeConfirmation:
		return RoleTous
	case EtapeCompletion:
		if td == TypeReverse {
			return RoleARTP
		}
		return RoleDestinataire
	}
	return RoleARTP
}

// EndpointEtape nomme l'endpoint qui traite une étape, pour construire le message
// d'ETAPE_INVALIDE tel que le guide le rédige (§7.10).
func EndpointEtape(e Etape) string {
	switch e {
	case EtapeAcceptation:
		return "POST /api/gateway/v1/demandes/acceptation"
	case EtapeConfirmation:
		return "POST /api/gateway/v1/demandes/a-confirmer"
	default:
		return "POST /api/gateway/v1/demandes/traitement"
	}
}

// PeutTraiter décide si un opérateur peut appeler /demandes/traitement maintenant.
func PeutTraiter(d Demande, operateurID string) *apperr.Error {
	if d.StatutDemande != StatutEnCours {
		return apperr.EtapeInvalide(fmt.Sprintf(
			"Cette demande n'est plus en cours (statut : %s).", d.StatutDemande))
	}
	if d.TransitionEnAttente {
		return apperr.EtapeInvalide(fmt.Sprintf(
			"L'étape %s a déjà été traitée pour cette demande.", d.EtapeActuelle))
	}

	switch d.EtapeActuelle {
	case EtapeAcceptation, EtapeConfirmation:
		return apperr.EtapeInvalide(fmt.Sprintf("L'étape %s se traite via %s.",
			d.EtapeActuelle, EndpointEtape(d.EtapeActuelle)))
	}

	switch ResponsableEtape(d.EtapeActuelle, d.TypeDemande) {
	case RoleARTP:
		return apperr.DemandeAccesRefuse(
			"La complétion (COMPLETION) d'une demande REVERSE est réservée à l'ARTP, une fois que tous les opérateurs ont confirmé.")
	case RoleSource:
		if operateurID != d.OperateurSourceID {
			return apperr.DemandeAccesRefuse(fmt.Sprintf(
				"L'étape %s incombe à l'opérateur source.", d.EtapeActuelle))
		}
	case RoleDestinataire:
		if operateurID != d.OperateurDestinataireID {
			return apperr.DemandeAccesRefuse(fmt.Sprintf(
				"L'étape %s incombe à l'opérateur destinataire.", d.EtapeActuelle))
		}
	}
	return nil
}

// ConfirmateursAttendus liste les opérateurs dont la confirmation est requise.
// PORTAGE : tous les opérateurs de la place sauf le destinataire, qui est
// auto-confirmé une fois les autres validés — vérifié par mesure au SIT, un
// opérateur tiers ni source ni destinataire devant confirmer pour solder l'étape.
// RESTITUTION et REVERSE : tout le monde, destinataire compris.
func ConfirmateursAttendus(d Demande, tousOperateurs []string) []string {
	out := make([]string, 0, len(tousOperateurs))
	for _, op := range tousOperateurs {
		if d.TypeDemande == TypePortage && op == d.OperateurDestinataireID {
			continue
		}
		out = append(out, op)
	}
	return out
}

func PeutAccepter(d Demande, operateurID string) *apperr.Error {
	if d.StatutDemande != StatutEnCours {
		return apperr.EtapeInvalide(fmt.Sprintf(
			"Cette demande n'est plus en cours (statut : %s).", d.StatutDemande))
	}
	if d.EtapeActuelle != EtapeAcceptation || d.TransitionEnAttente {
		return apperr.EtapeInvalide(fmt.Sprintf(
			"Cette demande n'est plus à l'étape ACCEPTATION (étape actuelle : %s).",
			d.EtapeActuelle))
	}
	if operateurID != d.OperateurSourceID {
		return apperr.DemandeAccesRefuse(
			"Seul l'opérateur source peut accepter ou rejeter cette demande.")
	}
	return nil
}

func PeutAnnuler(d Demande, operateurID string) *apperr.Error {
	if operateurID != d.CreateurOperateurID {
		return apperr.DemandeAccesRefuse(
			"Seul l'opérateur ayant créé la demande (opérateur destinataire) peut l'annuler.")
	}
	if d.StatutDemande != StatutEnCours || d.EtapeActuelle != EtapeAcceptation {
		return apperr.EtapeInvalide(fmt.Sprintf(
			"Cette demande ne peut plus être annulée (étape actuelle : %s).", d.EtapeActuelle))
	}
	return nil
}
