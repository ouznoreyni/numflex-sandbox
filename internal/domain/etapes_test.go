package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/apperr"
)

const (
	orange   = "6a21745ce6c37b5b5b487ec1"
	yas      = "6a2174c3e6c37b5b5b487ec4"
	expresso = "6a217510e6c37b5b5b487ec7"
)

var place = []string{orange, yas, expresso}

func portage() Demande {
	return Demande{
		ID: "d1", TypeDemande: TypePortage, TypeAbonne: AbonneParticulier,
		StatutDemande: StatutEnCours, EtapeActuelle: EtapeAcceptation,
		StatutEtapeActuel: EtapeEnCours,
		OperateurSourceID: orange, OperateurDestinataireID: yas,
		CreateurOperateurID: yas,
	}
}

func TestSequenceDesEtapes(t *testing.T) {
	suite := []Etape{
		EtapeAcceptation, EtapeDesactivation, EtapeActivation,
		EtapeConfirmation, EtapeCompletion,
	}
	for i := 0; i < len(suite)-1; i++ {
		suivante, ok := EtapeSuivante(suite[i])
		require.True(t, ok, string(suite[i]))
		require.Equal(t, suite[i+1], suivante)
	}
	_, ok := EtapeSuivante(EtapeCompletion)
	require.False(t, ok, "COMPLETION est terminale")
}

func TestResponsableEtape(t *testing.T) {
	require.Equal(t, RoleSource, ResponsableEtape(EtapeAcceptation, TypePortage))
	require.Equal(t, RoleSource, ResponsableEtape(EtapeDesactivation, TypePortage))
	require.Equal(t, RoleDestinataire, ResponsableEtape(EtapeActivation, TypePortage))
	require.Equal(t, RoleTous, ResponsableEtape(EtapeConfirmation, TypePortage))
	require.Equal(t, RoleDestinataire, ResponsableEtape(EtapeCompletion, TypePortage))

	// La COMPLETION d'un REVERSE est réservée à l'ARTP.
	require.Equal(t, RoleARTP, ResponsableEtape(EtapeCompletion, TypeReverse))
	require.Equal(t, RoleDestinataire, ResponsableEtape(EtapeCompletion, TypeRestitution))
}

func TestPeutTraiterRefuseLEtapeQuiNIncombePas(t *testing.T) {
	d := portage()
	d.EtapeActuelle = EtapeDesactivation

	require.Nil(t, PeutTraiter(d, orange), "la source traite la DESACTIVATION")

	e := PeutTraiter(d, yas)
	require.NotNil(t, e, "le destinataire ne peut pas désactiver")
	require.Equal(t, "DEMANDE_ACCES_REFUSE", e.Code)
}

func TestPeutTraiterRefuseAcceptationEtConfirmation(t *testing.T) {
	d := portage()

	e := PeutTraiter(d, orange)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
	require.Equal(t,
		"L'étape ACCEPTATION se traite via POST /api/gateway/v1/demandes/acceptation.",
		e.Message)

	d.EtapeActuelle = EtapeConfirmation
	e = PeutTraiter(d, orange)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
	require.Equal(t,
		"L'étape CONFIRMATION se traite via POST /api/gateway/v1/demandes/a-confirmer.",
		e.Message)
}

func TestPeutTraiterRefuseCompletionReverse(t *testing.T) {
	d := portage()
	d.TypeDemande = TypeReverse
	d.EtapeActuelle = EtapeCompletion

	e := PeutTraiter(d, yas)
	require.NotNil(t, e)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", e.Code)
	require.Equal(t,
		"La complétion (COMPLETION) d'une demande REVERSE est réservée à l'ARTP, une fois que tous les opérateurs ont confirmé.",
		e.Message)
}

func TestPeutTraiterRefuseUneDemandeNonEnCours(t *testing.T) {
	d := portage()
	d.EtapeActuelle = EtapeDesactivation
	d.StatutDemande = StatutAnnule

	e := PeutTraiter(d, orange)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
}

func TestPeutTraiterRefusePendantLaConvergence(t *testing.T) {
	// R-10 : l'étape a été traitée, la transition n'est pas encore appliquée.
	d := portage()
	d.EtapeActuelle = EtapeDesactivation
	d.TransitionEnAttente = true

	e := PeutTraiter(d, orange)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
}

func TestConfirmateursPortageExcluentLeDestinataire(t *testing.T) {
	// D-6, mesuré au SIT : sur un portage ORANGE → YAS, EXPRESSO doit confirmer.
	d := portage()
	d.EtapeActuelle = EtapeConfirmation

	require.ElementsMatch(t, []string{orange, expresso}, ConfirmateursAttendus(d, place))
}

func TestConfirmateursRestitutionEtReverseIncluentTousLeMonde(t *testing.T) {
	for _, td := range []TypeDemande{TypeRestitution, TypeReverse} {
		d := portage()
		d.TypeDemande = td
		d.EtapeActuelle = EtapeConfirmation
		require.ElementsMatchf(t, place, ConfirmateursAttendus(d, place), string(td))
	}
}

func TestPeutAccepter(t *testing.T) {
	d := portage()

	require.Nil(t, PeutAccepter(d, orange))

	// TC-034 : le destinataire ne peut pas accepter sa propre demande.
	e := PeutAccepter(d, yas)
	require.NotNil(t, e)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", e.Code)

	// Un tiers non plus.
	require.NotNil(t, PeutAccepter(d, expresso))

	// Hors de l'étape ACCEPTATION.
	d2 := portage()
	d2.EtapeActuelle = EtapeActivation
	e = PeutAccepter(d2, orange)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
}

func TestPeutAnnuler(t *testing.T) {
	d := portage()

	require.Nil(t, PeutAnnuler(d, yas), "le créateur annule")

	e := PeutAnnuler(d, orange)
	require.NotNil(t, e)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", e.Code)
	require.Equal(t,
		"Seul l'opérateur ayant créé la demande (opérateur destinataire) peut l'annuler.",
		e.Message)

	d.EtapeActuelle = EtapeDesactivation
	e = PeutAnnuler(d, yas)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
	require.Equal(t,
		"Cette demande ne peut plus être annulée (étape actuelle : DESACTIVATION).",
		e.Message)
}

func TestErreursSontDesAppErr(t *testing.T) {
	var e *apperr.Error = PeutAnnuler(portage(), orange)
	require.NotNil(t, e)
}
