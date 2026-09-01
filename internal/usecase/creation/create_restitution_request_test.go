package creation_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/creation"
)

func restitutionInteractor(f *fixture) *creation.CreateRestitutionRequestInteractor {
	return creation.NewCreateRestitutionRequest(f.numbers, f.uow, f.requests, f.ids, f.clock)
}

func TestCreateRestitutionRequestNominal(t *testing.T) {
	f := newFixture()
	portedLongAgo := time.Now().AddDate(0, 0, -240) // bien au-delà des 6 mois
	f.numbers.Seed(entity.NumberState{
		MSISDN: "773000001", CurrentOperatorID: yasID, OriginOperatorID: orangeID,
		LastPortingDate: &portedLongAgo,
	})

	view, fault := restitutionInteractor(f).Execute(ctxCaller(orangeID),
		creation.CreateRestitutionRequestInput{MSISDN: "773000001"})
	require.Nil(t, fault)
	require.Equal(t, "RESTITUTION", view.RequestType)
	// L'opérateur d'origine (appelant) récupère le numéro : il est destinataire.
	require.Equal(t, orangeID, view.RecipientOperatorID)
	require.Equal(t, yasID, view.SourceOperatorID)
	require.Nil(t, view.RoutingInfo)
	require.Nil(t, view.Processus)
	require.Nil(t, view.Client, "une restitution ne porte aucune identité client")
}

func TestCreateRestitutionRequestNumeroNonPorte(t *testing.T) {
	f := newFixture()
	// Jamais porté : détenteur actuel == opérateur d'origine.
	f.numbers.Seed(entity.NumberState{MSISDN: "771000001", CurrentOperatorID: orangeID, OriginOperatorID: orangeID})

	_, fault := restitutionInteractor(f).Execute(ctxCaller(orangeID),
		creation.CreateRestitutionRequestInput{MSISDN: "771000001"})
	require.NotNil(t, fault)
	require.Equal(t, "NUMERO_NON_PORTE", fault.Code)
}

func TestCreateRestitutionRequestReserveeALOperateurDOrigine(t *testing.T) {
	f := newFixture()
	portedLongAgo := time.Now().AddDate(0, 0, -240)
	f.numbers.Seed(entity.NumberState{
		MSISDN: "773000001", CurrentOperatorID: yasID, OriginOperatorID: orangeID,
		LastPortingDate: &portedLongAgo,
	})

	// L'appelant n'est ni le détenteur ni l'origine du numéro.
	_, fault := restitutionInteractor(f).Execute(ctxCaller("operateur-expresso"),
		creation.CreateRestitutionRequestInput{MSISDN: "773000001"})
	require.NotNil(t, fault)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", fault.Code)
	require.Equal(t, 0, f.requests.RequestCount())
}

func TestCreateRestitutionRequestDelaiNonRespecte(t *testing.T) {
	f := newFixture()
	portedRecently := time.Now().AddDate(0, 0, -60) // 60 jours < 6 mois
	f.numbers.Seed(entity.NumberState{
		MSISDN: "774000001", CurrentOperatorID: yasID, OriginOperatorID: orangeID,
		LastPortingDate: &portedRecently,
	})

	_, fault := restitutionInteractor(f).Execute(ctxCaller(orangeID),
		creation.CreateRestitutionRequestInput{MSISDN: "774000001"})
	require.NotNil(t, fault)
	require.Equal(t, "DELAI_RESTITUTION_NON_RESPECTE", fault.Code)
}

func TestCreateRestitutionRequestDejaRestituee(t *testing.T) {
	f := newFixture()
	portedLongAgo := time.Now().AddDate(0, 0, -240)
	f.numbers.Seed(entity.NumberState{
		MSISDN: "775000001", CurrentOperatorID: yasID, OriginOperatorID: orangeID,
		LastPortingDate: &portedLongAgo, AlreadyRestituted: true,
	})

	_, fault := restitutionInteractor(f).Execute(ctxCaller(orangeID),
		creation.CreateRestitutionRequestInput{MSISDN: "775000001"})
	require.NotNil(t, fault)
	require.Equal(t, "NUMERO_DEJA_RESTITUE", fault.Code)
}
