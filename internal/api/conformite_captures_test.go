package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/stretchr/testify/require"
)

// Ce fichier fige les réponses réellement enregistrées contre la plateforme
// ARTP le 2026-08-27, conservées dans « Num Flex API.postman_collection.json ».
//
// Elles priment sur les exemples du guide : ce sont des captures, pas des
// illustrations. On les distingue des exemples écrits à la main de la même
// collection par leurs identifiants — les captures portent de vrais ObjectId
// (`6a90bc9bad2131073eddbbdc`, opérateurs `6a21745c…` / `6a2174c3…`) et des
// horodatages à la nanoseconde, là où les exemples portent `65abc111111111`
// et « Orange Sénégal ».

// clientAttendu est le sous-objet client tel que la plateforme le rend, avec
// exactement ces six champs.
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

// Capture « 1.orange1_EN_COURS_Demandes à traiter_next_ACCEPTATION » : une
// demande à l'étape ACCEPTATION figure dans a-traiter de la source. La file
// répond à « nécessitant une action de votre part » (§7.7), pas à un
// sous-ensemble d'étapes.
func TestCaptureATraiterInclutAcceptation(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	data := h.liste("/api/gateway/v1/demandes/a-traiter", h.jeton("orange", "orange2026"))
	require.Len(t, data, 1)
	dto := data[0].(map[string]any)
	require.Equal(t, id, dto["id"])
	require.Equal(t, "ACCEPTATION", dto["etapeActuelle"])
	exigeClient(t, dto)

	// Le destinataire n'a rien à traiter à cette étape.
	require.Empty(t, h.liste("/api/gateway/v1/demandes/a-traiter", h.jeton("yas", "yas2026")))
}

// Captures « 1.orange_CONFIRMATION_Demandes à confirmer » et
// « 1_orange_Confirmer une demande » : ni la file ni la réponse de confirmation
// ne portent de client, là où toutes les autres en portent un. L'asymétrie est
// mesurée sur quatre captures, elle n'est pas un oubli d'enregistrement.
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

// Captures « in » et « 2_yas_confirmer-a COMPLETION » : une demande achevée
// porte statutEtapeActuel TERMINE, pas VALIDE. ANO-013 le disait déjà —
// TERMINE en nominal, EXPIRE par expiration — et la capture le confirme.
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

// Toutes les captures rendent les horodatages à la milliseconde
// (« 2026-08-27T22:39:23.583Z »), pas à la microseconde.
func TestCaptureHorodatagesEnMillisecondes(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	_, corps := h.appel(http.MethodGet, "/api/gateway/v1/demandes/a-accepter/"+id,
		h.jeton("orange", "orange2026"), nil)

	date := corps["data"].(map[string]any)["dateDemande"].(string)
	require.Regexp(t, `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{1,3})?Z$`, date,
		"la plateforme rend des horodatages à la milliseconde")
}

// Captures « 1.orange_3_DESACTIVATION…_next_ACTIVATION » et
// « 1. yas_4_ACTIVATION…_next_CONFIRMATION » : la reponse porte l'etape
// SUIVANTE. La transition est appliquee dans la requete.
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

	// L'acceptation suit la même règle : la capture rend DESACTIVATION.
	autre := h.creerPortage("771000002")
	_, corpsAcc := h.appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.jeton("orange", "orange2026"),
		map[string]any{"idDemande": autre, "accepte": true})
	require.Equal(t, "DESACTIVATION",
		corpsAcc["data"].(map[string]any)["etapeActuelle"])
}

// Le comportement mesuré au SIT v0.3 (R-10) reste atteignable : une fenêtre de
// convergence non nulle rend l'étape précédente, et la bascule survient plus
// tard. Les deux mesures restent donc reproductibles, celle de 2026-08-27 par
// défaut et celle du SIT sur demande.
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
