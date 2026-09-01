package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func daysAgo(days int) *time.Time {
	t := time.Now().AddDate(0, 0, -days)
	return &t
}

func TestPortingNominal(t *testing.T) {
	n := NumberState{MSISDN: "771000001", CurrentOperatorID: orange, OriginOperatorID: orange}
	require.Nil(t, CheckPortingEligibility(n, orange, yas, DelayBetweenPortings))
}

func TestPortingIncorrectSourceOperator(t *testing.T) {
	n := NumberState{MSISDN: "771000001", CurrentOperatorID: orange, OriginOperatorID: orange}
	e := CheckPortingEligibility(n, expresso, yas, DelayBetweenPortings)
	require.NotNil(t, e)
	require.Equal(t, "OPERATEUR_SOURCE_INCORRECT", e.Code)
}

func TestPortingNumberAlreadyAtRecipient(t *testing.T) {
	n := NumberState{MSISDN: "761000001", CurrentOperatorID: yas, OriginOperatorID: yas}
	e := CheckPortingEligibility(n, yas, yas, DelayBetweenPortings)
	require.NotNil(t, e)
	require.Equal(t, "NUMERO_DEJA_CHEZ_DESTINATAIRE", e.Code)
}

func TestPortingRequestAlreadyInProgress(t *testing.T) {
	n := NumberState{MSISDN: "771000001", CurrentOperatorID: orange,
		OriginOperatorID: orange, RequestInProgress: true}
	e := CheckPortingEligibility(n, orange, yas, DelayBetweenPortings)
	require.NotNil(t, e)
	require.Equal(t, "DEMANDE_EN_COURS_POUR_NUMERO", e.Code)
}

func TestPortingDelayNotRespected(t *testing.T) {
	n := NumberState{MSISDN: "772000001", CurrentOperatorID: orange,
		OriginOperatorID: yas, LastPortingDate: daysAgo(30)}
	e := CheckPortingEligibility(n, orange, yas, DelayBetweenPortings)
	require.NotNil(t, e)
	require.Equal(t, "DELAI_PORTAGE_NON_RESPECTE", e.Code)
	// ANO-002 : ce refus se présente comme une panne serveur.
	require.Equal(t, "Unexpected runtime exception", e.RealDetail)
}

func TestPortingDelayRespected(t *testing.T) {
	n := NumberState{MSISDN: "773000001", CurrentOperatorID: yas,
		OriginOperatorID: orange, LastPortingDate: daysAgo(240)}
	require.Nil(t, CheckPortingEligibility(n, yas, expresso, DelayBetweenPortings))
}

func TestRestitutionNumberNotPorted(t *testing.T) {
	n := NumberState{MSISDN: "771000001", CurrentOperatorID: orange, OriginOperatorID: orange}
	e := CheckRestitutionEligibility(n, DelayBeforeRestitution)
	require.NotNil(t, e)
	require.Equal(t, "NUMERO_NON_PORTE", e.Code)
}

func TestRestitutionAlreadyRestituted(t *testing.T) {
	n := NumberState{MSISDN: "775000001", CurrentOperatorID: yas, OriginOperatorID: orange,
		LastPortingDate: daysAgo(240), AlreadyRestituted: true}
	e := CheckRestitutionEligibility(n, DelayBeforeRestitution)
	require.NotNil(t, e)
	require.Equal(t, "NUMERO_DEJA_RESTITUE", e.Code)
}

func TestRestitutionTooEarly(t *testing.T) {
	n := NumberState{MSISDN: "774000001", CurrentOperatorID: yas, OriginOperatorID: orange,
		LastPortingDate: daysAgo(60)}
	e := CheckRestitutionEligibility(n, DelayBeforeRestitution)
	require.NotNil(t, e)
	require.Equal(t, "DELAI_RESTITUTION_NON_RESPECTE", e.Code)
	// ANO-020 : le code exploitable est enterré dans une chaîne.
	require.Contains(t, e.RealDetail, "error.numeroRestitutionTooEarly")
}

func TestRestitutionNominal(t *testing.T) {
	n := NumberState{MSISDN: "773000001", CurrentOperatorID: yas, OriginOperatorID: orange,
		LastPortingDate: daysAgo(240)}
	require.Nil(t, CheckRestitutionEligibility(n, DelayBeforeRestitution))
}
