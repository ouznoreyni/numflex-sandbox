# ADR 0004 — les tests d'intégration derrière un tag de build

*Statut : acceptée, appliquée partiellement. Date : 2026-09-01.*

## Contexte

Les tests d'intégration du sandbox ont besoin d'un Postgres réel : ils montent un
serveur `httptest` sur une base migrée et ensemencée, et assertent sur le code
HTTP et le corps exacts.

Leur helper commun, `testsupport.NewTestDB`, appelle `t.Skip` quand `DATABASE_URL`
est absente. Un `go test ./...` sans base saute donc les tests concernés **et
imprime quand même `ok`**. Un `ok` qui ne prouve rien est pire qu'un échec : il
autorise une revue à conclure « les tests passent » alors que rien de ce qui
touche la base n'a été exécuté.

## Décision

**Les fichiers de test qui exigent une base portent `//go:build integration`,**
et `make test` est la seule commande de vérification qui compte :

```make
test: up
	DATABASE_URL=… go test -tags=integration ./... -p 1 -count=1

test-unit:
	go test ./... -count=1
```

`make test` démarre les deux Postgres (`5432` applicatif, `5433` de test), impose
le profil déterministe (`ETAPE_TIMEOUT_SECONDS=0`, convergence à zéro, pas de
latence de complétion, pas de dérive d'horloge) et joue toute la suite.
`make test-unit` joue le reste, sans Docker, en quelques secondes.

## État d'application

Le tag est posé sur les quinze fichiers des suites de gateway, de contrôleur et
de persistance. Il ne l'est **pas** sur les suites de bout en bout (`test/`), du
moteur (`internal/framework/engine`) ni du seed : celles-là reposent encore sur
le seul `t.Skip` de `NewTestDB`. Un `go test ./...` sans base saute donc toujours
143 tests en affichant `ok`.

C'est une dette connue, pas un oubli silencieux — et c'est précisément ce qui
rend la règle « seul `make test` compte » non négociable tant qu'elle dure. La
combler consiste à ajouter le tag à ces fichiers, en laissant
`test/architecture_test.go` sans tag : il lit le graphe d'imports et n'a jamais
eu besoin de base.

## Conséquences

- `-p 1` est nécessaire : les paquets partagent une base et la vident entre les
  cas. `-count=1` interdit le cache, un test qui touche une base externe n'étant
  pas cachable de façon fiable.
- Sur les paquets déjà tagués, un `ok` sous `make test-unit` dit la vérité : les
  tests qui exigent une base ne sont même pas compilés, donc aucun n'est
  silencieusement sauté.
- Une revue ne peut pas accepter « les tests passent » sur la foi d'un
  `go test ./...`. La contrainte est écrite dans le plan de refactoring, dans le
  README et ici.
- Coût : un tag à ne pas oublier en écrivant un nouveau test d'intégration.
