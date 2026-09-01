package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/config"
	"github.com/yas/numflex-sandbox/internal/seed"
)

// TestPortageCompletJusquATermine rejoue le §10 du guide : ORANGE → YAS, avec la
// confirmation d'EXPRESSO, jusqu'à TERMINE et apparition dans /in et /out.
func TestPortageCompletJusquATermine(t *testing.T) {
	h := nouveauHarnais(t) // profil déterministe : convergence et latences nulles

	yas := h.jeton("yas", "yas2026")
	orange := h.jeton("orange", "orange2026")
	expresso := h.jeton("expresso", "expresso2026")

	// 1-2. OTP puis création par le destinataire.
	h.post("/api/gateway/v1/otp/send", yas, map[string]any{"numero": "771000001"})
	_, corps := h.post("/api/gateway/v1/demandes/particulier", yas, corpsParticulier("771000001"))
	id := corps["data"].(map[string]any)["id"].(string)

	// 3. La source voit la demande dans sa file.
	require.Len(t, h.liste("/api/gateway/v1/demandes/a-accepter", orange), 1)

	// 4. Acceptation par la source, puis convergence.
	h.post("/api/gateway/v1/demandes/acceptation", orange,
		map[string]any{"idDemande": id, "accepte": true})
	h.converger()
	require.Equal(t, "DESACTIVATION", h.etape(id))

	// 5. Désactivation par la source.
	h.post("/api/gateway/v1/demandes/traitement", orange, map[string]any{"idDemande": id})
	h.converger()
	require.Equal(t, "ACTIVATION", h.etape(id))

	// 6. Activation par le destinataire.
	h.post("/api/gateway/v1/demandes/traitement", yas, map[string]any{"idDemande": id})
	h.converger()
	require.Equal(t, "CONFIRMATION", h.etape(id))

	// 7. Confirmation : la source, puis le tiers. Une seule ne suffit pas.
	h.post("/api/gateway/v1/demandes/a-confirmer", orange, map[string]any{"idDemande": id})
	h.converger()
	require.Equal(t, "CONFIRMATION", h.etape(id), "il manque la confirmation d'EXPRESSO")

	h.post("/api/gateway/v1/demandes/a-confirmer", expresso, map[string]any{"idDemande": id})
	h.converger()
	require.Equal(t, "COMPLETION", h.etape(id))

	// 8. Clôture par le destinataire.
	h.post("/api/gateway/v1/demandes/traitement", yas, map[string]any{"idDemande": id})
	h.converger()

	require.Equal(t, "TERMINE", h.statutDemande(id))

	// La demande apparaît des deux côtés.
	in := h.liste("/api/gateway/v1/demandes/in", yas)
	require.Len(t, in, 1)
	require.Equal(t, id, in[0].(map[string]any)["id"])

	out := h.liste("/api/gateway/v1/demandes/out", orange)
	require.Len(t, out, 1)

	// Le numéro a changé d'opérateur au registre national.
	require.Equal(t, seed.OperateurYAS, h.detenteur("771000001"))
}

// TestPortageParExpirationSansAucunAppel rejoue le portage n°2 du SIT : créé,
// laissé sans action, TERMINE après cinq expirations d'étape (ANO-006, TC-062).
func TestPortageParExpirationSansAucunAppel(t *testing.T) {
	h := nouveauHarnais(t, func(c *config.Config) {
		c.EtapeTimeout = 50 * time.Millisecond
	})

	yas := h.jeton("yas", "yas2026")
	h.post("/api/gateway/v1/otp/send", yas, map[string]any{"numero": "771000001"})
	_, corps := h.post("/api/gateway/v1/demandes/particulier", yas, corpsParticulier("771000001"))
	id := corps["data"].(map[string]any)["id"].(string)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.moteur.Run(ctx)

	require.Eventually(t, func() bool {
		return h.statutDemande(id) == "TERMINE"
	}, 5*time.Second, 20*time.Millisecond)

	require.Equal(t, "EXPIRE", h.statutEtape(id))
	require.Equal(t, seed.OperateurYAS, h.detenteur("771000001"),
		"le numéro a changé d'opérateur alors qu'aucun HLR n'a été touché")
}

// TestMemeScenarioEnModeContrat vérifie que seule la présentation change entre
// les deux modes de fidélité : le rejeu du scénario nominal, en FIDELITY=contract,
// doit atteindre le même état terminal que TestPortageCompletJusquATermine.
// Aucune assertion sur les codes HTTP ou l'enveloppe ici — c'est le point : la
// bascule de fidélité change le rendu, jamais le comportement métier.
func TestMemeScenarioEnModeContrat(t *testing.T) {
	h := nouveauHarnais(t, func(c *config.Config) { c.Fidelity = config.FidelityContract })

	yas := h.jeton("yas", "yas2026")
	orange := h.jeton("orange", "orange2026")
	expresso := h.jeton("expresso", "expresso2026")

	h.post("/api/gateway/v1/otp/send", yas, map[string]any{"numero": "771000001"})
	_, corps := h.post("/api/gateway/v1/demandes/particulier", yas, corpsParticulier("771000001"))
	id := corps["data"].(map[string]any)["id"].(string)

	h.post("/api/gateway/v1/demandes/acceptation", orange,
		map[string]any{"idDemande": id, "accepte": true})
	h.converger()
	h.post("/api/gateway/v1/demandes/traitement", orange, map[string]any{"idDemande": id})
	h.converger()
	h.post("/api/gateway/v1/demandes/traitement", yas, map[string]any{"idDemande": id})
	h.converger()
	h.post("/api/gateway/v1/demandes/a-confirmer", orange, map[string]any{"idDemande": id})
	h.post("/api/gateway/v1/demandes/a-confirmer", expresso, map[string]any{"idDemande": id})
	h.converger()
	h.post("/api/gateway/v1/demandes/traitement", yas, map[string]any{"idDemande": id})
	h.converger()

	require.Equal(t, "TERMINE", h.statutDemande(id))
}

// TestAucuneErreurNePorteDeCodeEnModeReel — ANO-001, vérifié en volume. Chacun
// des huit appels ci-dessous provoque une situation d'erreur différente ; aucune
// des huit réponses ne doit porter de champ code ni être enveloppée.
func TestAucuneErreurNePorteDeCodeEnModeReel(t *testing.T) {
	h := nouveauHarnais(t)
	yas := h.jeton("yas", "yas2026")
	orange := h.jeton("orange", "orange2026")

	inconnu := "6a0000000000000000000000"
	appels := []struct {
		chemin string
		jeton  string
		corps  any
	}{
		// Demande inconnue, sur chacun des quatre endpoints de traitement.
		{"/api/gateway/v1/demandes/traitement", yas, map[string]any{"idDemande": inconnu}},
		{"/api/gateway/v1/demandes/acceptation", orange, map[string]any{"idDemande": inconnu, "accepte": true}},
		{"/api/gateway/v1/demandes/a-confirmer", orange, map[string]any{"idDemande": inconnu}},
		{"/api/gateway/v1/demandes/" + inconnu + "/annuler", yas, nil},
		// Restitution d'un numéro jamais porté (771000001 : ORANGE d'origine).
		{"/api/gateway/v1/demandes/restitution", orange, map[string]any{"numero": "771000001"}},
		// Restitution d'un numéro porté depuis moins de 6 mois (774000001 : 2 mois).
		{"/api/gateway/v1/demandes/restitution", orange, map[string]any{"numero": "774000001"}},
		// Reverse demandé par le destinataire (YAS) au lieu de la source (ORANGE).
		{"/api/gateway/v1/reverse-requests", yas, map[string]any{"numero": "773000001"}},
		// Vérification d'un OTP jamais envoyé.
		{"/api/gateway/v1/otp/verify", yas, map[string]any{"numero": "779999999", "otpCode": "123456"}},
	}

	for _, a := range appels {
		rep, corps := h.postBrut(a.chemin, a.jeton, a.corps)
		require.GreaterOrEqualf(t, rep.StatusCode, 400, a.chemin)
		require.NotContainsf(t, corps, "code", "%s ne doit porter aucun champ code", a.chemin)
		require.NotContainsf(t, corps, "success", "%s ne doit pas être enveloppée", a.chemin)
		require.Containsf(t, corps, "type", "%s doit être un problem+json", a.chemin)
	}
}
