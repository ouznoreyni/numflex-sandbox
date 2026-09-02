# ADR 0001 — les tables et colonnes SQL restent françaises

*Statut : acceptée. Date : 2026-09-01.*

## Contexte

Le refactoring en Clean Architecture a traduit en anglais tous les identifiants
Go et tous les commentaires du dépôt. Trois vocabulaires ne pouvaient pas suivre,
parce qu'ils ne sont pas du code : les chemins de route
(`/api/gateway/v1/demandes/a-accepter`), les noms et valeurs JSON
(`{"idDemande":…,"etapeActuelle":"ACCEPTATION"}`) et le schéma SQL
(`demande.etape_actuelle`).

Pour les deux premiers, la contrainte est évidente : ils sont le contrat ARTP,
et un client qui bascule son `baseUrl` du sandbox vers la plateforme ne doit
rien voir changer. Pour le schéma, la question était ouverte — une migration de
renommage était techniquement possible.

## Décision

**Le schéma reste français, et une seule couche a le droit de le nommer.**

Les tables et colonnes gardent leurs noms (`demande`, `numero`, `etape_actuelle`,
`motif_rejet`, `createur_operateur_id`…). `internal/adapter/gateway/postgres` est
le seul paquet du module autorisé à les écrire ; `mapping.go` y tient le registre
explicite « pour cette table, telle colonne porte tel champ ».

## Conséquences

- La frontière de traduction est **une couche, pas une convention** : au-dessus
  de `internal/adapter/gateway/postgres`, plus un seul identifiant français ; en
  dessous, plus un seul identifiant anglais. La lecture d'un `grep` devient un
  test : un mot français hors de ce paquet, hors chaîne SQL et hors tag JSON, est
  un défaut.
- Les migrations existantes ne sont pas réécrites, donc l'historique du schéma
  reste lisible et aucune base déjà déployée n'a de migration de renommage à
  jouer.
- Le coût est un aller-retour mental à la lecture d'un gateway. C'est le prix
  d'avoir concentré ce coût en un seul endroit au lieu de le disperser.
- Si le schéma devait un jour être traduit, seul `internal/adapter/gateway/postgres`
  changerait — c'est exactement la propriété que cette décision achète.
