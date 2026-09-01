package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOTPSendOmetLeChampDataEnModeReel(t *testing.T) {
	// ANO-011 : le champ data est absent, pas null.
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/otp/send",
		h.jeton("yas", "yas2026"), map[string]any{"numero": "771000001"})

	require.Equal(t, http.StatusOK, rep.StatusCode)
	require.Equal(t, true, corps["success"])
	require.NotContains(t, corps, "data")
}

func TestOTPVerifyNeConsommePas(t *testing.T) {
	// TC-021 : verify pré-vérifie sans consommer — le code reste utilisable.
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")

	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	for i := 0; i < 3; i++ {
		rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/otp/verify", jeton,
			map[string]any{"numero": "771000001", "otpCode": "123456"})
		require.Equal(t, http.StatusOK, rep.StatusCode, "vérification %d", i)
	}

	var consomme bool
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT consomme FROM otp WHERE numero = $1", "771000001").Scan(&consomme))
	require.False(t, consomme)
}

func TestOTPVerifyCodeIncorrect(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/otp/verify", jeton,
		map[string]any{"numero": "771000001", "otpCode": "000000"})

	// ANO-003 : les erreurs d'état sortent en 500 en mode réel.
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.NotContains(t, corps, "code")
}

func TestOTPMaxTentatives(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	for i := 0; i < 3; i++ {
		h.appel(http.MethodPost, "/api/gateway/v1/otp/verify", jeton,
			map[string]any{"numero": "771000001", "otpCode": "000000"})
	}

	// La quatrième tentative est refusée même avec le bon code.
	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/otp/verify", jeton,
		map[string]any{"numero": "771000001", "otpCode": "123456"})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestOTPExpire(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	_, err := h.db.Pool.Exec(context.Background(),
		"UPDATE otp SET expire_a = now() - interval '1 minute' WHERE numero = $1", "771000001")
	require.NoError(t, err)

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/otp/verify", jeton,
		map[string]any{"numero": "771000001", "otpCode": "123456"})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "Le code OTP a expiré", corps["detail"])
}

func TestOTPAbsent(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/otp/verify",
		h.jeton("yas", "yas2026"),
		map[string]any{"numero": "779999999", "otpCode": "123456"})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "Aucun OTP actif pour ce numéro", corps["detail"])
}

func TestOTPNumeroInvalideEstUneErreurDeValidation(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/otp/send",
		h.jeton("yas", "yas2026"), map[string]any{"numero": "77"})

	require.Equal(t, http.StatusBadRequest, rep.StatusCode)
	require.Contains(t, corps, "fieldErrors")
}

func TestOTPRenvoiReinitialiseCompteurEtConsommation(t *testing.T) {
	// Le brief exige qu'un renvoi d'OTP sur un numéro déjà couvert remplace le
	// code et remette tentatives/consomme à zéro/faux (clause ON CONFLICT).
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")

	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	// Deux tentatives ratées entament le compteur sans l'épuiser.
	for i := 0; i < 2; i++ {
		h.appel(http.MethodPost, "/api/gateway/v1/otp/verify", jeton,
			map[string]any{"numero": "771000001", "otpCode": "000000"})
	}

	var tentatives int
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT tentatives FROM otp WHERE numero = $1", "771000001").Scan(&tentatives))
	require.Equal(t, 2, tentatives, "précondition : le compteur a bien été entamé")

	// Le renvoi doit réinitialiser tentatives et consomme.
	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	var consomme bool
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT tentatives, consomme FROM otp WHERE numero = $1", "771000001").
		Scan(&tentatives, &consomme))
	require.Equal(t, 0, tentatives, "le renvoi doit remettre le compteur à zéro")
	require.False(t, consomme, "le renvoi doit remettre consomme à faux")

	// Le code reste vérifiable après le renvoi.
	rep, _ = h.appel(http.MethodPost, "/api/gateway/v1/otp/verify", jeton,
		map[string]any{"numero": "771000001", "otpCode": "123456"})
	require.Equal(t, http.StatusOK, rep.StatusCode)
}
