package test

import (
	"net/http"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/stretchr/testify/require"
)

// Moved from internal/api/conformite_captures_test.go (Task 18):
// internal/api is deleted, and this file's own harness — newHarness,
// token, call, list, advanceTo, createPorting, converge, step — now lives
// in test/harness_test.go instead. Renamed only in the final task.
//
// This file freezes the responses actually recorded against the ARTP platform
// on 2026-08-27, kept in « Num Flex API.postman_collection.json ».
//
// They outrank the guide's examples: these are captures, not illustrations.
// They are told apart from the same collection's hand-written examples by their
// identifiers — captures carry real ObjectIds (`6a90bc9bad2131073eddbbdc`,
// operators `6a21745c…` / `6a2174c3…`) and nanosecond timestamps, where the
// examples carry `65abc111111111` and « Orange Sénégal ».

// requireClient is the client sub-object as the platform renders it, with
// exactly these six fields.
func requireClient(t *testing.T, dto map[string]any) {
	t.Helper()
	client, ok := dto["client"].(map[string]any)
	require.Truef(t, ok, "the DTO carries no client: %v", dto)
	for _, field := range []string{
		"nom", "prenom", "dateNaissance", "lieuNaissance", "typePiece", "numeroPiece",
	} {
		require.Containsf(t, client, field, "client.%s missing", field)
	}
	require.Len(t, client, 6, "the client must carry only the six measured fields")
}

// Capture « yas-1 Créer une demande de portage — abonné particulier », 201.
func TestCaptureIndividualCreation(t *testing.T) {
	h := newHarness(t)
	token := h.token("yas", "yas2026")
	h.call(http.MethodPost, "/api/gateway/v1/otp/send", token,
		map[string]any{"numero": "771000001"})

	resp, body := h.call(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		token, individualBody("771000001"))

	require.Equal(t, http.StatusCreated, resp.StatusCode, body)
	require.Equal(t, "Demande particulier créée avec succès", body["message"])
	requireClient(t, body["data"].(map[string]any))
}

// Capture « 1. orange_2_ACCEPTATION Accepter ou rejeter une demande », 200.
func TestCaptureAcceptance(t *testing.T) {
	h := newHarness(t)
	id := h.createPorting("771000001")

	resp, body := h.call(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.token("orange", "orange2026"),
		map[string]any{"idDemande": id, "accepte": true,
			"commentaire": "Numéro validé, portage autorisé"})

	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	require.Equal(t, "Décision d'acceptation enregistrée", body["message"])
	requireClient(t, body["data"].(map[string]any))
}

// Capture « 1.orange1_EN_COURS_Demandes à traiter_next_ACCEPTATION »: a request
// at the ACCEPTATION step does appear in the source's a-traiter. The queue
// answers « nécessitant une action de votre part » (§7.7), not a subset of
// steps.
func TestCaptureToProcessIncludesAcceptance(t *testing.T) {
	h := newHarness(t)
	id := h.createPorting("771000001")

	data := h.list("/api/gateway/v1/demandes/a-traiter", h.token("orange", "orange2026"))
	require.Len(t, data, 1)
	dto := data[0].(map[string]any)
	require.Equal(t, id, dto["id"])
	require.Equal(t, "ACCEPTATION", dto["etapeActuelle"])
	requireClient(t, dto)

	// The recipient has nothing to process at this step.
	require.Empty(t, h.list("/api/gateway/v1/demandes/a-traiter", h.token("yas", "yas2026")))
}

// Captures « 1.orange_CONFIRMATION_Demandes à confirmer » and
// « 1_orange_Confirmer une demande »: neither the queue nor the confirmation
// response carries a client, where every other one does. The asymmetry is
// measured across four captures; it is not a recording oversight.
func TestCaptureConfirmationWithoutClient(t *testing.T) {
	h := newHarness(t)
	id := h.createPorting("771000001")
	h.advanceTo(id, "CONFIRMATION")

	token := h.token("orange", "orange2026")

	data := h.list("/api/gateway/v1/demandes/a-confirmer", token)
	require.Len(t, data, 1)
	require.NotContains(t, data[0].(map[string]any), "client")

	_, detail := h.call(http.MethodGet, "/api/gateway/v1/demandes/a-confirmer/"+id, token, nil)
	require.NotContains(t, detail["data"].(map[string]any), "client")

	resp, body := h.call(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer", token,
		map[string]any{"idDemande": id, "commentaire": "Portage confirmé"})
	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	require.NotContains(t, body["data"].(map[string]any), "client")
}

// Captures « in » and « 2_yas_confirmer-a COMPLETION »: a finished request
// carries statutEtapeActuel TERMINE, not VALIDE. ANO-013 already said so —
// TERMINE nominally, EXPIRE on expiry — and the capture confirms it.
func TestCaptureCompletedRequestCarriesTermine(t *testing.T) {
	h := newHarness(t)
	id := h.createPorting("771000001")
	h.advanceTo(id, "COMPLETION")

	h.call(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.token("yas", "yas2026"), map[string]any{"idDemande": id})
	h.converge()

	data := h.list("/api/gateway/v1/demandes/in", h.token("yas", "yas2026"))
	require.Len(t, data, 1)
	dto := data[0].(map[string]any)
	require.Equal(t, "TERMINE", dto["statutDemande"])
	require.Equal(t, "TERMINE", dto["statutEtapeActuel"])
	require.Contains(t, dto, "dateFinalisation")
	requireClient(t, dto)
}

// Every capture renders timestamps to the millisecond
// (« 2026-08-27T22:39:23.583Z »), not to the microsecond.
func TestCaptureTimestampsInMilliseconds(t *testing.T) {
	h := newHarness(t)
	id := h.createPorting("771000001")

	_, body := h.call(http.MethodGet, "/api/gateway/v1/demandes/a-accepter/"+id,
		h.token("orange", "orange2026"), nil)

	date := body["data"].(map[string]any)["dateDemande"].(string)
	require.Regexp(t, `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{1,3})?Z$`, date,
		"the platform renders millisecond timestamps")
}

// Captures « 1.orange_3_DESACTIVATION…_next_ACTIVATION » and
// « 1. yas_4_ACTIVATION…_next_CONFIRMATION »: the response carries the NEXT
// step. The transition is applied within the request.
func TestCaptureProcessingRendersNextStep(t *testing.T) {
	h := newHarness(t) // zero convergence: the captures' profile
	id := h.createPorting("771000001")
	h.advanceTo(id, "DESACTIVATION")

	_, body := h.call(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.token("orange", "orange2026"),
		map[string]any{"idDemande": id, "commentaire": "Numéro désactivé avec succès"})

	data := body["data"].(map[string]any)
	require.Equal(t, "ACTIVATION", data["etapeActuelle"])
	require.Equal(t, "EN_COURS", data["statutEtapeActuel"])

	// Acceptance follows the same rule: the capture renders DESACTIVATION.
	other := h.createPorting("771000002")
	_, otherBody := h.call(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.token("orange", "orange2026"),
		map[string]any{"idDemande": other, "accepte": true})
	require.Equal(t, "DESACTIVATION",
		otherBody["data"].(map[string]any)["etapeActuelle"])
}

// The behaviour measured at SIT v0.3 (R-10) stays reachable: a non-zero
// convergence window renders the previous step, and the switch happens later.
// Both measurements therefore stay reproducible — 2026-08-27's by default, the
// SIT's on demand.
func TestNonZeroConvergenceRestoresSITBehaviour(t *testing.T) {
	h := newHarness(t, func(c *config.Config) {
		c.ConvergenceMin = 30 * time.Second
		c.ConvergenceMax = 30 * time.Second
	})
	id := h.createPorting("771000001")
	h.advanceTo(id, "DESACTIVATION")

	_, body := h.call(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.token("orange", "orange2026"), map[string]any{"idDemande": id})

	require.Equal(t, "DESACTIVATION",
		body["data"].(map[string]any)["etapeActuelle"],
		"non-zero convergence window: the response carries the previous step (R-10)")
	require.Equal(t, "DESACTIVATION", h.step(id))
}
