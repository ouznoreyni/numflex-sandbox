package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func ilYA(jours int) *time.Time {
	t := time.Now().AddDate(0, 0, -jours)
	return &t
}

func TestPortageNominal(t *testing.T) {
	n := EtatNumero{MSISDN: "771000001", OperateurActuelID: orange, OperateurOrigineID: orange}
	require.Nil(t, VerifierEligibilitePortage(n, orange, yas, DelaiEntrePortages))
}

func TestPortageOperateurSourceIncorrect(t *testing.T) {
	n := EtatNumero{MSISDN: "771000001", OperateurActuelID: orange, OperateurOrigineID: orange}
	e := VerifierEligibilitePortage(n, expresso, yas, DelaiEntrePortages)
	require.NotNil(t, e)
	require.Equal(t, "OPERATEUR_SOURCE_INCORRECT", e.Code)
}

func TestPortageNumeroDejaChezDestinataire(t *testing.T) {
	n := EtatNumero{MSISDN: "761000001", OperateurActuelID: yas, OperateurOrigineID: yas}
	e := VerifierEligibilitePortage(n, yas, yas, DelaiEntrePortages)
	require.NotNil(t, e)
	require.Equal(t, "NUMERO_DEJA_CHEZ_DESTINATAIRE", e.Code)
}

func TestPortageDemandeDejaEnCours(t *testing.T) {
	n := EtatNumero{MSISDN: "771000001", OperateurActuelID: orange,
		OperateurOrigineID: orange, DemandeEnCours: true}
	e := VerifierEligibilitePortage(n, orange, yas, DelaiEntrePortages)
	require.NotNil(t, e)
	require.Equal(t, "DEMANDE_EN_COURS_POUR_NUMERO", e.Code)
}

func TestPortageDelaiNonRespecte(t *testing.T) {
	n := EtatNumero{MSISDN: "772000001", OperateurActuelID: orange,
		OperateurOrigineID: yas, DateDernierPortage: ilYA(30)}
	e := VerifierEligibilitePortage(n, orange, yas, DelaiEntrePortages)
	require.NotNil(t, e)
	require.Equal(t, "DELAI_PORTAGE_NON_RESPECTE", e.Code)
	// ANO-002 : ce refus se présente comme une panne serveur.
	require.Equal(t, "Unexpected runtime exception", e.RealDetail)
}

func TestPortageDelaiRespecte(t *testing.T) {
	n := EtatNumero{MSISDN: "773000001", OperateurActuelID: yas,
		OperateurOrigineID: orange, DateDernierPortage: ilYA(240)}
	require.Nil(t, VerifierEligibilitePortage(n, yas, expresso, DelaiEntrePortages))
}

func TestRestitutionNumeroNonPorte(t *testing.T) {
	n := EtatNumero{MSISDN: "771000001", OperateurActuelID: orange, OperateurOrigineID: orange}
	e := VerifierEligibiliteRestitution(n, DelaiAvantRestitution)
	require.NotNil(t, e)
	require.Equal(t, "NUMERO_NON_PORTE", e.Code)
}

func TestRestitutionDejaRestituee(t *testing.T) {
	n := EtatNumero{MSISDN: "775000001", OperateurActuelID: yas, OperateurOrigineID: orange,
		DateDernierPortage: ilYA(240), DejaRestitue: true}
	e := VerifierEligibiliteRestitution(n, DelaiAvantRestitution)
	require.NotNil(t, e)
	require.Equal(t, "NUMERO_DEJA_RESTITUE", e.Code)
}

func TestRestitutionTropTot(t *testing.T) {
	n := EtatNumero{MSISDN: "774000001", OperateurActuelID: yas, OperateurOrigineID: orange,
		DateDernierPortage: ilYA(60)}
	e := VerifierEligibiliteRestitution(n, DelaiAvantRestitution)
	require.NotNil(t, e)
	require.Equal(t, "DELAI_RESTITUTION_NON_RESPECTE", e.Code)
	// ANO-020 : le code exploitable est enterré dans une chaîne.
	require.Contains(t, e.RealDetail, "error.numeroRestitutionTooEarly")
}

func TestRestitutionNominale(t *testing.T) {
	n := EtatNumero{MSISDN: "773000001", OperateurActuelID: yas, OperateurOrigineID: orange,
		DateDernierPortage: ilYA(240)}
	require.Nil(t, VerifierEligibiliteRestitution(n, DelaiAvantRestitution))
}
