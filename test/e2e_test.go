package test

import (
	"context"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/seed"
	"github.com/stretchr/testify/require"
)

// TestPortingCompleteUntilTermine replays §10 of the guide: ORANGE → YAS, with
// EXPRESSO's confirmation, through to TERMINE and appearance in /in and /out.
func TestPortingCompleteUntilTermine(t *testing.T) {
	h := newHarness(t) // deterministic profile: zero convergence and latency

	yas := h.token("yas", "yas2026")
	orange := h.token("orange", "orange2026")
	expresso := h.token("expresso", "expresso2026")

	// 1-2. OTP then creation by the recipient.
	h.post("/api/gateway/v1/otp/send", yas, map[string]any{"numero": "771000001"})
	_, body := h.post("/api/gateway/v1/demandes/particulier", yas, individualBody("771000001"))
	id := body["data"].(map[string]any)["id"].(string)

	// 3. The source sees the request in its queue.
	require.Len(t, h.list("/api/gateway/v1/demandes/a-accepter", orange), 1)

	// 4. Acceptance by the source, then convergence.
	h.post("/api/gateway/v1/demandes/acceptation", orange,
		map[string]any{"idDemande": id, "accepte": true})
	h.converge()
	require.Equal(t, "DESACTIVATION", h.step(id))

	// 5. Deactivation by the source.
	h.post("/api/gateway/v1/demandes/traitement", orange, map[string]any{"idDemande": id})
	h.converge()
	require.Equal(t, "ACTIVATION", h.step(id))

	// 6. Activation by the recipient.
	h.post("/api/gateway/v1/demandes/traitement", yas, map[string]any{"idDemande": id})
	h.converge()
	require.Equal(t, "CONFIRMATION", h.step(id))

	// 7. Confirmation: the source, then the third party. One alone is not enough.
	h.post("/api/gateway/v1/demandes/a-confirmer", orange, map[string]any{"idDemande": id})
	h.converge()
	require.Equal(t, "CONFIRMATION", h.step(id), "EXPRESSO's confirmation is still missing")

	h.post("/api/gateway/v1/demandes/a-confirmer", expresso, map[string]any{"idDemande": id})
	h.converge()
	require.Equal(t, "COMPLETION", h.step(id))

	// 8. Closure by the recipient.
	h.post("/api/gateway/v1/demandes/traitement", yas, map[string]any{"idDemande": id})
	h.converge()

	require.Equal(t, "TERMINE", h.requestStatus(id))

	// The request appears on both sides.
	incoming := h.list("/api/gateway/v1/demandes/in", yas)
	require.Len(t, incoming, 1)
	require.Equal(t, id, incoming[0].(map[string]any)["id"])

	outgoing := h.list("/api/gateway/v1/demandes/out", orange)
	require.Len(t, outgoing, 1)

	// The number changed operator in the national registry.
	require.Equal(t, seed.OperatorYASID, h.holder("771000001"))
}

// TestPortingByExpirationWithoutAnyCall replays porting #2 of the SIT: created,
// left without action, TERMINE after five step expirations (ANO-006, TC-062).
func TestPortingByExpirationWithoutAnyCall(t *testing.T) {
	h := newHarness(t, func(c *config.Config) {
		c.StepTimeout = 50 * time.Millisecond
	})

	yas := h.token("yas", "yas2026")
	h.post("/api/gateway/v1/otp/send", yas, map[string]any{"numero": "771000001"})
	_, body := h.post("/api/gateway/v1/demandes/particulier", yas, individualBody("771000001"))
	id := body["data"].(map[string]any)["id"].(string)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.engine.Run(ctx)

	require.Eventually(t, func() bool {
		return h.requestStatus(id) == "TERMINE"
	}, 5*time.Second, 20*time.Millisecond)

	require.Equal(t, "EXPIRE", h.stepStatus(id))
	require.Equal(t, seed.OperatorYASID, h.holder("771000001"),
		"the number changed operator even though no HLR was ever touched")
}

// TestSameScenarioInContractMode checks that only the presentation changes
// between the two fidelity modes: replaying the nominal scenario under
// FIDELITY=contract must reach the same terminal state as
// TestPortingCompleteUntilTermine. No assertion on HTTP codes or the envelope
// here — that is the point: the fidelity switch changes the rendering, never
// the business behaviour.
func TestSameScenarioInContractMode(t *testing.T) {
	h := newHarness(t, func(c *config.Config) { c.Fidelity = config.FidelityContract })

	yas := h.token("yas", "yas2026")
	orange := h.token("orange", "orange2026")
	expresso := h.token("expresso", "expresso2026")

	h.post("/api/gateway/v1/otp/send", yas, map[string]any{"numero": "771000001"})
	_, body := h.post("/api/gateway/v1/demandes/particulier", yas, individualBody("771000001"))
	id := body["data"].(map[string]any)["id"].(string)

	h.post("/api/gateway/v1/demandes/acceptation", orange,
		map[string]any{"idDemande": id, "accepte": true})
	h.converge()
	h.post("/api/gateway/v1/demandes/traitement", orange, map[string]any{"idDemande": id})
	h.converge()
	h.post("/api/gateway/v1/demandes/traitement", yas, map[string]any{"idDemande": id})
	h.converge()
	h.post("/api/gateway/v1/demandes/a-confirmer", orange, map[string]any{"idDemande": id})
	h.post("/api/gateway/v1/demandes/a-confirmer", expresso, map[string]any{"idDemande": id})
	h.converge()
	h.post("/api/gateway/v1/demandes/traitement", yas, map[string]any{"idDemande": id})
	h.converge()

	require.Equal(t, "TERMINE", h.requestStatus(id))
}

// TestNoErrorCarriesCodeInRealMode — ANO-001, verified at volume. Each of the
// eight calls below triggers a different error situation; none of the eight
// responses may carry a code field or be enveloped.
func TestNoErrorCarriesCodeInRealMode(t *testing.T) {
	h := newHarness(t)
	yas := h.token("yas", "yas2026")
	orange := h.token("orange", "orange2026")

	unknown := "6a0000000000000000000000"
	calls := []struct {
		path  string
		token string
		body  any
	}{
		// Unknown request, on each of the four processing endpoints.
		{"/api/gateway/v1/demandes/traitement", yas, map[string]any{"idDemande": unknown}},
		{"/api/gateway/v1/demandes/acceptation", orange, map[string]any{"idDemande": unknown, "accepte": true}},
		{"/api/gateway/v1/demandes/a-confirmer", orange, map[string]any{"idDemande": unknown}},
		{"/api/gateway/v1/demandes/" + unknown + "/annuler", yas, nil},
		// Restitution of a number never ported (771000001: ORANGE by origin).
		{"/api/gateway/v1/demandes/restitution", orange, map[string]any{"numero": "771000001"}},
		// Restitution of a number ported less than 6 months ago (774000001: 2 months).
		{"/api/gateway/v1/demandes/restitution", orange, map[string]any{"numero": "774000001"}},
		// Reverse requested by the recipient (YAS) instead of the source (ORANGE).
		{"/api/gateway/v1/reverse-requests", yas, map[string]any{"numero": "773000001"}},
		// Verification of an OTP never sent.
		{"/api/gateway/v1/otp/verify", yas, map[string]any{"numero": "779999999", "otpCode": "123456"}},
	}

	for _, c := range calls {
		resp, body := h.postRaw(c.path, c.token, c.body)
		require.GreaterOrEqualf(t, resp.StatusCode, 400, c.path)
		require.NotContainsf(t, body, "code", "%s must not carry a code field", c.path)
		require.NotContainsf(t, body, "success", "%s must not be enveloped", c.path)
		require.Containsf(t, body, "type", "%s must be a problem+json", c.path)
	}
}
