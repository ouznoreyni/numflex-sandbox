package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/seed"
)

func TestAcceptationNominale(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.jeton("orange", "orange2026"),
		map[string]any{"idDemande": id, "accepte": true, "commentaire": "Demande conforme"})

	require.Equal(t, http.StatusOK, rep.StatusCode, corps)
	require.Equal(t, "Étape traitée avec succès", corps["message"])

	// R-10 : la réponse porte l'étape PRÉCÉDANT la transition.
	data := corps["data"].(map[string]any)
	require.Equal(t, "ACCEPTATION", data["etapeActuelle"])

	// La transition est planifiée, pas encore appliquée.
	var prevue *string
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT transition_prevue_a::text FROM demande WHERE id = $1", id).Scan(&prevue))
	require.NotNil(t, prevue)
}

func TestAcceptationParLeDestinataireRefusee(t *testing.T) {
	// TC-034 : refusé — mais en HTTP 500.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.jeton("yas", "yas2026"), map[string]any{"idDemande": id, "accepte": true})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.NotContains(t, corps, "code")
}

func TestRejetSansMotifRefuse(t *testing.T) {
	// TC-044 : refusé — en HTTP 500.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id, "accepte": false})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t,
		"RuntimeException: Un motif de rejet est obligatoire pour rejeter une demande",
		corps["detail"])
}

func TestRejetAvecMotifTermineLaDemande(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.jeton("orange", "orange2026"), map[string]any{
			"idDemande": id, "accepte": false,
			"motifRejetId": seed.MotifIdentiteNonProuvee,
			"commentaire":  "Contrat non résilié",
		})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	var statut, motif string
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT statut_demande, motif_rejet_id FROM demande WHERE id = $1", id).
		Scan(&statut, &motif))
	require.Equal(t, "REJETE", statut)
	require.Equal(t, seed.MotifIdentiteNonProuvee, motif)
}

func TestAcceptationIdDemandeInconnu(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.jeton("orange", "orange2026"),
		map[string]any{"idDemande": "6a0000000000000000000000", "accepte": true})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "RuntimeException: Demande introuvable", corps["detail"])
}

func TestAcceptationNumeroRefuseCarV2IdentifieParIdDemande(t *testing.T) {
	// Rupture v1 → v2 : le champ numero n'est plus reconnu.
	h := nouveauHarnais(t)
	h.creerPortage("771000001")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.jeton("orange", "orange2026"),
		map[string]any{"numero": "771000001", "accepte": true})

	require.Equal(t, http.StatusBadRequest, rep.StatusCode)
	champs := corps["fieldErrors"].([]any)
	require.Equal(t, "idDemande", champs[0].(map[string]any)["field"])
}

func TestAcceptationFlotteAvecRejetPartiel(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})
	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("771000001", []string{"771000001", "771000002", "771000003"}))
	id := corps["data"].(map[string]any)["demande"].(map[string]any)["id"].(string)

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/acceptation",
		h.jeton("orange", "orange2026"), map[string]any{
			"accepte": true,
			"numerosRejetes": []map[string]any{
				{"numero": "771000002", "motifRejetId": seed.MotifNumeroInactif},
			},
			"commentaire": "Numéro 771000002 non conforme",
		})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	var statut string
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT statut FROM demande_numero WHERE demande_id = $1 AND numero = '771000002'", id).
		Scan(&statut))
	require.Equal(t, "REJETE", statut)

	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT statut FROM demande_numero WHERE demande_id = $1 AND numero = '771000001'", id).
		Scan(&statut))
	require.Equal(t, "EN_COURS", statut)
}

func TestAcceptationFlotteRejetTotal(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})
	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("771000001", []string{"771000001", "771000002"}))
	id := corps["data"].(map[string]any)["demande"].(map[string]any)["id"].(string)

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/acceptation",
		h.jeton("orange", "orange2026"), map[string]any{
			"accepte": false, "motifRejetId": seed.MotifDonneesManquantes,
			"commentaire": "Dossier incomplet",
		})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	var statut string
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT statut_demande FROM demande WHERE id = $1", id).Scan(&statut))
	require.Equal(t, "REJETE", statut)
}
