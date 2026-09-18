package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/config"
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

// exigeClient vérifie le sous-objet client d'un particulier tel que la
// plateforme le rend, avec exactement ces six champs.
func exigeClient(t *testing.T, dto map[string]any) {
	t.Helper()
	exigeChampsClient(t, dto,
		"nom", "prenom", "dateNaissance", "lieuNaissance", "typePiece", "numeroPiece")
}

// exigeClientEntreprise vérifie le sous-objet client d'une flotte : captures
// du 2026-09-18 (« rec-numflex.artp.sn »), sept champs — raisonSociale et
// numRC en plus du particulier, et sans lieuNaissance, que la création
// entreprise ne reçoit pas.
func exigeClientEntreprise(t *testing.T, dto map[string]any) {
	t.Helper()
	exigeChampsClient(t, dto,
		"nom", "prenom", "dateNaissance", "typePiece", "numeroPiece", "raisonSociale", "numRC")
}

func exigeChampsClient(t *testing.T, dto map[string]any, champs ...string) {
	t.Helper()
	client, ok := dto["client"].(map[string]any)
	require.Truef(t, ok, "le DTO ne porte pas de client : %v", dto)
	for _, champ := range champs {
		require.Containsf(t, client, champ, "client.%s manquant", champ)
	}
	require.Len(t, client, len(champs), "le client ne doit porter que les champs mesurés")
}

// exigeFlotte vérifie la forme d'une demande ENTREPRISE dans toutes les
// réponses capturées le 2026-09-18 : numeros à la place de numero, tous les
// autres champs identiques au particulier.
func exigeFlotte(t *testing.T, dto map[string]any, numeros ...string) {
	t.Helper()
	require.Equal(t, "ENTREPRISE", dto["typeAbonne"])
	require.NotContains(t, dto, "numero")
	attendu := make([]any, 0, len(numeros))
	for _, n := range numeros {
		attendu = append(attendu, n)
	}
	require.Equal(t, attendu, dto["numeros"])
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

// motifInstantJava : la fraction de seconde par groupes de trois chiffres,
// comme java.time.Instant — « .580Z », jamais « .58Z ».
const motifInstantJava = `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.(\d{3}|\d{6}|\d{9}))?Z$`

// motifMilliseconde : au plus trois décimales.
const motifMilliseconde = `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{3})?Z$`

// exigeFrais vérifie qu'un horodatage rendu par la requête qui l'a écrit n'a
// pas été tronqué : il vaut exactement la valeur stockée (à la microseconde,
// précision de Postgres), et suit le format Java.
func exigeFrais(t *testing.T, h *harnais, rendu, colonne, id string) {
	t.Helper()
	require.Regexp(t, motifInstantJava, rendu)
	analyse, err := time.Parse(time.RFC3339Nano, rendu)
	require.NoError(t, err)
	var stocke time.Time
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT "+colonne+" FROM demande WHERE id = $1", id).Scan(&stocke))
	require.True(t, analyse.Truncate(time.Microsecond).Equal(stocke.Truncate(time.Microsecond)),
		"%s rendu %s, stocké %s : la valeur écrite par la requête sort sans troncature",
		colonne, rendu, stocke.Format(time.RFC3339Nano))
}

// Captures « yas-1 Créer une demande … particulier » (2026-08-27,
// « 22:39:23.583043149Z ») et « POST /demandes/entreprise » (2026-09-18,
// « 13:13:50.137967145Z ») : la création rend dateDemande à la nanoseconde —
// l'objet que la plateforme vient de construire — puis toutes les relectures
// le rendent à la milliseconde (« 22:39:23.583Z »), précision de Mongo.
func TestCaptureDateDemandeNanosecondeALaCreationMillisecondeEnsuite(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	orange := h.jeton("orange", "orange2026")

	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton, map[string]any{"numero": "771000001"})
	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/particulier", jeton,
		corpsParticulier("771000001"))
	dto := corps["data"].(map[string]any)
	exigeFrais(t, h, dto["dateDemande"].(string), "date_demande", dto["id"].(string))

	_, relu := h.appel(http.MethodGet, "/api/gateway/v1/demandes/a-accepter/"+dto["id"].(string), orange, nil)
	require.Regexp(t, motifMilliseconde, relu["data"].(map[string]any)["dateDemande"])

	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton, map[string]any{"numero": "771000002"})
	_, corpsFl := h.appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("771000002", []string{"771000002", "771000003"}))
	flotte := corpsFl["data"].(map[string]any)["demande"].(map[string]any)
	exigeFrais(t, h, flotte["dateDemande"].(string), "date_demande", flotte["id"].(string))

	_, reluFl := h.appel(http.MethodGet, "/api/gateway/v1/demandes/a-accepter/"+flotte["id"].(string), orange, nil)
	require.Regexp(t, motifMilliseconde, reluFl["data"].(map[string]any)["dateDemande"])
}

// Captures « 2_yas_confirmer-a COMPLETION » (2026-08-27,
// dateFinalisation « 23:14:55.794666796Z ») et traitement COMPLETION entreprise
// (2026-09-18, « 13:26:56.488082853Z ») : la requête qui clôt la demande rend
// dateFinalisation à la nanoseconde et dateDemande — relu — à la milliseconde ;
// la capture « in » relit ensuite les deux à la milliseconde.
func TestCaptureDateFinalisationNanosecondeALaCompletion(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerFlotte("771000001", []string{"771000001", "771000002"})
	h.avancerA(id, "COMPLETION")

	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("yas", "yas2026"), map[string]any{"idDemande": id})
	dto := corps["data"].(map[string]any)
	require.Equal(t, "TERMINE", dto["statutDemande"])
	exigeFrais(t, h, dto["dateFinalisation"].(string), "date_finalisation", id)
	require.Regexp(t, motifMilliseconde, dto["dateDemande"])

	in := h.liste("/api/gateway/v1/demandes/in", h.jeton("yas", "yas2026"))
	require.Len(t, in, 1)
	require.Regexp(t, motifMilliseconde, in[0].(map[string]any)["dateDemande"])
	require.Regexp(t, motifMilliseconde, in[0].(map[string]any)["dateFinalisation"])
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

// --- Captures entreprise, 2026-09-18 -----------------------------------------
//
// Une flotte traverse les mêmes endpoints qu'un particulier ; seule la forme
// de la demande change : numeros[] à la place de numero, et un client à sept
// champs. Les captures couvrent création, a-accepter, a-traiter à chaque
// étape, acceptation flotte, traitement et confirmation.

// Captures « a-accepter » et « a-traiter » : la flotte y figure avec numeros
// et son client entreprise, dans les deux files de la source.
func TestCaptureFlotteDansLesFiles(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerFlotte("771000001", []string{"771000001", "771000002"})
	jeton := h.jeton("orange", "orange2026")

	for _, chemin := range []string{"a-accepter", "a-traiter"} {
		data := h.liste("/api/gateway/v1/demandes/"+chemin, jeton)
		require.Len(t, data, 1, chemin)
		dto := data[0].(map[string]any)
		require.Equal(t, id, dto["id"])
		exigeFlotte(t, dto, "771000001", "771000002")
		exigeClientEntreprise(t, dto)

		_, detail := h.appel(http.MethodGet, "/api/gateway/v1/demandes/"+chemin+"/"+id, jeton, nil)
		exigeFlotte(t, detail["data"].(map[string]any), "771000001", "771000002")
		exigeClientEntreprise(t, detail["data"].(map[string]any))
	}
}

// Capture « POST /demandes/{id}/acceptation » : « Décision d'acceptation
// enregistrée », la demande complète en DESACTIVATION, client entreprise.
func TestCaptureAcceptationFlotte(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerFlotte("771000001", []string{"771000001", "771000002"})

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/acceptation",
		h.jeton("orange", "orange2026"),
		map[string]any{"accepte": true, "commentaire": "Flotte vérifiée, tout est conforme"})

	require.Equal(t, http.StatusOK, rep.StatusCode, corps)
	require.Equal(t, "Décision d'acceptation enregistrée", corps["message"])
	dto := corps["data"].(map[string]any)
	require.Equal(t, "DESACTIVATION", dto["etapeActuelle"])
	exigeFlotte(t, dto, "771000001", "771000002")
	exigeClientEntreprise(t, dto)
}

// Captures « traitement » (DESACTIVATION, ACTIVATION, COMPLETION) : même
// enveloppe que le particulier, la demande rendue avec numeros et client
// entreprise ; à la COMPLETION, dateFinalisation et TERMINE.
func TestCaptureTraitementFlotte(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerFlotte("771000001", []string{"771000001", "771000002"})

	h.avancerA(id, "DESACTIVATION")
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("orange", "orange2026"),
		map[string]any{"idDemande": id, "commentaire": "Numéro désactivé avec succès"})
	require.Equal(t, http.StatusOK, rep.StatusCode, corps)
	require.Equal(t, "Étape traitée avec succès", corps["message"])
	dto := corps["data"].(map[string]any)
	require.Equal(t, "ACTIVATION", dto["etapeActuelle"])
	exigeFlotte(t, dto, "771000001", "771000002")
	exigeClientEntreprise(t, dto)

	h.avancerA(id, "COMPLETION")
	_, corps = h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("yas", "yas2026"), map[string]any{"idDemande": id})
	dto = corps["data"].(map[string]any)
	require.Equal(t, "TERMINE", dto["statutDemande"])
	require.Equal(t, "TERMINE", dto["statutEtapeActuel"])
	require.Contains(t, dto, "dateFinalisation")
	exigeFlotte(t, dto, "771000001", "771000002")
	exigeClientEntreprise(t, dto)
}

// Capture « POST /demandes/a-confirmer » sur une flotte : numeros présent,
// client absent — la même asymétrie que sur un particulier.
func TestCaptureConfirmationFlotteSansClient(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerFlotte("771000001", []string{"771000001", "771000002"})
	h.avancerA(id, "CONFIRMATION")
	jeton := h.jeton("orange", "orange2026")

	data := h.liste("/api/gateway/v1/demandes/a-confirmer", jeton)
	require.Len(t, data, 1)
	exigeFlotte(t, data[0].(map[string]any), "771000001", "771000002")
	require.NotContains(t, data[0].(map[string]any), "client")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer", jeton,
		map[string]any{"idDemande": id, "commentaire": "Portage confirmé"})
	require.Equal(t, http.StatusOK, rep.StatusCode, corps)
	dto := corps["data"].(map[string]any)
	exigeFlotte(t, dto, "771000001", "771000002")
	require.NotContains(t, dto, "client")
}
