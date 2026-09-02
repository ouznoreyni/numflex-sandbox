package controller_test

// These 8 test functions are moved, unchanged in assertion, from the
// deleted internal/api/otp_test.go (Task 9). They still exercise the real,
// live router — routerharness.NewRouterHarness wraps api.NewRouter, wired
// exactly as cmd/server/main.go wires it — so a green run here proves a
// real HTTP request to /api/gateway/v1/otp/send or /otp/verify goes through
// the new OTPController and not through any leftover handler.
//
// This package (internal/adapter/controller) may not import
// internal/framework directly (test/architecture_test.go's dependency
// rule, which inspects test-file imports too); routerharness.RouterHarness
// is what lets this test drive the whole HTTP stack without doing so.

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
)

func TestOTPSendOmitsTheDataFieldInRealMode(t *testing.T) {
	// ANO-011: the data field is absent, not null.
	h := routerharness.NewRouterHarness(t)
	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/otp/send",
		h.Token("yas", "yas2026"), map[string]any{"numero": "771000001"})

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, true, body["success"])
	require.NotContains(t, body, "data")
}

func TestOTPVerifyDoesNotConsume(t *testing.T) {
	// TC-021: verify pre-checks without consuming — the code stays usable.
	h := routerharness.NewRouterHarness(t)
	token := h.Token("yas", "yas2026")

	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000001"})

	for i := 0; i < 3; i++ {
		resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/otp/verify", token,
			map[string]any{"numero": "771000001", "otpCode": "123456"})
		require.Equal(t, http.StatusOK, resp.StatusCode, "verification %d", i)
	}

	var consumed bool
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		"SELECT consomme FROM otp WHERE numero = $1", "771000001").Scan(&consumed))
	require.False(t, consumed)
}

func TestOTPVerifyIncorrectCode(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	token := h.Token("yas", "yas2026")
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000001"})

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/otp/verify", token,
		map[string]any{"numero": "771000001", "otpCode": "000000"})

	// ANO-003: state errors come out as 500 in real mode.
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.NotContains(t, body, "code")
}

func TestOTPMaxAttempts(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	token := h.Token("yas", "yas2026")
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000001"})

	for i := 0; i < 3; i++ {
		h.Call(http.MethodPost, "/api/gateway/v1/otp/verify", token,
			map[string]any{"numero": "771000001", "otpCode": "000000"})
	}

	// The fourth attempt is refused even with the right code.
	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/otp/verify", token,
		map[string]any{"numero": "771000001", "otpCode": "123456"})
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestOTPExpired(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	token := h.Token("yas", "yas2026")
	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000001"})

	_, err := h.DB.Pool.Exec(context.Background(),
		"UPDATE otp SET expire_a = now() - interval '1 minute' WHERE numero = $1", "771000001")
	require.NoError(t, err)

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/otp/verify", token,
		map[string]any{"numero": "771000001", "otpCode": "123456"})

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "Le code OTP a expiré", body["detail"])
}

func TestOTPMissing(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/otp/verify",
		h.Token("yas", "yas2026"),
		map[string]any{"numero": "779999999", "otpCode": "123456"})

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "Aucun OTP actif pour ce numéro", body["detail"])
}

func TestOTPInvalidNumeroIsAValidationError(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/otp/send",
		h.Token("yas", "yas2026"), map[string]any{"numero": "77"})

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Contains(t, body, "fieldErrors")
}

func TestOTPResendResetsCounterAndConsumption(t *testing.T) {
	// The brief requires that resending an OTP for an already-covered number
	// replace the code and reset tentatives/consomme to zero/false (ON CONFLICT clause).
	h := routerharness.NewRouterHarness(t)
	token := h.Token("yas", "yas2026")

	h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000001"})

	// Two failed attempts start the counter without exhausting it.
	for i := 0; i < 2; i++ {
		h.Call(http.MethodPost, "/api/gateway/v1/otp/verify", token,
			map[string]any{"numero": "771000001", "otpCode": "000000"})
	}

	var attempts int
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		"SELECT tentatives FROM otp WHERE numero = $1", "771000001").Scan(&attempts))
	require.Equal(t, 2, attempts, "precondition: the counter has indeed started")

	// The resend must reset tentatives and consomme.
	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000001"})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var consumed bool
	require.NoError(t, h.DB.Pool.QueryRow(context.Background(),
		"SELECT tentatives, consomme FROM otp WHERE numero = $1", "771000001").
		Scan(&attempts, &consumed))
	require.Equal(t, 0, attempts, "the resend must reset the counter to zero")
	require.False(t, consumed, "the resend must reset consomme to false")

	// The code stays verifiable after the resend.
	resp, _ = h.Call(http.MethodPost, "/api/gateway/v1/otp/verify", token,
		map[string]any{"numero": "771000001", "otpCode": "123456"})
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
