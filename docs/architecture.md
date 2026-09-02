# Architecture

Le sandbox suit la Clean Architecture canonique : quatre couches, une seule règle
de dépendance, et un test qui la fait respecter.

Le refactoring qui l'a mise en place n'a **rien changé au contrat** : mêmes 36
routes, mêmes codes HTTP, mêmes corps de réponse — `test/conformite_captures_test.go`
fige les réponses réellement capturées contre la plateforme et n'a pas été touché.

## Les quatre couches

```
                    ┌─────────────────────────────────────────────┐
                    │  cmd/ · internal/framework/                  │  3  Frameworks
                    │  Gin, pgx, migrations, moteur, config        │     & drivers
                    │  ┌───────────────────────────────────────┐   │
                    │  │  internal/adapter/                     │   │  2  Interface
                    │  │  controller · presenter · gateway      │   │     adapters
                    │  │  ┌─────────────────────────────────┐   │   │
                    │  │  │  internal/usecase/              │   │   │  1  Use cases
                    │  │  │  interactors + port/            │   │   │
                    │  │  │  ┌───────────────────────────┐  │   │   │
                    │  │  │  │  internal/entity/         │  │   │   │  0  Entities
                    │  │  │  │  règles métier pures      │  │   │   │
                    │  │  │  └───────────────────────────┘  │   │   │
                    │  │  └─────────────────────────────────┘   │   │
                    │  └───────────────────────────────────────┘   │
                    └─────────────────────────────────────────────┘
                              les flèches d'import vont vers l'intérieur
```

| Couche | Paquet | Responsabilité | Ce qu'elle ignore |
|---|---|---|---|
| 0 | `internal/entity` | Règles d'entreprise : étapes, habilitations, éligibilité, catalogue de fautes | HTTP, SQL, mode de fidélité, l'horloge |
| 1 | `internal/usecase/<capacité>` | Un interactor par cas d'usage : orchestration, pas de technologie | Gin, pgx, la forme du JSON sortant |
| 1 | `internal/usecase/port` | Les interfaces dont la couche 1 a besoin : gateways, `UnitOfWork`, `Clock`, `IDGenerator`, `Engine` | Toute implémentation concrète |
| 2 | `internal/adapter/controller` | HTTP → modèle d'entrée, résultat → `presenter.ViewModel` | SQL, règle métier |
| 2 | `internal/adapter/presenter` | Modèle de sortie → view model, dans l'un des deux modes de fidélité | Gin, SQL |
| 2 | `internal/adapter/gateway/postgres` | Les implémentations des gateways. **Seul endroit** qui nomme une table ou colonne française | Gin, mode de fidélité |
| 3 | `internal/framework/web` | Le moteur Gin, les middlewares, le câblage des 36 routes | La règle métier |
| 3 | `internal/framework/persistence` | Le pool pgx, les migrations, l'implémentation de `UnitOfWork`. Seul à construire un `pgxpool.Pool` ou un `pgx.Tx` | Gin |
| 3 | `internal/framework/engine` | Le ticker : expiration (ANO-006), convergence (R-10), cycle du reverse (§6) | Ce que fait un tick — il appelle `usecase/platform` |
| 3 | `cmd/server`, `cmd/artp` | Racines de composition | Tout le reste |

## La règle de dépendance, et le test qui l'applique

**Un paquet ne peut importer qu'un paquet de couche inférieure ou égale.** Rien
d'autre. `internal/entity` n'importe aucun paquet de ce module ; `internal/usecase`
n'importe qu'`entity` ; `internal/adapter` n'importe jamais `internal/framework`.

L'inversion qui rend cela possible tient en une phrase : un interactor ne connaît
pas Postgres, il connaît `port.RequestGateway` — une interface **déclarée dans sa
propre couche** et implémentée deux couches plus loin.

`test/architecture_test.go` la vérifie littéralement, en lisant le graphe
d'imports réel :

- `TestDependencyRule` attribue un numéro de couche à chaque paquet du module et
  échoue dès qu'un import remonte vers l'extérieur.
- `TestEntityIsPure` échoue si `internal/entity` importe quoi que ce soit de ce
  module.

Ces deux tests tournent dans `make test`. Un import fautif ne se discute pas en
revue : il casse la suite.

## Le trajet complet d'une requête

`POST /api/gateway/v1/otp/send`, de Gin à pgx et retour :

1. **`internal/framework/web`** — le moteur Gin reçoit la requête. L'authentification
   n'est pas un middleware de groupe mais un middleware **global gardé par préfixe**
   (`GuardGatewayPrefix`) : un chemin inexistant sous `/api/gateway/v1` doit rendre le
   même 401 qu'un chemin valide, ce qu'un middleware de groupe — que Gin n'exécute que
   pour une route effectivement enregistrée — ne sait pas faire. Il résout le porteur du
   jeton en `entity.Caller` et le dépose dans le contexte. La route, elle, a été câblée
   une seule fois à la construction du routeur, par `Deps.otpController()`.
2. **`internal/adapter/controller`** — `OTPController.Send` lie le JSON, vérifie la
   seule règle qui n'est pas l'affaire de l'interactor (la forme du MSISDN), et
   construit `otp.SendOTPInput`.
3. **`internal/usecase/otp`** — `SendOTPInteractor.Execute` applique la règle
   (durée de validité, code statique du sandbox) et écrit via `port.OTPGateway`.
   Il ne sait pas qu'il y a une base derrière.
4. **`internal/adapter/gateway/postgres`** — `OTPGateway.Save` traduit le modèle en
   `INSERT INTO otp (numero, code, expire_a, …)`. C'est ici, et uniquement ici,
   que le vocabulaire français des colonnes apparaît.
5. **`internal/framework/persistence`** — le `*pgxpool.Pool` exécute. Quand
   plusieurs écritures doivent être atomiques, l'interactor ne voit pas la
   transaction : il appelle `port.UnitOfWork.Do`, et c'est `persistence` qui
   ouvre le `pgx.Tx` et lui rend un `port.Repositories` lié à cette transaction.
6. **Retour** — l'interactor rend un modèle de sortie ou une `*entity.Fault`. Le
   contrôleur passe l'un ou l'autre au `presenter.Presenter` choisi au démarrage
   selon `FIDELITY`, qui produit un `ViewModel` (statut + corps). Le contrôleur
   l'écrit avec `c.JSON`.

La faute ne change jamais de forme en chemin : c'est `*entity.Fault` de bout en
bout, et seul le presenter décide si elle sort en `problem+json` HTTP 500
(`FIDELITY=real`, ANO-003) ou dans l'enveloppe du guide (`FIDELITY=contract`).

## La frontière du contrat

Trois vocabulaires restent français, mot pour mot, parce qu'ils **sont** le
contrat. Tout le reste du code est anglais.

| Vocabulaire | Exemple | Où il a le droit d'apparaître |
|---|---|---|
| Chemins de route | `/api/gateway/v1/demandes/a-accepter` | `internal/framework/web` (câblage), les tests |
| Noms et valeurs JSON | `{"idDemande":…,"etapeActuelle":"ACCEPTATION"}` | Tags de struct dans `internal/adapter/controller` et `presenter` |
| Tables et colonnes SQL | `demande.etape_actuelle` | `internal/adapter/gateway/postgres` uniquement, `mapping.go` en tenant le registre |

Corollaire vérifiable : la valeur des constantes ne bouge pas non plus.
`entity.StepAcceptance` vaut toujours `"ACCEPTATION"` — le nom Go est anglais,
la valeur transmise est celle du contrat.

## Décisions

Les quatre décisions structurantes de cette architecture sont documentées à part :

- [ADR 0001 — les colonnes SQL restent françaises](adr/0001-french-sql-columns.md)
- [ADR 0002 — l'unité de travail](adr/0002-unit-of-work.md)
- [ADR 0003 — le mode de fidélité est porté par les presenters](adr/0003-presenters-carry-fidelity.md)
- [ADR 0004 — les tests d'intégration derrière un tag de build](adr/0004-integration-build-tag.md)
