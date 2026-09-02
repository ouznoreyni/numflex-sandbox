# ADR 0002 — l'unité de travail, et non le `pgx.Tx` passé de main en main

*Statut : acceptée. Date : 2026-09-01.*

## Contexte

Plusieurs cas d'usage doivent écrire dans plusieurs agrégats sans laisser de
demi-état derrière eux :

- la création de demande consomme l'OTP **et** insère la demande — la garantie
  posée par le commit `643415f` ;
- l'acceptation écrit `etape_historique` puis `demande` ;
- la purge du bac à sable touche cinq tables ;
- une transition de plateforme transfère au registre, recalcule le routage et
  applique les effets de fin de demande.

Avant le refactoring, la transaction voyageait explicitement : trois helpers de
`transitions.go` prenaient un `pgx.Tx` en paramètre. Cela marchait, mais faisait
entrer pgx dans du code de règle métier — et rendait ce code intestable sans base.

## Décision

**Un interactor n'ouvre jamais de transaction ; il en décrit une.**
`internal/usecase/port` déclare :

```go
type UnitOfWork interface {
    Do(ctx context.Context, fn func(Repositories) error) error
}
```

`Repositories` est un jeu de gateways — tous ceux qui peuvent participer à une
transaction. L'interactor appelle `uow.Do(ctx, func(repos port.Repositories) error { … })`
et ne voit rien d'autre.

`internal/framework/persistence` en donne la seule implémentation réelle :
elle ouvre le `pgx.Tx`, construit un `Repositories` dont chaque gateway est lié à
cette transaction, puis commit ou rollback selon le retour de `fn` — panique
comprise, roulée en arrière puis relancée plutôt qu'avalée.

## Conséquences

- `pgx.Tx` n'existe plus que dans `internal/framework/persistence`. La règle de
  dépendance vérifiée par `test/architecture_test.go` en découle mécaniquement.
- La frontière transactionnelle devient **lisible dans l'interactor** : ce qui est
  dans le `Do` commit ensemble, ce qui est au-dessus est une lecture préalable.
  Les créations lisent `port.NumberGateway` et vérifient l'OTP *avant* d'ouvrir la
  transaction, exprès.
- Un interactor se teste contre un double en mémoire qui exécute simplement `fn` :
  aucune base n'est nécessaire pour prouver l'orchestration.
- Le rollback, lui, ne peut se prouver que contre Postgres. C'est le rôle des
  tests de `internal/framework/persistence`, qui font échouer `fn` volontairement
  et vérifient qu'il ne reste rien.
- Coût assumé : `Repositories` grossit à chaque nouveau gateway transactionnel,
  et une méthode ajoutée à un gateway doit l'être aussi sur la version liée à la
  transaction. C'est un coût de compilation, donc impossible à oublier.
