package controller_test

// These 9 test functions are moved, unchanged in assertion, from the
// deleted internal/api/incidents_test.go (Task 16). They still exercise the
// real, live router — routerharness.NewRouterHarness wraps api.NewRouter,
// wired exactly as cmd/server/main.go wires it — so a green run here proves
// a real HTTP request to /incidents/{gateway,interne} goes through the new
// IncidentController, its three interactors (internal/usecase/incident) and
// port.UnitOfWork, not through any leftover handler.

import (
	"net/http"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/routerharness"
	"github.com/stretchr/testify/require"
)

// typeIncidentGateway and typeIncidentTechnique are
// internal/framework/seed.TypeIncidentGateway and .TypeIncidentTechnique,
// recopiés en littéral : un test de contrôleur ne peut pas importer
// internal/framework (règle de dépendance) — même précédent
// qu'operateurOrange (creation_particulier_test.go) et operateurExpresso
// (confirmation_boundary_test.go).
const (
	typeIncidentGateway   = "65abc456def001"
	typeIncidentTechnique = "65abc456def002"
)

func TestDeclarationIncidentGateway(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/incidents/gateway",
		h.Jeton("orange", "orange2026"),
		map[string]any{"commentaire": "Timeout de connexion à l'API gateway numFlex"})

	require.Equal(t, http.StatusCreated, rep.StatusCode)
	require.Equal(t, "Incident déclaré avec succès", corps["message"])

	data := corps["data"].(map[string]any)
	require.Equal(t, typeIncidentGateway, data["typeIncidentId"])
	require.Equal(t, "Gateway", data["type"])
	require.Equal(t, false, data["figeSysteme"])
	require.Equal(t, "Timeout de connexion à l'API gateway numFlex", data["description"])
	require.Equal(t, "EN_COURS", data["statut"])
	require.Equal(t, "ORANGE", data["operateur"].(map[string]any)["nom"])
}

func TestDeclarationIncidentInterneGelLaPlace(t *testing.T) {
	// BR-012 : un incident interne bloque le traitement pour tous les opérateurs.
	h := routerharness.NewRouterHarness(t)
	id := creerPortage(h, "771000001")
	avancerA(h, id, "DESACTIVATION")

	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/incidents/interne",
		h.Jeton("expresso", "expresso2026"),
		map[string]any{"commentaire": "Panne du système de routage interne, portages bloqués"})
	require.Equal(t, http.StatusCreated, rep.StatusCode)
	require.Equal(t, true, corps["data"].(map[string]any)["figeSysteme"])
	incidentID := corps["data"].(map[string]any)["id"].(string)

	// ORANGE, étranger à l'incident, ne peut plus traiter.
	rep, _ = h.Appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.Jeton("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)

	// Résolution par le déclarant : la place repart.
	rep, _ = h.Appel(http.MethodPost,
		"/api/gateway/v1/incidents/interne/"+incidentID+"/resoudre",
		h.Jeton("expresso", "expresso2026"), map[string]any{"commentaire": "Service rétabli"})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	rep, _ = h.Appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.Jeton("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusOK, rep.StatusCode)
}

func TestSeulLeDeclarantResout(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	_, corps := h.Appel(http.MethodPost, "/api/gateway/v1/incidents/gateway",
		h.Jeton("orange", "orange2026"), map[string]any{"commentaire": "timeout"})
	id := corps["data"].(map[string]any)["id"].(string)

	rep, _ := h.Appel(http.MethodPost, "/api/gateway/v1/incidents/gateway/"+id+"/resoudre",
		h.Jeton("yas", "yas2026"), map[string]any{"commentaire": "rétabli"})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestResolutionParLeMauvaisSegment(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	_, corps := h.Appel(http.MethodPost, "/api/gateway/v1/incidents/interne",
		h.Jeton("orange", "orange2026"), map[string]any{"commentaire": "panne"})
	id := corps["data"].(map[string]any)["id"].(string)

	rep, corps2 := h.Appel(http.MethodPost, "/api/gateway/v1/incidents/gateway/"+id+"/resoudre",
		h.Jeton("orange", "orange2026"), map[string]any{"commentaire": "rétabli"})

	// Le bon endpoint est porté par le detail du problem+json, pas par un
	// fieldErrors : `id` est une variable de chemin, pas un champ de DTO, et une
	// pile Spring ne rend une constraint-violation que pour la bean validation.
	// L'exigence du guide (§7.12) porte sur l'indication, pas sur son support.
	require.Equal(t, http.StatusBadRequest, rep.StatusCode)
	require.Contains(t, corps2["detail"], "/api/gateway/v1/incidents/interne")
}

func TestUnSeulIncidentInterneOuvertParOperateur(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	jeton := h.Jeton("orange", "orange2026")

	rep, _ := h.Appel(http.MethodPost, "/api/gateway/v1/incidents/interne", jeton,
		map[string]any{"commentaire": "panne 1"})
	require.Equal(t, http.StatusCreated, rep.StatusCode)

	rep, _ = h.Appel(http.MethodPost, "/api/gateway/v1/incidents/interne", jeton,
		map[string]any{"commentaire": "panne 2"})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

// TestPlusieursIncidentsGatewayOuvertsAutorises est le miroir de
// TestUnSeulIncidentInterneOuvertParOperateur : la règle « un seul incident
// ouvert par opérateur » (§7.12) ne vaut que pour les incidents internes. Sans
// ce test, une refonte qui étendrait par erreur la garde `if figeSysteme` aux
// incidents gateway ne ferait échouer aucun test existant.
func TestPlusieursIncidentsGatewayOuvertsAutorises(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	jeton := h.Jeton("orange", "orange2026")

	rep, _ := h.Appel(http.MethodPost, "/api/gateway/v1/incidents/gateway", jeton,
		map[string]any{"commentaire": "timeout 1"})
	require.Equal(t, http.StatusCreated, rep.StatusCode)

	rep, _ = h.Appel(http.MethodPost, "/api/gateway/v1/incidents/gateway", jeton,
		map[string]any{"commentaire": "timeout 2"})
	require.Equal(t, http.StatusCreated, rep.StatusCode)
}

func TestMesIncidentsSontCloisonnesParSegmentEtParOperateur(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	h.Appel(http.MethodPost, "/api/gateway/v1/incidents/gateway",
		h.Jeton("orange", "orange2026"), map[string]any{"commentaire": "timeout"})
	h.Appel(http.MethodPost, "/api/gateway/v1/incidents/interne",
		h.Jeton("orange", "orange2026"), map[string]any{"commentaire": "panne"})

	jetonOrange := h.Jeton("orange", "orange2026")
	require.Len(t, h.Liste("/api/gateway/v1/incidents/gateway/mes-incidents", jetonOrange), 1)
	require.Len(t, h.Liste("/api/gateway/v1/incidents/interne/mes-incidents", jetonOrange), 1)

	require.Empty(t, h.Liste("/api/gateway/v1/incidents/gateway/mes-incidents",
		h.Jeton("yas", "yas2026")))
}

func TestDeclarationSansTypeIncidentId(t *testing.T) {
	// §7.12 : le corps ne prend qu'un commentaire ; le type est résolu côté serveur.
	h := routerharness.NewRouterHarness(t)
	rep, corps := h.Appel(http.MethodPost, "/api/gateway/v1/incidents/gateway",
		h.Jeton("yas", "yas2026"),
		map[string]any{"commentaire": "test", "typeIncidentId": typeIncidentTechnique})

	require.Equal(t, http.StatusCreated, rep.StatusCode)
	// Le typeIncidentId fourni est ignoré : c'est l'endpoint qui décide.
	require.Equal(t, typeIncidentGateway, corps["data"].(map[string]any)["typeIncidentId"])
}

// TestResolutionParLeMauvaisSegmentIndiqueLeBonEndpoint — §7.12 du guide :
// « Résoudre un incident via le mauvais segment renvoie une erreur
// VALIDATION_ECHOUEE indiquant le bon endpoint. » Le code seul ne suffit pas :
// c'est l'indication de l'endpoint que le guide promet, et elle doit atteindre
// le client, pas rester dans un fieldError que l'enveloppe de contrat jette.
func TestResolutionParLeMauvaisSegmentIndiqueLeBonEndpoint(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.FiabiliteContrat)
	jeton := h.Jeton("orange", "orange2026")

	_, corps := h.Appel(http.MethodPost, "/api/gateway/v1/incidents/gateway", jeton,
		map[string]any{"commentaire": "Timeout de connexion à l'API gateway numFlex"})
	incidentID := corps["data"].(map[string]any)["id"].(string)

	// L'incident est de catégorie Gateway : le résoudre via /interne est un
	// mauvais segment.
	rep, corps := h.Appel(http.MethodPost,
		"/api/gateway/v1/incidents/interne/"+incidentID+"/resoudre", jeton,
		map[string]any{"commentaire": "Service rétabli"})

	require.Equal(t, http.StatusBadRequest, rep.StatusCode)
	require.Equal(t, "VALIDATION_ECHOUEE", corps["code"])
	require.Equal(t,
		"Cet incident se résout via POST /api/gateway/v1/incidents/gateway/{id}/resoudre.",
		corps["message"])
}
