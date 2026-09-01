package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/config"
	"github.com/stretchr/testify/require"
)

func TestTraitementDesactivationParLaSource(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "DESACTIVATION")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("orange", "orange2026"),
		map[string]any{"idDemande": id, "commentaire": "Numéro désactivé"})

	require.Equal(t, http.StatusOK, rep.StatusCode, corps)
	require.Equal(t, "Étape traitée avec succès", corps["message"])
	// La réponse porte l'étape suivante : capture
	// « 1.orange_3_DESACTIVATION…_next_ACTIVATION ».
	require.Equal(t, "ACTIVATION", corps["data"].(map[string]any)["etapeActuelle"])
}

func TestTraitementParLeMauvaisOperateur(t *testing.T) {
	// TC-036 dans son principe : l'étape n'incombe pas à l'appelant.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "DESACTIVATION")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("yas", "yas2026"), map[string]any{"idDemande": id})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Contains(t, corps["detail"], "source")
	require.NotContains(t, corps, "code")
}

func TestTraitementRefuseAcceptationEtConfirmation(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t,
		"RuntimeException: L'étape ACCEPTATION se traite via POST /api/gateway/v1/demandes/acceptation.",
		corps["detail"])

	h.avancerA(id, "CONFIRMATION")
	rep, corps = h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t,
		"RuntimeException: L'étape CONFIRMATION se traite via POST /api/gateway/v1/demandes/a-confirmer.",
		corps["detail"])
}

func TestTraitementEnModeContratRendEtapeInvalideEn409(t *testing.T) {
	h := nouveauHarnais(t, func(c *config.Config) { c.Fidelity = config.FidelityContract })
	id := h.creerPortage("771000001")
	h.avancerA(id, "CONFIRMATION")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})

	require.Equal(t, http.StatusConflict, rep.StatusCode)
	require.Equal(t, "ETAPE_INVALIDE", corps["code"])
	require.Equal(t,
		"L'étape CONFIRMATION se traite via POST /api/gateway/v1/demandes/a-confirmer.",
		corps["message"])
}

func TestChampEtapeAccepteEtIgnoreEnSilence(t *testing.T) {
	// ANO-018 : une intégration v1 non migrée n'échoue pas — elle exécute
	// silencieusement l'étape courante, quelle qu'elle soit.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "DESACTIVATION")

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("orange", "orange2026"),
		map[string]any{"idDemande": id, "etape": "CONFIRMATION"})

	require.Equal(t, http.StatusOK, rep.StatusCode, "ni rejet, ni avertissement")

	// La ligne d'historique est écrite par le moteur au moment de la transition,
	// pas par le handler — il faut donc converger avant de la lire (R-10).
	h.converger()

	var etape, statut string
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		`SELECT etape, statut FROM etape_historique WHERE demande_id = $1`, id).
		Scan(&etape, &statut))
	require.Equal(t, "DESACTIVATION", etape, "c'est l'étape courante qui a été exécutée")
}

func TestSecondTraitementPendantLaConvergenceRefuse(t *testing.T) {
	h := nouveauHarnais(t, func(c *config.Config) {
		c.ConvergenceMin = time.Hour
		c.ConvergenceMax = time.Hour
	})
	id := h.creerPortage("771000001")
	h.avancerA(id, "DESACTIVATION")
	jeton := h.jeton("orange", "orange2026")

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement", jeton,
		map[string]any{"idDemande": id})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	rep, _ = h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement", jeton,
		map[string]any{"idDemande": id})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestCompletionReverseReserveeALARTP(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	_, err := h.db.Pool.Exec(context.Background(),
		`UPDATE demande SET type_demande = 'REVERSE', etape_actuelle = 'COMPLETION',
		                    date_debut_etape = now() WHERE id = $1`, id)
	require.NoError(t, err)

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("yas", "yas2026"), map[string]any{"idDemande": id})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t,
		"RuntimeException: La complétion (COMPLETION) d'une demande REVERSE est réservée à l'ARTP, une fois que tous les opérateurs ont confirmé.",
		corps["detail"])
}

func TestLatenceDeCompletion(t *testing.T) {
	// ANO-005 : COMPLETION répond en ~30 s. Ici réduit à 300 ms pour le test.
	h := nouveauHarnais(t, func(c *config.Config) {
		c.CompletionLatency = 300 * time.Millisecond
	})
	id := h.creerPortage("771000001")
	h.avancerA(id, "COMPLETION")

	debut := time.Now()
	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("yas", "yas2026"), map[string]any{"idDemande": id})
	ecoule := time.Since(debut)

	require.Equal(t, http.StatusOK, rep.StatusCode)
	require.GreaterOrEqual(t, ecoule, 300*time.Millisecond)
}

func TestAucuneCleDIdempotenceNEstLue(t *testing.T) {
	// ANO-005 : NumFlex n'accepte aucun Idempotency-Key ; le rejeu tombe sur
	// une erreur d'état, indiscernable d'une panne.
	h := nouveauHarnais(t, func(c *config.Config) {
		c.ConvergenceMin = time.Hour
		c.ConvergenceMax = time.Hour
	})
	id := h.creerPortage("771000001")
	h.avancerA(id, "DESACTIVATION")

	req := map[string]any{"idDemande": id}
	jeton := h.jeton("orange", "orange2026")
	h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement", jeton, req)

	rep := h.brut(http.MethodPost, "/api/gateway/v1/demandes/traitement", jeton, req)
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}
