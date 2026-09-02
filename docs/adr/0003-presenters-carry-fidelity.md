# ADR 0003 — le mode de fidélité est porté par les presenters

*Statut : acceptée. Date : 2026-09-01.*

## Contexte

Le sandbox a deux modes, et c'est sa raison d'être :

- `FIDELITY=real` reproduit ce que la plateforme fait **réellement** en recette —
  `problem+json` JHipster, erreurs métier en HTTP 500 (ANO-003), aucun champ
  `code` (ANO-001) ;
- `FIDELITY=contract` rend ce que le guide **promet**.

Avant le refactoring, ce choix était un `if` en ligne dans
`internal/httpx.Renderer`, qui branchait entre `failReel` et `failContrat` à
chaque erreur rendue, et lisait `c.Request.URL.Path` directement dans le contexte
Gin.

## Décision

**Deux implémentations d'une même interface, choisies une fois au démarrage.**

`internal/adapter/presenter` déclare `Presenter`, avec deux implémentations —
`Real` et `Contract` — et un type de sortie unique, `ViewModel` (statut + corps),
sans dépendance à HTTP. Le choix se fait dans `Deps.presenter()`, à la
construction du routeur ; plus aucun code d'exécution ne teste le mode.

Le presenter reçoit le chemin de la requête **en argument** au lieu de le lire
dans un `*gin.Context`, ce qui le rend indépendant du transport.

## Conséquences

- La différence entre les deux modes est concentrée dans deux fichiers, `real.go`
  et `contract.go`, et se lit comme un diff. Reproduire une anomalie nouvellement
  mesurée ne touche que `real.go`.
- Un interactor ne connaît que `*entity.Fault` : la faute garde la même forme de
  bout en bout, et seul le presenter décide de son rendu. Aucune règle métier ne
  peut dépendre du mode — ce qui est la propriété qu'on veut, une règle étant la
  même dans les deux modes.
- Le presenter se teste sans HTTP : on lui donne une faute, on lit le `ViewModel`.
- Coût : une indirection de plus entre le contrôleur et la réponse. Elle est
  payée une fois par requête et achète la garantie qu'aucun `if fidelity` ne peut
  réapparaître ailleurs.
