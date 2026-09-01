# Refactoring du sandbox NumFlex en Clean Architecture

Design validé le 2026-09-01. Branche `refactoring`.

## 1. Objectif

Le sandbox fonctionne et il est couvert : 200 fonctions de test, dont 120 sur la
couche HTTP. Ce qu'il n'a pas, c'est une structure qui survive à sa propre
croissance. Trois défauts précis :

1. **Le SQL vit dans les gestionnaires HTTP.** 166 lignes de SQL réparties dans
   `internal/api`. Une règle métier ne peut pas être exercée sans une base
   Postgres réelle, et un gestionnaire ne peut pas être lu sans lire du SQL.
2. **Le vocabulaire est français dans le code comme sur le fil.** Rien ne
   distingue ce qui est *contrat ARTP*, donc figé, de ce qui n'est qu'un choix
   d'implémentation, donc libre.
3. **Des fichiers trop gros.** `demandes_creation.go` fait 468 lignes,
   `demandes_lecture.go` 408, `acceptation.go` 335.

L'objectif est une architecture Clean canonique — les quatre couches d'Uncle
Bob, la règle de dépendance appliquée et vérifiée — avec un code et une
documentation entièrement en anglais, sans qu'un seul octet du contrat ARTP
observable ne change.

## 2. Contraintes non négociables

Le sandbox n'a de valeur que s'il est indiscernable de la plateforme ARTP
réelle. Trois vocabulaires sont donc **figés en français** et ne seront pas
traduits :

| Figé | Exemple | Pourquoi |
|---|---|---|
| Chemins de routes | `/api/gateway/v1/demandes/a-accepter` | le client d'intégration les appelle |
| Champs et valeurs JSON | `{"idDemande":…,"etapeActuelle":"ACCEPTATION"}` | comparés octet à octet aux captures |
| Tables et colonnes SQL | `demande.etape_actuelle` | renommer coûterait une migration sans bénéfice observable |

Ces trois vocabulaires sont protégés par `conformite_captures_test.go`, qui fige
les réponses réellement mesurées sur la plateforme. **Ce fichier ne sera pas
modifié pendant le refactoring** : c'est lui qui prouve, à chaque commit, que
rien d'observable n'a bougé.

Sont également hors périmètre : le schéma de base, les migrations, la collection
Postman, la spécification OpenAPI, le comportement du moteur, et les deux modes
de fidélité.

## 3. Les quatre couches

```
        ┌──────────────────────────────────────────────┐
        │  frameworks & drivers    Gin · pgx · main    │
        │   ┌────────────────────────────────────────┐ │
        │   │ interface adapters                     │ │
        │   │  controllers · presenters · gateways   │ │
        │   │   ┌──────────────────────────────────┐ │ │
        │   │   │ use cases   interactors + ports  │ │ │
        │   │   │   ┌────────────────────────────┐ │ │ │
        │   │   │   │ entities  règles métier    │ │ │ │
        │   │   │   └────────────────────────────┘ │ │ │
        │   │   └──────────────────────────────────┘ │ │
        │   └────────────────────────────────────────┘ │
        └──────────────────────────────────────────────┘
```

**La règle de dépendance** : le code d'une couche ne nomme jamais rien d'une
couche plus externe. `entity` n'importe rien du projet. `usecase` n'importe que
`entity`. `adapter` importe `usecase` et `entity`. `framework` peut tout
importer. La règle est vérifiée par un test, pas par la discipline — voir §9.

Le flux d'un appel :

```
Gin ─► Controller ─► InputBoundary ─► Interactor ─► Gateway ─► pgx
                                          │
                                          ▼
                                    OutputBoundary ─► Presenter ─► ViewModel ─► Gin
```

L'inversion est là : `Interactor` appelle `port.RequestGateway`, une interface
**déclarée dans la couche use case** et implémentée dans
`adapter/gateway/postgres`. La dépendance de compilation pointe vers l'intérieur
alors que le flux de contrôle va vers l'extérieur.

## 4. Arborescence cible

```
cmd/
  server/main.go                 racine de composition HTTP
  artp/main.go                   CLI régulateur

internal/
  entity/                        règles d'entreprise — zéro import du projet
    porting_request.go           PortingRequest, RequestType, RequestStatus, SubscriberType
    step.go                      Step, StepStatus, séquence, NextStep, StepOwner
    eligibility.go               règles d'éligibilité au portage
    authorization.go             CanAccept, CanProcess, CanCancel, ExpectedConfirmers
    operator.go                  Operator, Role
    otp.go                       OneTimePassword et ses règles de validité
    incident.go                  Incident, IncidentScope
    fault.go / fault_catalog.go  le catalogue d'erreurs (ex-apperr, déjà pur)

  usecase/
    port/
      gateway.go                 interfaces de persistance, par agrégat
      unit_of_work.go            frontière transactionnelle
      services.go                Clock, IDGenerator, Engine, MarketState
    porting/                     un fichier par cas d'usage
    query/  otp/  reverse/  incident/  reference/  auth/  sandbox/  platform/

  adapter/
    controller/                  décode le HTTP → modèle d'entrée → input port
    presenter/
      real.go                    enveloppe ARTP — mode de fidélité `real`
      contract.go                problem+json JHipster — mode `contract`
      view_model.go              la vue neutre que Gin sérialise
    gateway/postgres/            implémente port/gateway.go
      mapping.go                 SEUL endroit qui connaît les colonnes françaises

  framework/
    web/                         moteur Gin, routeur, middlewares (auth, CORS)
    persistence/                 pool pgx, migrations, unité de travail
    config/  auth/  engine/  seed/  oid/

  testsupport/                   fixtures, base de test, doubles en mémoire

test/                            scénarios de bout en bout
```

## 5. Catalogue des cas d'usage

36 routes enregistrées : 33 du contrat, 2 d'authentification, 1 hors contrat.
Elles se répartissent en ~30 cas d'usage.

| Paquet | Cas d'usage | Route |
|---|---|---|
| `auth` | `Authenticate` | `POST /api/authenticate` |
| | `DescribeCaller` | `GET /api/authenticate` |
| `reference` | `ListOperators` | `GET /operateurs` |
| | `ListRejectionReasons` | `GET /motifs-rejet` |
| | `ListRequestTypes` | `GET /types-demande` |
| | `ListProcesses` | `GET /processus` |
| | `ListIncidentTypes` | `GET /types-incident` |
| `otp` | `SendOTP` | `POST /otp/send` |
| | `VerifyOTP` | `POST /otp/verify` |
| `porting` | `CreateIndividualRequest` | `POST /demandes/particulier` |
| | `CreateEnterpriseRequest` | `POST /demandes/entreprise` |
| | `CreateRestitutionRequest` | `POST /demandes/restitution` |
| | `AcceptRequest` | `POST /demandes/acceptation` |
| | `AcceptFleetRequest` | `POST /demandes/:id/acceptation` |
| | `ConfirmRequest` | `POST /demandes/a-confirmer` |
| | `ProcessStep` | `POST /demandes/traitement` |
| | `CancelRequest` | `POST /demandes/:id/annuler` |
| `query` | `ListRequestsToAccept` + détail | `GET /demandes/a-accepter[/:id]` |
| | `ListRequestsToProcess` + détail | `GET /demandes/a-traiter[/:id]` |
| | `ListRequestsToConfirm` + détail | `GET /demandes/a-confirmer[/:id]` |
| | `ListConfirmedRequests` | `GET /demandes/deja-confirmees` |
| | `ListIncomingRequests` | `GET /demandes/in` |
| | `ListOutgoingRequests` | `GET /demandes/out` |
| | `ListOwnRequests` | `GET /demandes/mes-demandes` |
| `reverse` | `CreateReverseRequest` | `POST /reverse-requests` |
| | `ListOwnReverseRequests` | `GET /reverse-requests/mes-demandes` |
| | `ValidateReverseRequest` | CLI `artp` |
| `incident` | `DeclareIncident` | `POST /incidents/{gateway,interne}` |
| | `ResolveIncident` | `POST /incidents/{…}/:id/resoudre` |
| | `ListOwnIncidents` | `GET /incidents/{…}/mes-incidents` |
| `sandbox` | `PurgeTestData` | `DELETE /api/sandbox/v1/demandes` |
| `platform` | `ExpireOverdueSteps`, `ConvergePendingTransitions`, `AutoValidateReverses` | moteur, hors HTTP |

Chaque cas d'usage tient dans un fichier portant quatre déclarations : son
modèle d'entrée, son modèle de sortie, son `InputBoundary`, et l'interactor qui
l'implémente. Exemple de forme :

```go
type AcceptRequestInput struct {
    RequestID  string
    CallerID   string
    Accepted   bool
    RejectionReasonID string
}

type AcceptRequestOutput struct {
    Request entity.PortingRequest
    NextStep entity.Step
}

type AcceptRequestBoundary interface {
    Execute(ctx context.Context, in AcceptRequestInput) (AcceptRequestOutput, error)
}

type AcceptRequestInteractor struct {
    requests port.RequestGateway
    work     port.UnitOfWork
    clock    port.Clock
}
```

## 6. Les ports

**Gateways**, un par agrégat : `RequestGateway`, `NumberRegistryGateway`,
`OTPGateway`, `ReverseGateway`, `IncidentGateway`, `ReferenceGateway`,
`OperatorGateway`, `StepHistoryGateway`, `ConfirmationGateway`.

**`UnitOfWork`** est le point de conception le plus délicat. Plusieurs cas
d'usage exigent aujourd'hui une transaction couvrant plusieurs agrégats — la
consommation de l'OTP et la création de la demande sont dans la même
transaction, la purge en touche cinq. Un gateway par agrégat ne doit pas faire
perdre cette garantie. La frontière est donc explicite :

```go
type UnitOfWork interface {
    Do(ctx context.Context, fn func(Repositories) error) error
}

type Repositories struct {
    Requests  RequestGateway
    Numbers   NumberRegistryGateway
    OTP       OTPGateway
    // …
}
```

L'interactor décide de la transaction ; l'adaptateur Postgres décide de ce
qu'est une transaction. Aucune notion de `pgx.Tx` ne remonte dans la couche use
case.

**Services** : `Clock` (l'horloge, dérive de `CLOCK_SKEW_SECONDS` comprise),
`IDGenerator` (l'ObjectID), `Engine` (planification des transitions),
`MarketState` (le gel de la place). Tous injectés, tous doublables en mémoire.

## 7. Presenters et modes de fidélité

Le sandbox rend la même donnée sous deux formes selon `FIDELITY` : enveloppe
ARTP en mode `real`, `problem+json` JHipster en mode `contract`. Aujourd'hui ce
choix est enfoui dans `httpx.Renderer` et dans des tests de fidélité dispersés.

Il devient structurel : l'interactor produit un modèle de sortie neutre, deux
présentateurs le formatent. `presenter.Real` et `presenter.Contract`
implémentent la même interface ; la racine de composition en choisit un selon la
configuration. Un troisième mode ne coûterait qu'un fichier.

C'est la raison principale pour laquelle la forme canonique se rembourse ici :
la séparation présentateur / cas d'usage correspond exactement à une distinction
que le domaine porte déjà.

## 8. Glossaire de renommage

| Aujourd'hui | Demain |
|---|---|
| `Demande` | `PortingRequest` |
| `Etape`, `EtapeSuivante` | `Step`, `NextStep` |
| `StatutDemande`, `StatutEtape` | `RequestStatus`, `StepStatus` |
| `TypeAbonne`, `TypeDemande` | `SubscriberType`, `RequestType` |
| `PeutTraiter`, `PeutAccepter`, `PeutAnnuler` | `CanProcess`, `CanAccept`, `CanCancel` |
| `ResponsableEtape`, `ConfirmateursAttendus` | `StepOwner`, `ExpectedConfirmers` |
| `EndpointEtape` | `StepEndpoint` |
| `Moteur`, `PlaceGelee`, `PlanifierTransition` | `Engine`, `MarketFrozen`, `ScheduleTransition` |
| `Appelant`, `Identite` | `Caller`, `Identity` |
| `apperr` | `entity.Fault` |
| `httpx.Renderer` | `presenter.Real` / `presenter.Contract` |
| `Numero` (registre) | `NumberRegistry` |
| `MotifRejet` | `RejectionReason` |

Les valeurs de chaînes ne changent jamais : `entity.StepAcceptance` vaut
toujours `"ACCEPTATION"`.

Les commentaires passent en anglais. Ils portent le *pourquoi* des écarts
mesurés — les références `ANO-0xx`, `R-10`, `[HYP]` et les renvois au guide sont
conservés à l'identique.

## 9. Tests

La convention Go est conservée : `foo.go` et `foo_test.go` côte à côte. Trois
changements.

**Des tests unitaires qui le sont vraiment.** Les interactors se testent contre
des doubles en mémoire des ports, sans Postgres. C'est impossible aujourd'hui.

**Un tag de build sur ce qui exige une base.** `//go:build integration` sur les
tests de gateway et de bout en bout. `go test ./...` devient honnête : il
n'affiche plus `ok` sur 119 tests silencieusement sautés. `make test` conserve
son comportement actuel et reste la commande de référence.

**Un test qui vérifie la règle de dépendance.** `test/architecture_test.go`
appelle `go list -deps` et échoue si `entity` importe quoi que ce soit du
projet, si `usecase` importe `adapter` ou `framework`, ou si `adapter` importe
`framework`. La règle cesse d'être une intention.

**`internal/testsupport/`** rassemble les fixtures aujourd'hui dupliquées entre
`testutil_test.go` et `harnais_test.go`.

## 10. Documentation

En anglais, contrairement à la présente spec qui est un artefact de
collaboration.

- Un `doc.go` par paquet : son rôle, ses dépendances autorisées, ce qu'il
  n'a pas le droit de connaître.
- `docs/architecture.md` : le schéma des couches, la règle de dépendance, le
  parcours complet d'une requête, et la frontière du contrat.
- `docs/adr/` : les décisions non évidentes — pourquoi les colonnes SQL restent
  en français, pourquoi `UnitOfWork` plutôt qu'un gateway transactionnel,
  pourquoi les présentateurs portent les modes de fidélité.
- README : la section structure mise à jour.

## 11. Exécution

Par tranches verticales : chaque tranche déplace **une capacité de bout en
bout** — extraction du SQL, mise en couches, renommage — et se termine par
`make test` vert. Une tranche, un commit relisible.

1. Socle : `entity` traduit, `port`, `testsupport`, tags d'intégration, test de
   la règle de dépendance
2. Plomberie : routeur, middlewares, présentateurs, unité de travail
3. `otp` — la plus petite capacité, elle valide le patron
4. `reference` puis `query`
5. `porting` : les trois créations
6. `porting` : acceptation, confirmation, traitement, annulation
7. `reverse`, `incident`, `sandbox`
8. `platform` : le moteur
9. Documentation et suppression du code mort

## 12. Critères d'acceptation

- `make test` vert à chaque commit — 200 fonctions de test, aucune supprimée sans remplacement au moins équivalent.
- `conformite_captures_test.go` non modifié entre les tranches 2 et 8.
- `test/architecture_test.go` passe : la règle de dépendance tient.
- Aucune occurrence de `pgx` ni de `gin` hors de `adapter/` et `framework/`.
- `gofmt -l .` vide, `go vet ./...` silencieux.
- Les 36 routes répondent exactement comme avant : mêmes codes, mêmes corps.
- Aucun identifiant ni commentaire français hors des trois vocabulaires figés.
