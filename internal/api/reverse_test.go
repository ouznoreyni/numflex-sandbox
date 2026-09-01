package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/config"
	"github.com/yas/numflex-sandbox/internal/engine"
	"github.com/yas/numflex-sandbox/internal/seed"
)

func TestSoumissionReverseParLOperateurDOrigine(t *testing.T) {
	// Tranche 773 : YAS actuellement, ORANGE à l'origine.
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})

	require.Equal(t, http.StatusCreated, rep.StatusCode, corps)
	require.Equal(t, "Demande de reverse soumise avec succès", corps["message"])

	data := corps["data"].(map[string]any)
	require.Equal(t, "773000001", data["numero"])
	require.Equal(t, "EN_ATTENTE", data["statut"])
	require.Equal(t, seed.OperateurOrange, data["operateur"].(map[string]any)["id"])
}

func TestSoumissionReverseParUnAutreOperateurRefusee(t *testing.T) {
	h := nouveauHarnais(t)
	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.jeton("yas", "yas2026"), map[string]any{"numero": "773000001"})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestMesDemandesReverse(t *testing.T) {
	h := nouveauHarnais(t)
	h.appel(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})

	data := h.liste("/api/gateway/v1/reverse-requests/mes-demandes",
		h.jeton("orange", "orange2026"))
	require.Len(t, data, 1)
	require.Equal(t, "EN_ATTENTE", data[0].(map[string]any)["statut"])

	require.Empty(t, h.liste("/api/gateway/v1/reverse-requests/mes-demandes",
		h.jeton("yas", "yas2026")))
}

func TestAucunEndpointDAnnulationDeReverse(t *testing.T) {
	// §7.6 : « Il n'existe pas d'endpoint pour annuler une demande de reverse. »
	h := nouveauHarnais(t)
	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})
	id := corps["data"].(map[string]any)["id"].(string)

	rep := h.brut(http.MethodPost, "/api/gateway/v1/reverse-requests/"+id+"/annuler",
		h.jeton("orange", "orange2026"), nil)
	require.Equal(t, http.StatusNotFound, rep.StatusCode)
}

// TestReverseAtteintTermineParLesVraisEndpoints prouve le flux complet en
// passant par les vrais endpoints — /reverse-requests puis
// /demandes/a-confirmer, comme un opérateur réel le ferait — plutôt qu'en
// insérant les confirmations directement en SQL. postAConfirmer est
// agnostique du type de demande : quand la dernière confirmation tombe, il
// planifie une transition générique (PlanifierTransition). Au tick suivant,
// appliquerConvergencesDues fait avancer la demande de CONFIRMATION à
// COMPLETION par ce chemin commun, avant que completerReversesConfirmes ne
// s'exécute — qui doit donc savoir rattraper une demande REVERSE déjà à
// COMPLETION, faute de quoi elle y reste bloquée pour toujours, puisque
// aucun opérateur ne peut traiter la COMPLETION d'un REVERSE.
func TestReverseAtteintTermineParLesVraisEndpoints(t *testing.T) {
	h := nouveauHarnais(t)

	// 1. Soumission par l'opérateur d'origine (tranche 773 : YAS actuellement,
	// ORANGE à l'origine).
	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})
	reverseID := corps["data"].(map[string]any)["id"].(string)

	// 2. Acte de l'ARTP, hors API : validation — crée la Demande REVERSE à
	// CONFIRMATION.
	require.NoError(t, engine.ValiderReverse(context.Background(), h.db, reverseID))

	var demandeID string
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		`SELECT demande_id FROM reverse_request WHERE id = $1`, reverseID).Scan(&demandeID))

	// 3. Tous les opérateurs confirment via le vrai endpoint — destinataire
	// (opérateur d'origine du numéro) compris, comme l'exige un REVERSE.
	for _, compte := range [][2]string{
		{"orange", "orange2026"}, {"yas", "yas2026"}, {"expresso", "expresso2026"},
	} {
		rep, corpsConf := h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
			h.jeton(compte[0], compte[1]), map[string]any{"idDemande": demandeID})
		require.Equal(t, http.StatusOK, rep.StatusCode, corpsConf)
	}

	// 4. Convergence du moteur : la demande doit atteindre TERMINE, et le
	// numéro doit être revenu à son opérateur d'origine dans le registre.
	h.converger()
	h.converger()

	require.Equal(t, "TERMINE", h.statutDemande(demandeID))

	var operateurActuel string
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		`SELECT operateur_actuel_id FROM numero WHERE msisdn = '773000001'`).
		Scan(&operateurActuel))
	require.Equal(t, seed.OperateurOrange, operateurActuel)
}

// TestCompletionDUnReverseToujoursRefuseeAuxOperateurs — §7.9 du guide : une
// tentative de COMPLETION sur un REVERSE renvoie DEMANDE_ACCES_REFUSE avec le
// message documenté. Le moteur joue l'ARTP et complète la demande dès que
// toutes les confirmations sont là ; si le refus n'était rendu que tant que la
// demande est EN_COURS, la fenêtre serait d'un tick et le message documenté
// deviendrait inatteignable en pratique — l'opérateur recevrait un
// ETAPE_INVALIDE générique à la place.
func TestCompletionDUnReverseToujoursRefuseeAuxOperateurs(t *testing.T) {
	h := nouveauHarnais(t, func(c *config.Config) { c.Fidelity = config.FidelityContract })

	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})
	reverseID := corps["data"].(map[string]any)["id"].(string)
	require.NoError(t, engine.ValiderReverse(context.Background(), h.db, reverseID))

	var demandeID string
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		`SELECT demande_id FROM reverse_request WHERE id = $1`, reverseID).Scan(&demandeID))

	for _, compte := range [][2]string{
		{"orange", "orange2026"}, {"yas", "yas2026"}, {"expresso", "expresso2026"},
	} {
		h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
			h.jeton(compte[0], compte[1]), map[string]any{"idDemande": demandeID})
	}
	h.converger()
	h.converger()
	require.Equal(t, "COMPLETION", h.etape(demandeID))

	// Destinataire comme source : le refus est le même, et il est celui du guide.
	for _, compte := range [][2]string{{"orange", "orange2026"}, {"yas", "yas2026"}} {
		rep, corpsRefus := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
			h.jeton(compte[0], compte[1]), map[string]any{"idDemande": demandeID})

		require.Equal(t, http.StatusForbidden, rep.StatusCode, compte[0])
		require.Equal(t, "DEMANDE_ACCES_REFUSE", corpsRefus["code"], compte[0])
		require.Equal(t,
			"La complétion (COMPLETION) d'une demande REVERSE est réservée à l'ARTP, "+
				"une fois que tous les opérateurs ont confirmé.",
			corpsRefus["message"], compte[0])
	}
}
