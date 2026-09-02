package engine

import (
	"context"
	"testing"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/adapter/gateway/postgres"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/config"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/persistence"
	"github.com/ouznoreyni/numflex-sandbox/internal/framework/seed"
	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
	"github.com/stretchr/testify/require"
)

// insererDemande crée une demande directement en base, à l'étape voulue.
func insererDemande(t *testing.T, db *persistence.DB, id string, etape entity.Step, ageEtape time.Duration) {
	t.Helper()
	debut := time.Now().Add(-ageEtape)
	_, err := db.Pool.Exec(context.Background(),
		`INSERT INTO demande
		   (id, numero, type_abonne, type_demande, statut_demande, etape_actuelle,
		    statut_etape_actuel, operateur_source_id, operateur_destinataire_id,
		    createur_operateur_id, processus, routage_info, date_demande, date_debut_etape)
		 VALUES ($1,'771000001','PARTICULIER','PORTAGE','EN_COURS',$2,'EN_COURS',
		         $3,$4,$4,'PREPAID','191',now(),$5)`,
		id, string(etape), seed.OperateurOrange, seed.OperateurYAS, debut)
	require.NoError(t, err)

	_, err = db.Pool.Exec(context.Background(),
		`INSERT INTO demande_numero (demande_id, numero, statut) VALUES ($1,'771000001','EN_COURS')`, id)
	require.NoError(t, err)
}

func etatDemande(t *testing.T, db *persistence.DB, id string) (etape, statutEtape, statutDemande string) {
	t.Helper()
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT etape_actuelle, statut_etape_actuel, statut_demande FROM demande WHERE id = $1`, id).
		Scan(&etape, &statutEtape, &statutDemande))
	return
}

func moteur(t *testing.T, ajuste ...func(*config.Config)) (*Engine, *persistence.DB) {
	t.Helper()
	db := testsupport.NewTestDB(t)
	cfg := &config.Config{EngineTick: time.Millisecond, EtapeTimeout: 0}
	for _, f := range ajuste {
		f(cfg)
	}
	return New(cfg, db), db
}

func TestExpirationFaitAvancerSansAucunAppel(t *testing.T) {
	// TC-062 / ANO-006 : les étapes progressent seules.
	e, db := moteur(t, func(c *config.Config) { c.EtapeTimeout = 2 * time.Second })
	insererDemande(t, db, "d1", entity.StepAcceptance, 3*time.Second)

	require.NoError(t, e.Tick(context.Background()))

	etape, statutEtape, statutDemande := etatDemande(t, db, "d1")
	require.Equal(t, "DESACTIVATION", etape)
	require.Equal(t, "EN_COURS", statutEtape)
	require.Equal(t, "EN_COURS", statutDemande)

	// L'historique conserve la trace de l'expiration.
	var origine, statut string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT origine, statut FROM etape_historique
		  WHERE demande_id = 'd1' AND etape = 'ACCEPTATION'`).Scan(&origine, &statut))
	require.Equal(t, "EXPIRATION", origine)
	require.Equal(t, "EXPIRE", statut)
}

func TestExpirationNAvancePasAvantLeDelai(t *testing.T) {
	e, db := moteur(t, func(c *config.Config) { c.EtapeTimeout = time.Hour })
	insererDemande(t, db, "d1", entity.StepAcceptance, time.Minute)

	require.NoError(t, e.Tick(context.Background()))

	etape, _, _ := etatDemande(t, db, "d1")
	require.Equal(t, "ACCEPTATION", etape)
}

func TestExpirationDesactiveeQuandLeDelaiEstNul(t *testing.T) {
	e, db := moteur(t) // EtapeTimeout = 0
	insererDemande(t, db, "d1", entity.StepAcceptance, 48*time.Hour)

	require.NoError(t, e.Tick(context.Background()))

	etape, _, _ := etatDemande(t, db, "d1")
	require.Equal(t, "ACCEPTATION", etape)
}

func TestCycleCompletParExpiration(t *testing.T) {
	// Le portage n°2 du SIT : créé, aucun appel, TERMINE 29 minutes plus tard.
	e, db := moteur(t, func(c *config.Config) { c.EtapeTimeout = time.Nanosecond })
	insererDemande(t, db, "d1", entity.StepAcceptance, time.Second)

	for i := 0; i < 5; i++ {
		require.NoError(t, e.Tick(context.Background()))
	}

	etape, statutEtape, statutDemande := etatDemande(t, db, "d1")
	require.Equal(t, "COMPLETION", etape)
	require.Equal(t, "EXPIRE", statutEtape)
	require.Equal(t, "TERMINE", statutDemande)

	var finalisation *time.Time
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT date_finalisation FROM demande WHERE id = 'd1'`).Scan(&finalisation))
	require.NotNil(t, finalisation)

	// Le numéro a réellement changé d'opérateur au registre national.
	var actuel string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT operateur_actuel_id FROM numero WHERE msisdn = '771000001'`).Scan(&actuel))
	require.Equal(t, seed.OperateurYAS, actuel)
}

// Fenêtre de convergence nulle — le défaut : PlanifierTransition applique la
// transition dans l'appel, de sorte que le handler relise l'étape suivante.
func TestFenetreNulleAppliqueLaTransitionImmediatement(t *testing.T) {
	e, db := moteur(t)
	insererDemande(t, db, "d1", entity.StepDeactivation, time.Second)

	require.NoError(t, e.PlanifierTransition(context.Background(), "d1"))

	etape, _, _ := etatDemande(t, db, "d1")
	require.Equal(t, "ACTIVATION", etape, "la transition est appliquée sans attendre un tick")

	// Rien ne reste à faire au moteur.
	require.NoError(t, e.Tick(context.Background()))
	etape, _, _ = etatDemande(t, db, "d1")
	require.Equal(t, "ACTIVATION", etape)
}

// Fenêtre non nulle : le comportement différé mesuré au SIT v0.3 (R-10).
func TestFenetreNonNullePlanifiePuisApplique(t *testing.T) {
	e, db := moteur(t, func(c *config.Config) {
		c.ConvergenceMin = 0
		c.ConvergenceMax = time.Millisecond
	})
	insererDemande(t, db, "d1", entity.StepDeactivation, time.Second)

	require.NoError(t, e.PlanifierTransition(context.Background(), "d1"))

	// Tant que le moteur n'a pas tourné, l'étape reste la précédente.
	etape, _, _ := etatDemande(t, db, "d1")
	require.Equal(t, "DESACTIVATION", etape)

	require.NoError(t, e.Tick(context.Background()))

	etape, _, _ = etatDemande(t, db, "d1")
	require.Equal(t, "ACTIVATION", etape)

	var origine string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT origine FROM etape_historique
		  WHERE demande_id = 'd1' AND etape = 'DESACTIVATION'`).Scan(&origine))
	require.Equal(t, "ACTION", origine)
}

func TestConvergenceRespecteLeDelai(t *testing.T) {
	e, db := moteur(t, func(c *config.Config) {
		c.ConvergenceMin = time.Hour
		c.ConvergenceMax = time.Hour
	})
	insererDemande(t, db, "d1", entity.StepDeactivation, time.Second)

	require.NoError(t, e.PlanifierTransition(context.Background(), "d1"))
	require.NoError(t, e.Tick(context.Background()))

	etape, _, _ := etatDemande(t, db, "d1")
	require.Equal(t, "DESACTIVATION", etape, "la transition n'est pas encore due")
}

func TestRoutageRecalculeAuPassageEnConfirmation(t *testing.T) {
	e, db := moteur(t)
	insererDemande(t, db, "d1", entity.StepActivation, time.Second)

	require.NoError(t, e.PlanifierTransition(context.Background(), "d1"))
	require.NoError(t, e.Tick(context.Background()))

	var routage string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT routage_info FROM demande WHERE id = 'd1'`).Scan(&routage))
	require.Equal(t, "192", routage, "routage destinataire (YAS) pour un numéro porté")
}

func TestPlaceGeleeSuspendLeMoteur(t *testing.T) {
	// BR-012 : un incident interne gèle le traitement pour tous.
	e, db := moteur(t, func(c *config.Config) { c.EtapeTimeout = time.Nanosecond })
	insererDemande(t, db, "d1", entity.StepAcceptance, time.Second)

	_, err := db.Pool.Exec(context.Background(),
		`INSERT INTO incident (id, operateur_id, type_incident_id, fige_systeme,
		                       description, statut, date_ouverture)
		 VALUES ('i1',$1,$2,true,'panne','EN_COURS',now())`,
		seed.OperateurExpresso, seed.TypeIncidentTechnique)
	require.NoError(t, err)

	gelee, err := e.PlaceGelee(context.Background())
	require.NoError(t, err)
	require.True(t, gelee)

	require.NoError(t, e.Tick(context.Background()))

	etape, _, _ := etatDemande(t, db, "d1")
	require.Equal(t, "ACCEPTATION", etape, "le moteur ne doit rien avancer pendant le gel")
}

func TestIncidentGatewayNeGelePas(t *testing.T) {
	e, db := moteur(t)
	_, err := db.Pool.Exec(context.Background(),
		`INSERT INTO incident (id, operateur_id, type_incident_id, fige_systeme,
		                       description, statut, date_ouverture)
		 VALUES ('i1',$1,$2,false,'timeout','EN_COURS',now())`,
		seed.OperateurYAS, seed.TypeIncidentGateway)
	require.NoError(t, err)

	gelee, err := e.PlaceGelee(context.Background())
	require.NoError(t, err)
	require.False(t, gelee)
}

func TestTransfertRegistreExclutNumerosExclusEtRejetes(t *testing.T) {
	// Un filtre qui disparaîtrait dans transfererAuRegistre ou recalculerRoutage
	// transférerait — ou routerait vers le destinataire — un numéro rejeté :
	// un défaut grave dans un système de portabilité.
	e, db := moteur(t)
	insererDemande(t, db, "d1", entity.StepActivation, time.Second)

	_, err := db.Pool.Exec(context.Background(),
		`INSERT INTO demande_numero (demande_id, numero, statut, exclu)
		 VALUES ('d1','771000002','EN_COURS', true)`)
	require.NoError(t, err)
	_, err = db.Pool.Exec(context.Background(),
		`INSERT INTO demande_numero (demande_id, numero, statut, exclu)
		 VALUES ('d1','771000003','REJETE', false)`)
	require.NoError(t, err)

	require.NoError(t, e.PlanifierTransition(context.Background(), "d1"))
	require.NoError(t, e.Tick(context.Background()))

	operateurActuel := func(msisdn string) string {
		var op string
		require.NoError(t, db.Pool.QueryRow(context.Background(),
			`SELECT operateur_actuel_id FROM numero WHERE msisdn = $1`, msisdn).Scan(&op))
		return op
	}
	routageNumero := func(msisdn string) string {
		var routage string
		require.NoError(t, db.Pool.QueryRow(context.Background(),
			`SELECT routage_info FROM demande_numero WHERE demande_id = 'd1' AND numero = $1`, msisdn).Scan(&routage))
		return routage
	}

	// Le numéro normal a bien changé d'opérateur et porte le préfixe destinataire.
	require.Equal(t, seed.OperateurYAS, operateurActuel("771000001"))
	require.Equal(t, "192", routageNumero("771000001"))

	// L'exclu et le rejeté restent chez l'opérateur source et portent son préfixe.
	require.Equal(t, seed.OperateurOrange, operateurActuel("771000002"), "un numéro exclu ne doit pas être transféré")
	require.Equal(t, "191", routageNumero("771000002"))

	require.Equal(t, seed.OperateurOrange, operateurActuel("771000003"), "un numéro rejeté ne doit pas être transféré")
	require.Equal(t, "191", routageNumero("771000003"))
}

// TestAnnulationPendantConvergenceEnCours pins the outcome of a race Task 17
// inherited rather than introduced: porting.CancelRequestInteractor reads a
// request (entity.CanCancel) outside any lock, before opening its own
// port.UnitOfWork.Do. Task 17b closes the gap this used to leave open:
// RequestGateway.Cancel now guards both of its writes on the demande still
// sitting at the step the caller authorized against, so a convergence that
// applies in the window between that read and Cancel's write loses the
// race instead of overwriting stale information. This test reproduces the
// worst-case interleaving deterministically — no goroutines needed, since
// the race is a read-then-write ordering problem, not a data race in the Go
// sense — by applying the due convergence first and only then issuing the
// same guarded write porting.CancelRequestInteractor.Execute would have
// issued from its own (by-then stale) read.
//
// The result, now that the guard is in place: Cancel refuses
// (port.ErrCancelStepChanged), and the request is left exactly as the
// convergence left it — EN_COURS at DESACTIVATION, not ANNULE — with no
// etape_historique row for DESACTIVATION, a step nobody ever processed.
func TestAnnulationPendantConvergenceEnCours(t *testing.T) {
	e, db := moteur(t)
	insererDemande(t, db, "d1", entity.StepAcceptance, time.Second)

	// An operator accepts the request with a non-zero convergence window:
	// the transition is scheduled, already due.
	_, err := db.Pool.Exec(context.Background(),
		`UPDATE demande SET transition_prevue_a = now() - interval '1 second' WHERE id = 'd1'`)
	require.NoError(t, err)

	// CancelRequestInteractor would read the request here and find it still
	// at ACCEPTATION (entity.CanCancel would authorize it against that step)
	// — read reproduced implicitly: nothing about the state below has
	// changed yet.
	etape, _, statut := etatDemande(t, db, "d1")
	require.Equal(t, "ACCEPTATION", etape)
	require.Equal(t, "EN_COURS", statut)
	stepAutorise := entity.Step(etape)

	// ... then, before Cancel writes, the engine converges the scheduled
	// transition.
	require.NoError(t, e.Tick(context.Background()))
	etape, _, statut = etatDemande(t, db, "d1")
	require.Equal(t, "DESACTIVATION", etape, "convergence moved the request forward")
	require.Equal(t, "EN_COURS", statut)

	// ... and only then does Cancel write, carrying the step it was
	// authorized against (ACCEPTATION, from the stale read above) — the
	// same call porting.CancelRequestInteractor.Execute would make through
	// port.UnitOfWork.Do, made directly here to isolate the race from the
	// authorization that precedes it. The guard on that step no longer
	// matches the request's actual current step, so Cancel refuses.
	gw := postgres.NewRequestGateway(db.Pool)
	err = gw.Cancel(context.Background(), "d1", seed.OperateurOrange, stepAutorise, time.Now())
	require.ErrorIs(t, err, port.ErrCancelStepChanged)

	etape, statutEtape, statutDemande := etatDemande(t, db, "d1")
	require.Equal(t, "EN_COURS", statutDemande, "Cancel refused: the request keeps convergence's own state")
	require.Equal(t, "DESACTIVATION", etape, "the step convergence left it at, untouched by the refused cancel")
	require.Equal(t, "EN_COURS", statutEtape)

	// No etape_historique row was written for DESACTIVATION — nobody ever
	// processed that step, and the refused Cancel must not fabricate one.
	var n int
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM etape_historique
		  WHERE demande_id = 'd1' AND etape = 'DESACTIVATION'`).Scan(&n))
	require.Equal(t, 0, n, "no history row for a step nobody processed")
}
