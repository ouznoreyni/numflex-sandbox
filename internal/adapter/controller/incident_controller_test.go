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

// incidentTypeGateway and incidentTypeTechnical are
// internal/framework/seed.IncidentTypeGatewayID and .IncidentTypeTechnicalID,
// copied as literals: a controller test cannot import
// internal/framework (dependency rule) — the same precedent as
// operatorOrange (creation_particulier_test.go) and operatorExpresso
// (confirmation_boundary_test.go).
const (
	incidentTypeGateway   = "65abc456def001"
	incidentTypeTechnical = "65abc456def002"
)

func TestDeclareIncidentGateway(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/incidents/gateway",
		h.Token("orange", "orange2026"),
		map[string]any{"commentaire": "Timeout de connexion à l'API gateway numFlex"})

	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Equal(t, "Incident déclaré avec succès", body["message"])

	data := body["data"].(map[string]any)
	require.Equal(t, incidentTypeGateway, data["typeIncidentId"])
	require.Equal(t, "Gateway", data["type"])
	require.Equal(t, false, data["figeSysteme"])
	require.Equal(t, "Timeout de connexion à l'API gateway numFlex", data["description"])
	require.Equal(t, "EN_COURS", data["statut"])
	require.Equal(t, "ORANGE", data["operateur"].(map[string]any)["nom"])
}

func TestDeclareInternalIncidentFreezesTheMarket(t *testing.T) {
	// BR-012: an internal incident blocks processing for every operator.
	h := routerharness.NewRouterHarness(t)
	id := createPorting(h, "771000001")
	advanceTo(h, id, "DESACTIVATION")

	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/incidents/interne",
		h.Token("expresso", "expresso2026"),
		map[string]any{"commentaire": "Panne du système de routage interne, portages bloqués"})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Equal(t, true, body["data"].(map[string]any)["figeSysteme"])
	incidentID := body["data"].(map[string]any)["id"].(string)

	// ORANGE, a stranger to the incident, can no longer process.
	resp, _ = h.Call(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.Token("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	// Resolution by the declarer: the market unfreezes.
	resp, _ = h.Call(http.MethodPost,
		"/api/gateway/v1/incidents/interne/"+incidentID+"/resoudre",
		h.Token("expresso", "expresso2026"), map[string]any{"commentaire": "Service rétabli"})
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp, _ = h.Call(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.Token("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestOnlyTheDeclarerResolves(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	_, body := h.Call(http.MethodPost, "/api/gateway/v1/incidents/gateway",
		h.Token("orange", "orange2026"), map[string]any{"commentaire": "timeout"})
	id := body["data"].(map[string]any)["id"].(string)

	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/incidents/gateway/"+id+"/resoudre",
		h.Token("yas", "yas2026"), map[string]any{"commentaire": "rétabli"})
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestResolveByTheWrongSegment(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	_, body := h.Call(http.MethodPost, "/api/gateway/v1/incidents/interne",
		h.Token("orange", "orange2026"), map[string]any{"commentaire": "panne"})
	id := body["data"].(map[string]any)["id"].(string)

	resp, corps2 := h.Call(http.MethodPost, "/api/gateway/v1/incidents/gateway/"+id+"/resoudre",
		h.Token("orange", "orange2026"), map[string]any{"commentaire": "rétabli"})

	// The right endpoint is carried by the problem+json's detail, not by a
	// fieldErrors: `id` is a path variable, not a DTO field, and a Spring
	// stack only renders a constraint-violation for bean validation. The
	// guide's requirement (§7.12) is about the indication, not its carrier.
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Contains(t, corps2["detail"], "/api/gateway/v1/incidents/interne")
}

func TestOnlyOneOpenInternalIncidentPerOperator(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	token := h.Token("orange", "orange2026")

	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/incidents/interne", token,
		map[string]any{"commentaire": "panne 1"})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, _ = h.Call(http.MethodPost, "/api/gateway/v1/incidents/interne", token,
		map[string]any{"commentaire": "panne 2"})
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// TestMultipleOpenGatewayIncidentsAllowed mirrors
// TestOnlyOneOpenInternalIncidentPerOperator: the rule "one open incident
// per operator" (§7.12) only holds for internal incidents. Without this
// test, a refactor that mistakenly extended the `if figeSysteme` guard to
// gateway incidents would fail no existing test.
func TestMultipleOpenGatewayIncidentsAllowed(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	token := h.Token("orange", "orange2026")

	resp, _ := h.Call(http.MethodPost, "/api/gateway/v1/incidents/gateway", token,
		map[string]any{"commentaire": "timeout 1"})
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, _ = h.Call(http.MethodPost, "/api/gateway/v1/incidents/gateway", token,
		map[string]any{"commentaire": "timeout 2"})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestOwnIncidentsAreSegmentedBySegmentAndOperator(t *testing.T) {
	h := routerharness.NewRouterHarness(t)
	h.Call(http.MethodPost, "/api/gateway/v1/incidents/gateway",
		h.Token("orange", "orange2026"), map[string]any{"commentaire": "timeout"})
	h.Call(http.MethodPost, "/api/gateway/v1/incidents/interne",
		h.Token("orange", "orange2026"), map[string]any{"commentaire": "panne"})

	jetonOrange := h.Token("orange", "orange2026")
	require.Len(t, h.List("/api/gateway/v1/incidents/gateway/mes-incidents", jetonOrange), 1)
	require.Len(t, h.List("/api/gateway/v1/incidents/interne/mes-incidents", jetonOrange), 1)

	require.Empty(t, h.List("/api/gateway/v1/incidents/gateway/mes-incidents",
		h.Token("yas", "yas2026")))
}

func TestDeclareWithoutTypeIncidentId(t *testing.T) {
	// §7.12: the body only takes a comment; the type is resolved server-side.
	h := routerharness.NewRouterHarness(t)
	resp, body := h.Call(http.MethodPost, "/api/gateway/v1/incidents/gateway",
		h.Token("yas", "yas2026"),
		map[string]any{"commentaire": "test", "typeIncidentId": incidentTypeTechnical})

	require.Equal(t, http.StatusCreated, resp.StatusCode)
	// The supplied typeIncidentId is ignored: it is the endpoint that decides.
	require.Equal(t, incidentTypeGateway, body["data"].(map[string]any)["typeIncidentId"])
}

// TestResolveByTheWrongSegmentPointsToTheRightEndpoint — guide §7.12:
// "Resolving an incident via the wrong segment returns a VALIDATION_ECHOUEE
// error naming the right endpoint." The code alone is not enough: it is the
// endpoint indication the guide promises, and it must reach the client, not
// stay in a fieldError the contract envelope discards.
func TestResolveByTheWrongSegmentPointsToTheRightEndpoint(t *testing.T) {
	h := routerharness.NewRouterHarness(t, routerharness.ContractFidelity)
	token := h.Token("orange", "orange2026")

	_, body := h.Call(http.MethodPost, "/api/gateway/v1/incidents/gateway", token,
		map[string]any{"commentaire": "Timeout de connexion à l'API gateway numFlex"})
	incidentID := body["data"].(map[string]any)["id"].(string)

	// The incident is of category Gateway: resolving it via /interne is the
	// wrong segment.
	resp, body := h.Call(http.MethodPost,
		"/api/gateway/v1/incidents/interne/"+incidentID+"/resoudre", token,
		map[string]any{"commentaire": "Service rétabli"})

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Equal(t, "VALIDATION_ECHOUEE", body["code"])
	require.Equal(t,
		"Cet incident se résout via POST /api/gateway/v1/incidents/gateway/{id}/resoudre.",
		body["message"])
}
