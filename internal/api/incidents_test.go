package api

import (
	"net/http"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/seed"
	"github.com/stretchr/testify/require"
)

func TestDeclarationIncidentGateway(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/incidents/gateway",
		h.jeton("orange", "orange2026"),
		map[string]any{"commentaire": "Timeout de connexion à l'API gateway numFlex"})

	require.Equal(t, http.StatusCreated, rep.StatusCode)
	require.Equal(t, "Incident déclaré avec succès", corps["message"])

	data := corps["data"].(map[string]any)
	require.Equal(t, seed.TypeIncidentGateway, data["typeIncidentId"])
	require.Equal(t, "Gateway", data["type"])
	require.Equal(t, false, data["figeSysteme"])
	require.Equal(t, "Timeout de connexion à l'API gateway numFlex", data["description"])
	require.Equal(t, "EN_COURS", data["statut"])
	require.Equal(t, "ORANGE", data["operateur"].(map[string]any)["nom"])
}

func TestDeclarationIncidentInterneGelLaPlace(t *testing.T) {
	// BR-012 : un incident interne bloque le traitement pour tous les opérateurs.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "DESACTIVATION")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/incidents/interne",
		h.jeton("expresso", "expresso2026"),
		map[string]any{"commentaire": "Panne du système de routage interne, portages bloqués"})
	require.Equal(t, http.StatusCreated, rep.StatusCode)
	require.Equal(t, true, corps["data"].(map[string]any)["figeSysteme"])
	incidentID := corps["data"].(map[string]any)["id"].(string)

	// ORANGE, étranger à l'incident, ne peut plus traiter.
	rep, _ = h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)

	// Résolution par le déclarant : la place repart.
	rep, _ = h.appel(http.MethodPost,
		"/api/gateway/v1/incidents/interne/"+incidentID+"/resoudre",
		h.jeton("expresso", "expresso2026"), map[string]any{"commentaire": "Service rétabli"})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	rep, _ = h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusOK, rep.StatusCode)
}

func TestSeulLeDeclarantResout(t *testing.T) {
	h := nouveauHarnais(t)
	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/incidents/gateway",
		h.jeton("orange", "orange2026"), map[string]any{"commentaire": "timeout"})
	id := corps["data"].(map[string]any)["id"].(string)

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/incidents/gateway/"+id+"/resoudre",
		h.jeton("yas", "yas2026"), map[string]any{"commentaire": "rétabli"})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestResolutionParLeMauvaisSegment(t *testing.T) {
	h := nouveauHarnais(t)
	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/incidents/interne",
		h.jeton("orange", "orange2026"), map[string]any{"commentaire": "panne"})
	id := corps["data"].(map[string]any)["id"].(string)

	rep, corps2 := h.appel(http.MethodPost, "/api/gateway/v1/incidents/gateway/"+id+"/resoudre",
		h.jeton("orange", "orange2026"), map[string]any{"commentaire": "rétabli"})

	// Le bon endpoint est porté par le detail du problem+json, pas par un
	// fieldErrors : `id` est une variable de chemin, pas un champ de DTO, et une
	// pile Spring ne rend une constraint-violation que pour la bean validation.
	// L'exigence du guide (§7.12) porte sur l'indication, pas sur son support.
	require.Equal(t, http.StatusBadRequest, rep.StatusCode)
	require.Contains(t, corps2["detail"], "/api/gateway/v1/incidents/interne")
}

func TestUnSeulIncidentInterneOuvertParOperateur(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("orange", "orange2026")

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/incidents/interne", jeton,
		map[string]any{"commentaire": "panne 1"})
	require.Equal(t, http.StatusCreated, rep.StatusCode)

	rep, _ = h.appel(http.MethodPost, "/api/gateway/v1/incidents/interne", jeton,
		map[string]any{"commentaire": "panne 2"})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

// TestPlusieursIncidentsGatewayOuvertsAutorises est le miroir de
// TestUnSeulIncidentInterneOuvertParOperateur : la règle « un seul incident
// ouvert par opérateur » (§7.12) ne vaut que pour les incidents internes. Sans
// ce test, une refonte qui étendrait par erreur la garde `if figeSysteme` aux
// incidents gateway ne ferait échouer aucun test existant.
func TestPlusieursIncidentsGatewayOuvertsAutorises(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("orange", "orange2026")

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/incidents/gateway", jeton,
		map[string]any{"commentaire": "timeout 1"})
	require.Equal(t, http.StatusCreated, rep.StatusCode)

	rep, _ = h.appel(http.MethodPost, "/api/gateway/v1/incidents/gateway", jeton,
		map[string]any{"commentaire": "timeout 2"})
	require.Equal(t, http.StatusCreated, rep.StatusCode)
}

func TestMesIncidentsSontCloisonnesParSegmentEtParOperateur(t *testing.T) {
	h := nouveauHarnais(t)
	h.appel(http.MethodPost, "/api/gateway/v1/incidents/gateway",
		h.jeton("orange", "orange2026"), map[string]any{"commentaire": "timeout"})
	h.appel(http.MethodPost, "/api/gateway/v1/incidents/interne",
		h.jeton("orange", "orange2026"), map[string]any{"commentaire": "panne"})

	jetonOrange := h.jeton("orange", "orange2026")
	require.Len(t, h.liste("/api/gateway/v1/incidents/gateway/mes-incidents", jetonOrange), 1)
	require.Len(t, h.liste("/api/gateway/v1/incidents/interne/mes-incidents", jetonOrange), 1)

	require.Empty(t, h.liste("/api/gateway/v1/incidents/gateway/mes-incidents",
		h.jeton("yas", "yas2026")))
}

func TestDeclarationSansTypeIncidentId(t *testing.T) {
	// §7.12 : le corps ne prend qu'un commentaire ; le type est résolu côté serveur.
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/incidents/gateway",
		h.jeton("yas", "yas2026"),
		map[string]any{"commentaire": "test", "typeIncidentId": seed.TypeIncidentTechnique})

	require.Equal(t, http.StatusCreated, rep.StatusCode)
	// Le typeIncidentId fourni est ignoré : c'est l'endpoint qui décide.
	require.Equal(t, seed.TypeIncidentGateway, corps["data"].(map[string]any)["typeIncidentId"])
}

// TestResolutionParLeMauvaisSegmentIndiqueLeBonEndpoint — §7.12 du guide :
// « Résoudre un incident via le mauvais segment renvoie une erreur
// VALIDATION_ECHOUEE indiquant le bon endpoint. » Le code seul ne suffit pas :
// c'est l'indication de l'endpoint que le guide promet, et elle doit atteindre
// le client, pas rester dans un fieldError que l'enveloppe de contrat jette.
func TestResolutionParLeMauvaisSegmentIndiqueLeBonEndpoint(t *testing.T) {
	h := nouveauHarnais(t, func(c *config.Config) { c.Fidelity = config.FidelityContract })
	jeton := h.jeton("orange", "orange2026")

	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/incidents/gateway", jeton,
		map[string]any{"commentaire": "Timeout de connexion à l'API gateway numFlex"})
	incidentID := corps["data"].(map[string]any)["id"].(string)

	// L'incident est de catégorie Gateway : le résoudre via /interne est un
	// mauvais segment.
	rep, corps := h.appel(http.MethodPost,
		"/api/gateway/v1/incidents/interne/"+incidentID+"/resoudre", jeton,
		map[string]any{"commentaire": "Service rétabli"})

	require.Equal(t, http.StatusBadRequest, rep.StatusCode)
	require.Equal(t, "VALIDATION_ECHOUEE", corps["code"])
	require.Equal(t,
		"Cet incident se résout via POST /api/gateway/v1/incidents/gateway/{id}/resoudre.",
		corps["message"])
}
