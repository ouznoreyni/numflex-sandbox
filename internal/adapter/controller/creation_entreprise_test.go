package controller_test

// These 8 test functions are moved, unchanged in assertion, from the
// deleted internal/api/creation_entreprise_test.go (Task 12).

import (
	"context"
	"net/http"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
	"github.com/stretchr/testify/require"
)

func corpsEntreprise(porteur string, flotte []string) map[string]any {
	return map[string]any{
		"numeroPorteurFlotte":     porteur,
		"otpCode":                 "123456",
		"operateurSourceId":       operateurOrange,
		"operateurDestinataireId": operateurYAS,
		"typePortabilite":         "POSTPAID",
		"numerosFlotte":           flotte,
		"client": map[string]any{
			"raisonSociale": "Entreprise SARL", "numRC": "123456789",
			"prenom": "Ousmane", "nom": "Diallo", "dateNaissance": "1975-03-20",
			"typePiece": "CNI", "numeroPiece": "1234567890123",
		},
	}
}

func TestFlotteNominale(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	jeton := h.Jeton("yas", "yas2026")
	h.Appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("771000001", []string{"771000001", "771000002", "771000003"}))

	require.Equal(t, http.StatusCreated, rep.StatusCode, corps)
	require.Equal(t, "Demande flotte créée", corps["message"])

	data := corps["data"].(map[string]any)
	demande := data["demande"].(map[string]any)
	require.Equal(t, "ENTREPRISE", demande["typeAbonne"])
	require.Equal(t, "ACCEPTATION", demande["etapeActuelle"])
	require.Equal(t, float64(3), data["numerosPortesCount"])
	require.Equal(t, float64(0), data["numerosExclusCount"])
}

func TestFlotteExclusionPartielle(t *testing.T) {
	// BR-006 / invariant 11 : la flotte réussit avec moins de numéros que demandé.
	h := routerharness.NewRouterHarness(t)
	creerPortage(h, "771000009") // ce numéro a désormais une demande en cours

	jeton := h.Jeton("yas", "yas2026")
	h.Appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("771000001", []string{"771000001", "771000002", "771000009"}))

	require.Equal(t, http.StatusCreated, rep.StatusCode)
	data := corps["data"].(map[string]any)
	require.Equal(t, float64(2), data["numerosPortesCount"])
	require.Equal(t, float64(1), data["numerosExclusCount"])
	require.Equal(t, "1 numéro(s) exclu(s) de la demande.", data["avertissement"])

	exclus := data["numerosExclus"].([]any)
	require.Len(t, exclus, 1)
	premier := exclus[0].(map[string]any)
	require.Equal(t, "771000009", premier["numero"])
	require.Equal(t, "Demande déjà en cours pour ce numéro", premier["raison"])
	require.Equal(t, "DEMANDE_EN_COURS_POUR_NUMERO", premier["codeErreur"])
}

func TestFlotteVide(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	jeton := h.Jeton("yas", "yas2026")
	h.Appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("771000001", []string{}))

	// En fidélité real, FLOTTE_VIDE sort en problem-with-message : c'est une
	// validation métier, pas une violation de bean validation, donc pas de
	// fieldErrors — et aucun champ code (ANO-001). Le message reste lisible.
	require.Equal(t, http.StatusBadRequest, rep.StatusCode)
	require.NotContains(t, corps, "fieldErrors")
	require.NotContains(t, corps, "code")
	require.Equal(t, "La liste des numéros de flotte est vide", corps["detail"])
}

func TestFlotteOperateursMixtes(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	jeton := h.Jeton("yas", "yas2026")
	h.Appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	rep, _ := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("771000001", []string{"771000001", "701000001"}))

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestFlotteAucunNumeroEligible(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	jeton := h.Jeton("yas", "yas2026")
	h.Appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "772000001"})

	// Tranche 772 : portée il y a 30 jours, donc sous le délai de 3 mois.
	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("772000001", []string{"772000001", "772000002"}))

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "RuntimeException: Aucun numéro de la flotte n'est éligible au portage",
		corps["detail"])

	var n int
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM demande").Scan(&n))
	require.Equal(t, 0, n, "aucune demande ne doit être créée")
}

func TestFlotteUnSeulOTPCouvreToutLaFlotte(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	jeton := h.Jeton("yas", "yas2026")
	// OTP envoyé uniquement sur le porteur.
	h.Appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	rep, _ := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("771000001", []string{"771000001", "771000002", "771000003"}))

	require.Equal(t, http.StatusCreated, rep.StatusCode)
}

func TestFlotteExclusionLibereLeNumeroPourUneNouvelleDemande(t *testing.T) {
	// Un numéro exclu d'une flotte ne doit plus être compté comme "en cours" une
	// fois que la demande qui le bloquait réellement se termine — sinon la ligne
	// exclue laissée par la flotte elle-même le bloque indéfiniment (Task 13+).
	h := routerharness.NewRouterHarness(t)
	idBlocage := creerPortage(h, "771000009") // demande réelle, encore EN_COURS

	jeton := h.Jeton("yas", "yas2026")
	h.Appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("771000001", []string{"771000001", "771000002", "771000009"}))
	require.Equal(t, http.StatusCreated, rep.StatusCode, corps)
	data := corps["data"].(map[string]any)
	require.Equal(t, float64(1), data["numerosExclusCount"])

	// La demande qui bloquait réellement 771000009 se termine : plus rien ne le
	// concerne, y compris la ligne exclue de la flotte ci-dessus.
	_, err := h.DB.Pool.Exec(context.Background(),
		"UPDATE demande SET statut_demande = 'TERMINE' WHERE id = $1", idBlocage)
	require.NoError(t, err)

	h.Appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000009"})
	rep2, corps2 := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/particulier", jeton,
		corpsParticulier("771000009"))

	require.Equal(t, http.StatusCreated, rep2.StatusCode, corps2)
}

// TestFlotteVideRenvoieFlotteVide — §9 du guide : le code FLOTTE_VIDE existe au
// catalogue pour ce cas précis (« La liste des numéros de flotte est vide »).
// Le traiter comme une violation de bean validation le rendrait inatteignable.
func TestFlotteVideRenvoieFlotteVide(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.FiabiliteContrat)
	jeton := h.Jeton("yas", "yas2026")
	h.Appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("771000001", []string{}))

	require.Equal(t, http.StatusBadRequest, rep.StatusCode)
	require.Equal(t, "FLOTTE_VIDE", corps["code"])
	require.Equal(t, "La liste des numéros de flotte est vide", corps["message"])
}
