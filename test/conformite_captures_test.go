package test

import (
	"net/http"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/stretchr/testify/require"
)

// Moved from internal/api/conformite_captures_test.go (Task 18):
// internal/api is deleted, and this file's own harnais — nouveauHarnais,
// jeton, appel, liste, avancerA, creerPortage, converger, etape — now lives
// in test/harnais_test.go instead. Renamed only in the final task.
//
// This file freezes the responses actually recorded against the ARTP platform
// on 2026-08-27, kept in « Num Flex API.postman_collection.json ».
//
// They outrank the guide's examples: these are captures, not illustrations.
// They are told apart from the same collection's hand-written examples by their
// identifiers — captures carry real ObjectIds (`6a90bc9bad2131073eddbbdc`,
// operators `6a21745c…` / `6a2174c3…`) and nanosecond timestamps, where the
// examples carry `65abc111111111` and « Orange Sénégal ».

// clientAttendu is the client sub-object as the platform renders it, with
// exactly these six fields.
func exigeClient(t *testing.T, dto map[string]any) {
	t.Helper()
	client, ok := dto["client"].(map[string]any)
	require.Truef(t, ok, "le DTO ne porte pas de client : %v", dto)
	for _, champ := range []string{
		"nom", "prenom", "dateNaissance", "lieuNaissance", "typePiece", "numeroPiece",
	} {
		require.Containsf(t, client, champ, "client.%s manquant", champ)
	}
	require.Len(t, client, 6, "le client ne doit porter que les six champs mesurés")
}

// Capture « yas-1 Créer une demande de portage — abonné particulier », 201.
func TestCaptureCreationParticulier(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		jeton, corpsParticulier("771000001"))

	require.Equal(t, http.StatusCreated, rep.StatusCode, corps)
	require.Equal(t, "Demande particulier créée avec succès", corps["message"])
	exigeClient(t, corps["data"].(map[string]any))
}

// Capture « 1. orange_2_ACCEPTATION Accepter ou rejeter une demande », 200.
func TestCaptureAcceptation(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.jeton("orange", "orange2026"),
		map[string]any{"idDemande": id, "accepte": true,
			"commentaire": "Numéro validé, portage autorisé"})

	require.Equal(t, http.StatusOK, rep.StatusCode, corps)
	require.Equal(t, "Décision d'acceptation enregistrée", corps["message"])
	exigeClient(t, corps["data"].(map[string]any))
}

// Capture « 1.orange1_EN_COURS_Demandes à traiter_next_ACCEPTATION »: a request
// at the ACCEPTATION step does appear in the source's a-traiter. The queue
// answers « nécessitant une action de votre part » (§7.7), not a subset of
// steps.
func TestCaptureATraiterInclutAcceptation(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	data := h.liste("/api/gateway/v1/demandes/a-traiter", h.jeton("orange", "orange2026"))
	require.Len(t, data, 1)
	dto := data[0].(map[string]any)
	require.Equal(t, id, dto["id"])
	require.Equal(t, "ACCEPTATION", dto["etapeActuelle"])
	exigeClient(t, dto)

	// The recipient has nothing to process at this step.
	require.Empty(t, h.liste("/api/gateway/v1/demandes/a-traiter", h.jeton("yas", "yas2026")))
}

// Captures « 1.orange_CONFIRMATION_Demandes à confirmer » and
// « 1_orange_Confirmer une demande »: neither the queue nor the confirmation
// response carries a client, where every other one does. The asymmetry is
// measured across four captures; it is not a recording oversight.
func TestCaptureConfirmationSansClient(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "CONFIRMATION")

	jeton := h.jeton("orange", "orange2026")

	data := h.liste("/api/gateway/v1/demandes/a-confirmer", jeton)
	require.Len(t, data, 1)
	require.NotContains(t, data[0].(map[string]any), "client")

	_, detail := h.appel(http.MethodGet, "/api/gateway/v1/demandes/a-confirmer/"+id, jeton, nil)
	require.NotContains(t, detail["data"].(map[string]any), "client")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer", jeton,
		map[string]any{"idDemande": id, "commentaire": "Portage confirmé"})
	require.Equal(t, http.StatusOK, rep.StatusCode, corps)
	require.NotContains(t, corps["data"].(map[string]any), "client")
}

// Captures « in » and « 2_yas_confirmer-a COMPLETION »: a finished request
// carries statutEtapeActuel TERMINE, not VALIDE. ANO-013 already said so —
// TERMINE nominally, EXPIRE on expiry — and the capture confirms it.
func TestCaptureDemandeAcheveePorteTermine(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "COMPLETION")

	h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("yas", "yas2026"), map[string]any{"idDemande": id})
	h.converger()

	data := h.liste("/api/gateway/v1/demandes/in", h.jeton("yas", "yas2026"))
	require.Len(t, data, 1)
	dto := data[0].(map[string]any)
	require.Equal(t, "TERMINE", dto["statutDemande"])
	require.Equal(t, "TERMINE", dto["statutEtapeActuel"])
	require.Contains(t, dto, "dateFinalisation")
	exigeClient(t, dto)
}

// Every capture renders timestamps to the millisecond
// (« 2026-08-27T22:39:23.583Z »), not to the microsecond.
func TestCaptureHorodatagesEnMillisecondes(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	_, corps := h.appel(http.MethodGet, "/api/gateway/v1/demandes/a-accepter/"+id,
		h.jeton("orange", "orange2026"), nil)

	date := corps["data"].(map[string]any)["dateDemande"].(string)
	require.Regexp(t, `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{1,3})?Z$`, date,
		"la plateforme rend des horodatages à la milliseconde")
}

// Captures « 1.orange_3_DESACTIVATION…_next_ACTIVATION » and
// « 1. yas_4_ACTIVATION…_next_CONFIRMATION »: the response carries the NEXT
// step. The transition is applied within the request.
func TestCaptureTraitementRendLEtapeSuivante(t *testing.T) {
	h := nouveauHarnais(t) // convergence nulle : le profil des captures
	id := h.creerPortage("771000001")
	h.avancerA(id, "DESACTIVATION")

	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("orange", "orange2026"),
		map[string]any{"idDemande": id, "commentaire": "Numéro désactivé avec succès"})

	data := corps["data"].(map[string]any)
	require.Equal(t, "ACTIVATION", data["etapeActuelle"])
	require.Equal(t, "EN_COURS", data["statutEtapeActuel"])

	// Acceptance follows the same rule: the capture renders DESACTIVATION.
	autre := h.creerPortage("771000002")
	_, corpsAcc := h.appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.jeton("orange", "orange2026"),
		map[string]any{"idDemande": autre, "accepte": true})
	require.Equal(t, "DESACTIVATION",
		corpsAcc["data"].(map[string]any)["etapeActuelle"])
}

// The behaviour measured at SIT v0.3 (R-10) stays reachable: a non-zero
// convergence window renders the previous step, and the switch happens later.
// Both measurements therefore stay reproducible — 2026-08-27's by default, the
// SIT's on demand.
func TestConvergenceNonNulleRestaureLeComportementDuSIT(t *testing.T) {
	h := nouveauHarnais(t, func(c *config.Config) {
		c.ConvergenceMin = 30 * time.Second
		c.ConvergenceMax = 30 * time.Second
	})
	id := h.creerPortage("771000001")
	h.avancerA(id, "DESACTIVATION")

	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})

	require.Equal(t, "DESACTIVATION",
		corps["data"].(map[string]any)["etapeActuelle"],
		"fenêtre de convergence non nulle : la réponse porte l'étape précédente (R-10)")
	require.Equal(t, "DESACTIVATION", h.etape(id))
}
