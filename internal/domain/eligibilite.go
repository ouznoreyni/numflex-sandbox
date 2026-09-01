package domain

import (
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/apperr"
)

const (
	// Délai entre deux portages — motif de rejet ARTP « Dernier portage inférieur à 3 mois ».
	DelaiEntrePortages = 3 * 30 * 24 * time.Hour
	// Délai avant restitution — §7.5 du guide.
	DelaiAvantRestitution = 6 * 30 * 24 * time.Hour
)

type EtatNumero struct {
	MSISDN             string
	OperateurActuelID  string
	OperateurOrigineID string
	DateDernierPortage *time.Time
	DejaRestitue       bool
	DemandeEnCours     bool
}

func VerifierEligibilitePortage(n EtatNumero, sourceID, destinataireID string,
	delaiPortage time.Duration) *apperr.Error {

	if n.OperateurActuelID == destinataireID {
		return apperr.NumeroDejaChezDestinataire()
	}
	if n.OperateurActuelID != sourceID {
		return apperr.OperateurSourceIncorrect()
	}
	if n.DemandeEnCours {
		return apperr.DemandeEnCoursPourNumero()
	}
	if n.DateDernierPortage != nil && time.Since(*n.DateDernierPortage) < delaiPortage {
		return apperr.DelaiPortageNonRespecte()
	}
	return nil
}

func VerifierEligibiliteRestitution(n EtatNumero, delaiRestitution time.Duration) *apperr.Error {
	if n.DateDernierPortage == nil || n.OperateurActuelID == n.OperateurOrigineID {
		return apperr.NumeroNonPorte()
	}
	if n.DejaRestitue {
		return apperr.NumeroDejaRestitue()
	}
	if time.Since(*n.DateDernierPortage) < delaiRestitution {
		return apperr.DelaiRestitutionNonRespecte()
	}
	if n.DemandeEnCours {
		return apperr.DemandeEnCoursPourNumero()
	}
	return nil
}
