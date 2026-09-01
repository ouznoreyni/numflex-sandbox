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
// port.UnitOfWork.Do; RequestGateway.Cancel's own UPDATE carries no WHERE
// condition on the current step. If a scheduled convergence applies in the
// window between that read and Cancel's write, Cancel proceeds on stale
// information. This test reproduces the worst-case interleaving
// deterministically — no goroutines needed, since the bug is a
// read-then-write ordering problem, not a data race in the Go sense — by
// applying the due convergence first and only then issuing the same write
// porting.CancelRequestInteractor.Execute would have issued from its own
// stale read.
//
// The result: the request ends up ANNULE (cancelled) while sitting at
// DESACTIVATION — a step nobody ever processed — with a spurious
// etape_historique row claiming DESACTIVATION was itself terminated by an
// "ACTION" origin cancellation. This is a real defect, pre-existing this
// task (Cancel's SQL is unchanged in shape), and is reported rather than
// fixed here — see the task-17 report.
func TestAnnulationPendantConvergenceEnCours(t *testing.T) {
	e, db := moteur(t)
	insererDemande(t, db, "d1", entity.StepAcceptance, time.Second)

	// Un opérateur accepte la demande avec une fenêtre de convergence non
	// nulle : la transition est planifiée, déjà due.
	_, err := db.Pool.Exec(context.Background(),
		`UPDATE demande SET transition_prevue_a = now() - interval '1 second' WHERE id = 'd1'`)
	require.NoError(t, err)

	// L'interactor de CancelRequest lirait ici la demande, la trouverait
	// encore à ACCEPTATION (entity.CanCancel l'autoriserait) — read reproduced
	// implicitly: nothing about the state below has changed yet.
	etape, _, statut := etatDemande(t, db, "d1")
	require.Equal(t, "ACCEPTATION", etape)
	require.Equal(t, "EN_COURS", statut)

	// ... puis, avant que Cancel n'écrive, le moteur fait converger la
	// transition planifiée.
	require.NoError(t, e.Tick(context.Background()))
	etape, _, statut = etatDemande(t, db, "d1")
	require.Equal(t, "DESACTIVATION", etape, "la convergence a fait avancer la demande")
	require.Equal(t, "EN_COURS", statut)

	// ... et enfin Cancel écrit, sans revérifier l'étape courante — le même
	// appel que porting.CancelRequestInteractor.Execute ferait à travers
	// port.UnitOfWork.Do, ici fait directement pour isoler la course de
	// l'autorisation qui la précède.
	gw := postgres.NewRequestGateway(db.Pool)
	require.NoError(t, gw.Cancel(context.Background(), "d1", seed.OperateurOrange, time.Now()))

	etape, statutEtape, statutDemande := etatDemande(t, db, "d1")
	require.Equal(t, "ANNULE", statutDemande, "Cancel a bien écrasé l'état, sans revérifier l'étape")
	require.Equal(t, "DESACTIVATION", etape, "mais l'étape n'a jamais bougé : demande incohérente")
	require.Equal(t, "TERMINE", statutEtape)

	// La ligne d'historique écrite par Cancel porte l'étape DESACTIVATION —
	// alors que personne n'a jamais traité cette étape.
	var origine, historiqueStatut string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT origine, statut FROM etape_historique
		  WHERE demande_id = 'd1' AND etape = 'DESACTIVATION'`).Scan(&origine, &historiqueStatut))
	require.Equal(t, "ACTION", origine)
	require.Equal(t, "TERMINE", historiqueStatut)
}
