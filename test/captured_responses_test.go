package test

import (
	"context"
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

// requireClient checks an individual's client sub-object as the platform
// renders it, with exactly these six fields.
func requireClient(t *testing.T, dto map[string]any) {
	t.Helper()
	requireClientFields(t, dto,
		"nom", "prenom", "dateNaissance", "lieuNaissance", "typePiece", "numeroPiece")
}

// requireEnterpriseClient checks a fleet's client sub-object: 2026-09-18
// captures (« rec-numflex.artp.sn »), seven fields — raisonSociale and numRC
// on top of the individual's, and no lieuNaissance, which an enterprise
// creation does not receive.
func requireEnterpriseClient(t *testing.T, dto map[string]any) {
	t.Helper()
	requireClientFields(t, dto,
		"nom", "prenom", "dateNaissance", "typePiece", "numeroPiece", "raisonSociale", "numRC")
}

func requireClientFields(t *testing.T, dto map[string]any, fields ...string) {
	t.Helper()
	client, ok := dto["client"].(map[string]any)
	require.Truef(t, ok, "the DTO carries no client: %v", dto)
	for _, field := range fields {
		require.Containsf(t, client, field, "client.%s missing", field)
	}
	require.Len(t, client, len(fields), "the client must carry only the measured fields")
}

// requireFleet checks the shape of an ENTREPRISE request in every response
// captured on 2026-09-18: numeros in place of numero, every other field
// identical to an individual's.
func requireFleet(t *testing.T, dto map[string]any, numbers ...string) {
	t.Helper()
	require.Equal(t, "ENTREPRISE", dto["typeAbonne"])
	require.NotContains(t, dto, "numero")
	want := make([]any, 0, len(numbers))
	for _, n := range numbers {
		want = append(want, n)
	}
	require.Equal(t, want, dto["numeros"])
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

// javaInstantPattern: the fraction of a second in groups of three digits, as
// java.time.Instant writes it — « .580Z », never « .58Z ».
const javaInstantPattern = `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.(\d{3}|\d{6}|\d{9}))?Z$`

// millisecondPattern: at most three decimals.
const millisecondPattern = `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{3})?Z$`

// requireFresh checks that a timestamp rendered by the request that wrote it
// was not truncated: it equals the stored value exactly (at the microsecond,
// Postgres's precision) and follows the Java format.
func requireFresh(t *testing.T, h *harness, rendered, column, id string) {
	t.Helper()
	require.Regexp(t, javaInstantPattern, rendered)
	parsed, err := time.Parse(time.RFC3339Nano, rendered)
	require.NoError(t, err)
	var stored time.Time
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT "+column+" FROM demande WHERE id = $1", id).Scan(&stored))
	require.True(t, parsed.Truncate(time.Microsecond).Equal(stored.Truncate(time.Microsecond)),
		"%s rendered %s, stored %s: the value the request wrote comes out untruncated",
		column, rendered, stored.Format(time.RFC3339Nano))
}

// Captures « yas-1 Créer une demande … particulier » (2026-08-27,
// « 22:39:23.583043149Z ») and « POST /demandes/entreprise » (2026-09-18,
// « 13:13:50.137967145Z »): creation renders dateDemande to the nanosecond —
// the object the platform has just built — then every read-back renders it to
// the millisecond (« 22:39:23.583Z »), Mongo's precision.
func TestCaptureRequestDateNanosecondOnCreationMillisecondAfter(t *testing.T) {
	h := newHarness(t)
	yas := h.token("yas", "yas2026")
	orange := h.token("orange", "orange2026")

	h.call(http.MethodPost, "/api/gateway/v1/otp/send", yas, map[string]any{"numero": "771000001"})
	_, body := h.call(http.MethodPost, "/api/gateway/v1/demandes/particulier", yas,
		individualBody("771000001"))
	dto := body["data"].(map[string]any)
	requireFresh(t, h, dto["dateDemande"].(string), "date_demande", dto["id"].(string))

	_, reread := h.call(http.MethodGet, "/api/gateway/v1/demandes/a-accepter/"+dto["id"].(string), orange, nil)
	require.Regexp(t, millisecondPattern, reread["data"].(map[string]any)["dateDemande"])

	h.call(http.MethodPost, "/api/gateway/v1/otp/send", yas, map[string]any{"numero": "771000002"})
	_, fleetBody := h.call(http.MethodPost, "/api/gateway/v1/demandes/entreprise", yas,
		enterpriseBody("771000002", []string{"771000002", "771000003"}))
	fleet := fleetBody["data"].(map[string]any)["demande"].(map[string]any)
	requireFresh(t, h, fleet["dateDemande"].(string), "date_demande", fleet["id"].(string))

	_, rereadFleet := h.call(http.MethodGet, "/api/gateway/v1/demandes/a-accepter/"+fleet["id"].(string), orange, nil)
	require.Regexp(t, millisecondPattern, rereadFleet["data"].(map[string]any)["dateDemande"])
}

// Captures « 2_yas_confirmer-a COMPLETION » (2026-08-27, dateFinalisation
// « 23:14:55.794666796Z ») and enterprise COMPLETION processing (2026-09-18,
// « 13:26:56.488082853Z »): the request closing the porting renders
// dateFinalisation to the nanosecond and dateDemande — read back — to the
// millisecond; the « in » capture then reads both back to the millisecond.
func TestCaptureCompletionDateNanosecondOnCompletion(t *testing.T) {
	h := newHarness(t)
	id := h.createFleet("771000001", []string{"771000001", "771000002"})
	h.advanceTo(id, "COMPLETION")

	_, body := h.call(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.token("yas", "yas2026"), map[string]any{"idDemande": id})
	dto := body["data"].(map[string]any)
	require.Equal(t, "TERMINE", dto["statutDemande"])
	requireFresh(t, h, dto["dateFinalisation"].(string), "date_finalisation", id)
	require.Regexp(t, millisecondPattern, dto["dateDemande"])

	in := h.list("/api/gateway/v1/demandes/in", h.token("yas", "yas2026"))
	require.Len(t, in, 1)
	require.Regexp(t, millisecondPattern, in[0].(map[string]any)["dateDemande"])
	require.Regexp(t, millisecondPattern, in[0].(map[string]any)["dateFinalisation"])
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

// --- Enterprise captures, 2026-09-18 -----------------------------------------
//
// A fleet goes through the same endpoints as an individual; only the shape of
// the request changes: numeros[] in place of numero, and a seven-field client.
// The captures cover creation, a-accepter, a-traiter at each step, fleet
// acceptance, processing and confirmation.

// Capture « POST /demandes/entreprise », 201: « Demande entreprise créée avec
// succès », data.demande in full — the same shape as the queues.
func TestCaptureFleetCreation(t *testing.T) {
	h := newHarness(t)
	token := h.token("yas", "yas2026")
	h.call(http.MethodPost, "/api/gateway/v1/otp/send", token, map[string]any{"numero": "771000001"})

	resp, body := h.call(http.MethodPost, "/api/gateway/v1/demandes/entreprise", token,
		enterpriseBody("771000001", []string{"771000001", "771000002"}))

	require.Equal(t, http.StatusCreated, resp.StatusCode, body)
	require.Equal(t, "Demande entreprise créée avec succès", body["message"])
	data := body["data"].(map[string]any)
	require.Equal(t, float64(2), data["numerosPortesCount"])
	require.Equal(t, float64(0), data["numerosExclusCount"])
	require.Equal(t, []any{}, data["numerosExclus"])

	dto := data["demande"].(map[string]any)
	requireFleet(t, dto, "771000001", "771000002")
	requireEnterpriseClient(t, dto)
	for _, field := range []string{
		"id", "typeDemande", "statutDemande", "etapeActuelle", "statutEtapeActuel",
		"operateurSource", "operateurDestinataire", "dateDemande", "processus", "routageInfo",
	} {
		require.Containsf(t, dto, field, "demande.%s missing", field)
	}
}

// Captures « a-accepter » and « a-traiter »: the fleet shows there with
// numeros and its enterprise client, in both of the source's queues.
func TestCaptureFleetInQueues(t *testing.T) {
	h := newHarness(t)
	id := h.createFleet("771000001", []string{"771000001", "771000002"})
	token := h.token("orange", "orange2026")

	for _, path := range []string{"a-accepter", "a-traiter"} {
		data := h.list("/api/gateway/v1/demandes/"+path, token)
		require.Len(t, data, 1, path)
		dto := data[0].(map[string]any)
		require.Equal(t, id, dto["id"])
		requireFleet(t, dto, "771000001", "771000002")
		requireEnterpriseClient(t, dto)

		_, detail := h.call(http.MethodGet, "/api/gateway/v1/demandes/"+path+"/"+id, token, nil)
		requireFleet(t, detail["data"].(map[string]any), "771000001", "771000002")
		requireEnterpriseClient(t, detail["data"].(map[string]any))
	}
}

// Capture « POST /demandes/{id}/acceptation »: « Décision d'acceptation
// enregistrée », the full request at DESACTIVATION, enterprise client.
func TestCaptureFleetAcceptance(t *testing.T) {
	h := newHarness(t)
	id := h.createFleet("771000001", []string{"771000001", "771000002"})

	resp, body := h.call(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/acceptation",
		h.token("orange", "orange2026"),
		map[string]any{"accepte": true, "commentaire": "Flotte vérifiée, tout est conforme"})

	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	require.Equal(t, "Décision d'acceptation enregistrée", body["message"])
	dto := body["data"].(map[string]any)
	require.Equal(t, "DESACTIVATION", dto["etapeActuelle"])
	requireFleet(t, dto, "771000001", "771000002")
	requireEnterpriseClient(t, dto)
}

// Captures « traitement » (DESACTIVATION, ACTIVATION, COMPLETION): the same
// envelope as an individual, the request rendered with numeros and the
// enterprise client; at COMPLETION, dateFinalisation and TERMINE.
func TestCaptureFleetProcessing(t *testing.T) {
	h := newHarness(t)
	id := h.createFleet("771000001", []string{"771000001", "771000002"})

	h.advanceTo(id, "DESACTIVATION")
	resp, body := h.call(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.token("orange", "orange2026"),
		map[string]any{"idDemande": id, "commentaire": "Numéro désactivé avec succès"})
	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	require.Equal(t, "Étape traitée avec succès", body["message"])
	dto := body["data"].(map[string]any)
	require.Equal(t, "ACTIVATION", dto["etapeActuelle"])
	requireFleet(t, dto, "771000001", "771000002")
	requireEnterpriseClient(t, dto)

	h.advanceTo(id, "COMPLETION")
	_, body = h.call(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.token("yas", "yas2026"), map[string]any{"idDemande": id})
	dto = body["data"].(map[string]any)
	require.Equal(t, "TERMINE", dto["statutDemande"])
	require.Equal(t, "TERMINE", dto["statutEtapeActuel"])
	require.Contains(t, dto, "dateFinalisation")
	requireFleet(t, dto, "771000001", "771000002")
	requireEnterpriseClient(t, dto)
}

// Capture « POST /demandes/a-confirmer » on a fleet: numeros present, client
// absent — the same asymmetry as on an individual.
func TestCaptureFleetConfirmationWithoutClient(t *testing.T) {
	h := newHarness(t)
	id := h.createFleet("771000001", []string{"771000001", "771000002"})
	h.advanceTo(id, "CONFIRMATION")
	token := h.token("orange", "orange2026")

	data := h.list("/api/gateway/v1/demandes/a-confirmer", token)
	require.Len(t, data, 1)
	requireFleet(t, data[0].(map[string]any), "771000001", "771000002")
	require.NotContains(t, data[0].(map[string]any), "client")

	resp, body := h.call(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer", token,
		map[string]any{"idDemande": id, "commentaire": "Portage confirmé"})
	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	dto := body["data"].(map[string]any)
	requireFleet(t, dto, "771000001", "771000002")
	require.NotContains(t, dto, "client")
}

// 2026-09-18 capture: numerosFlotte carried four entries, three of them the
// same number; the platform rendered two distinct numbers in numeros but
// numerosPortesCount: 4. The count follows the list received, not the
// retained fleet — reproduced, not corrected.
func TestCaptureFleetDuplicatesDeduplicated(t *testing.T) {
	h := newHarness(t)
	token := h.token("yas", "yas2026")
	h.call(http.MethodPost, "/api/gateway/v1/otp/send", token, map[string]any{"numero": "771000001"})

	resp, body := h.call(http.MethodPost, "/api/gateway/v1/demandes/entreprise", token,
		enterpriseBody("771000001", []string{"771000002", "771000003", "771000003", "771000003"}))

	require.Equal(t, http.StatusCreated, resp.StatusCode, body)
	data := body["data"].(map[string]any)
	requireFleet(t, data["demande"].(map[string]any), "771000002", "771000003")
	require.Equal(t, float64(4), data["numerosPortesCount"])
	require.Equal(t, float64(0), data["numerosExclusCount"])
}

// Numbers come out in the order the fleet declared them, not in
// lexicographic order.
func TestCaptureFleetNumbersInDeclaredOrder(t *testing.T) {
	h := newHarness(t)
	id := h.createFleet("771000001", []string{"771000003", "771000001", "771000002"})

	_, body := h.call(http.MethodGet, "/api/gateway/v1/demandes/a-accepter/"+id,
		h.token("orange", "orange2026"), nil)
	requireFleet(t, body["data"].(map[string]any), "771000003", "771000001", "771000002")
}
