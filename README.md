# NumFlex Sandbox

Un double local de l'**API Gateway NumFlex de l'ARTP** (guide v2 du 2026-08-10), écrit pour que
l'intégration YAS puisse être développée et testée sans dépendre de la disponibilité de la recette
ARTP.

## Ce que c'est

- Les **33 routes** du contrat gateway, plus les deux routes d'authentification. Rien d'autre :
  aucune route de santé, de metrics ou de debug, pour que la surface exposée soit exactement celle
  de la plateforme réelle.
- Une **machine à états complète** — `ACCEPTATION` → `DESACTIVATION` → `ACTIVATION` →
  `CONFIRMATION` → `COMPLETION` — avec ses habilitations, son moteur d'expiration et sa convergence
  différée.
- Un **registre national de numéros** en PostgreSQL : le portage change réellement l'opérateur
  détenteur, et les contrôles d'éligibilité s'appuient dessus.
- Les **anomalies mesurées en recette**, reproduites à l'identique (voir plus bas).

## Ce que ce n'est pas

- Ce n'est pas la plateforme ARTP, et ce n'est pas une spécification de ce qu'elle *devrait* faire.
  Quand la recette est incohérente, le sandbox l'est aussi.
- Aucun SMS n'est envoyé. Le code OTP est **statique** : `123456` (variable `OTP_STATIC_CODE`).
- Aucune notification de l'abonné (ANO-022) — ce manque est **reproduit**, pas corrigé.
- Pas de back-office, pas d'UI, pas d'API réseau aval.

## Démarrer

```bash
docker compose up
```

Postgres démarre, les migrations sont appliquées, le seed est joué, l'API écoute sur
`http://localhost:8080`.

### Comptes

| username | mot de passe | opérateur | rôles |
|---|---|---|---|
| `orange` | `orange2026` | ORANGE | `ROLE_OPERATEUR_ADMIN`, `ROLE_USER` |
| `yas` | `yas2026` | YAS | `ROLE_OPERATEUR_ADMIN`, `ROLE_USER` |
| `expresso` | `expresso2026` | EXPRESSO | `ROLE_OPERATEUR_ADMIN`, `ROLE_USER` |

```bash
curl -s localhost:8080/api/authenticate \
  -H 'Content-Type: application/json' \
  -d '{"username":"yas","password":"yas2026"}'
```

Le jeton renvoyé (`id_token`) est un JWT HS512 valable 24 h, à présenter en `Authorization: Bearer`.

### Vivier de numéros

Dix numéros par tranche (`…0001` à `…0010`), chaque tranche rendant une règle exerçable dès le
premier démarrage :

| Tranche | Situation | Rend testable |
|---|---|---|
| `77100xxxx` | ORANGE, jamais porté | Portage nominal ORANGE → YAS |
| `76100xxxx` | YAS, jamais porté | Portage sortant YAS → ORANGE |
| `70100xxxx` | EXPRESSO, jamais porté | Portage impliquant un tiers |
| `77200xxxx` | ORANGE, porté il y a 30 jours | `DELAI_PORTAGE_NON_RESPECTE` / ANO-002 |
| `77300xxxx` | YAS, porté il y a 8 mois depuis ORANGE | Restitution nominale |
| `77400xxxx` | YAS, porté il y a 2 mois depuis ORANGE | `DELAI_RESTITUTION_NON_RESPECTE` / ANO-020 |
| `77500xxxx` | YAS, porté puis déjà restitué | `NUMERO_DEJA_RESTITUE` |

Le seed est idempotent (`ON CONFLICT DO NOTHING`) : il est rejoué à chaque démarrage sans écraser
l'état courant. Pour repartir à zéro, supprimer le volume Postgres.

## Configuration

| Variable | Défaut | Effet |
|---|---|---|
| `PORT` | `8080` | Port d'écoute HTTP |
| `DATABASE_URL` | — | **Obligatoire**, aucun défaut |
| `JWT_SECRET` | `numflex-sandbox-dev-secret` | Secret de signature HS512 |
| `JWT_TTL_HOURS` | `24` | Validité du jeton |
| `FIDELITY` | `real` | `real` \| `contract` — voir ci-dessous |
| `ETAPE_TIMEOUT_SECONDS` | `349` | Expiration d'une étape ; `0` = pas d'expiration |
| `ENGINE_TICK_SECONDS` | `10` | Cadence du moteur |
| `CONVERGENCE_MIN_SECONDS` | `60` | Convergence minimale après traitement |
| `CONVERGENCE_MAX_SECONDS` | `360` | Convergence maximale |
| `COMPLETION_LATENCY_MS` | `30500` | Latence simulée de `COMPLETION` |
| `CLOCK_SKEW_SECONDS` | `540` | Dérive d'horloge injectée dans les horodatages |
| `OTP_STATIC_CODE` | `123456` | Code OTP accepté |
| `OTP_TTL_SECONDS` | `300` | Validité de l'OTP |
| `OTP_MAX_ATTEMPTS` | `3` | Tentatives de saisie |
| `REVERSE_AUTO_VALIDATION_SECONDS` | `0` | `0` = validation par le CLI `artp` uniquement |
| `CORS_ALLOWED_ORIGINS` | — | Origines autorisées, séparées par des virgules ; vide = aucun en-tête CORS, comme la plateforme réelle. `*` autorise tout |

### `FIDELITY=real` reproduit les anomalies de la recette, et c'est voulu

En mode `real` — le défaut — le sandbox se comporte comme la plateforme **mesurée**, pas comme la
plateforme **documentée**. Les erreurs métier sortent en `500`, aucune réponse d'erreur ne porte de
champ `code`, la réponse d'`otp/send` omet `data`, les horodatages dérivent de neuf minutes. C'est
le point du sandbox : une intégration qui passe ici passera en recette.

En mode `contract`, la même machine à états est servie selon le contrat tel qu'écrit — codes HTTP
corrects, enveloppe systématique, catalogue de codes d'erreur renseigné. Utile pour vérifier qu'un
client ne s'est pas rendu dépendant d'une anomalie. **La machine à états est identique dans les
deux modes ; seule la présentation change** — c'est ce que verrouille
`TestMemeScenarioEnModeContrat`.

### Profil déterministe (CI)

```
ETAPE_TIMEOUT_SECONDS=0 CONVERGENCE_MIN_SECONDS=0 CONVERGENCE_MAX_SECONDS=0
COMPLETION_LATENCY_MS=0 CLOCK_SKEW_SECONDS=0
```

Aucune expiration, convergence immédiate, pas de latence : un cycle complet en moins d'une seconde.
C'est le profil utilisé par `make test`.

## Anomalies reproduites

Toutes portent leur identifiant du rapport SIT `YAS-PORT-SIT-001`.

| Identifiant | Comportement reproduit |
|---|---|
| ANO-001 | Aucune réponse d'erreur ne porte de champ `code` — le catalogue ARTP n'est pas implémenté |
| ANO-002 | Re-portage à moins de 3 mois → `500 Unexpected runtime exception`, pas un refus métier propre |
| ANO-003 | Les erreurs métier (demande introuvable, étape non atteinte, opérateur non habilité) sortent en `500` |
| ANO-004 | Le nom de la classe d'exception Java fuit dans le corps d'erreur |
| ANO-005 | `COMPLETION` répond en ~30,5 s, et aucun en-tête `Idempotency-Key` n'est lu |
| ANO-006 | Les étapes expirent seules en ~349 s : un cycle complet s'achève sans qu'aucun opérateur n'agisse |
| ANO-008 | Jeton **invalide ou expiré** → `401` à corps vide, sans `Content-Type` |
| ANO-011 | `POST /otp/send` omet le champ `data` de l'enveloppe au lieu de le porter à `null` |
| ANO-014 | Les états d'OTP sortent en `500` avec des messages libres, hors catalogue |
| ANO-015 | Dérive d'horloge de ~9 min : une demande créée est horodatée dans le futur |
| ANO-016 | Échec d'authentification servi hors enveloppe, en `problem+json` |
| ANO-018 | Le champ `etape` de `/demandes/traitement`, retiré en v2, est accepté et **ignoré en silence** |
| ANO-019 | `deja-confirmees` ne trace pas la confirmation de l'opérateur **source** ; le tiers voit bien les siennes |
| ANO-020 | Restitution à moins de 6 mois → `500` dont le `detail` contient une erreur 400 sérialisée en chaîne |
| ANO-021 | La réponse d'`otp/send` n'atteste pas la remise du SMS — aucun SMS n'est d'ailleurs envoyé |
| ANO-022 | Aucune notification de l'abonné à l'issue du portage |

## Hypothèses assumées

Trois comportements ne sont ni documentés au guide v2, ni mesurés en recette. Ils sont marqués
`[HYP]` dans le code, à l'endroit exact où la décision est prise :

1. **Préfixe de routage EXPRESSO** (`internal/seed/seed.go`) — `191` (ORANGE) et `192` (YAS) sont
   documentés ; `193` pour EXPRESSO est déduit de la série.
2. **Statut `REJETE`** (`internal/domain/demande.go`) — ni documenté au cycle de vie, ni observé.
   Une demande refusée doit bien porter un état terminal distinct de `TERMINE`.
3. **Répartition des rôles en restitution** (`internal/api/demandes_creation.go`) — le guide ne
   tranche pas qui est source et qui est destinataire ; le sandbox fait de l'opérateur d'origine le
   destinataire.

D'autres `[HYP]` plus locaux existent (flotte intégralement rejetée, absence de garde de gel sur
l'annulation, messages d'erreur non mesurés) : `grep -rn '\[HYP\]' internal/`.

## Tests

```bash
make test
```

Démarre les deux Postgres (`5432` applicatif, `5433` de test), puis joue toute la suite sous le
profil déterministe. Les tests d'intégration montent un serveur `httptest` sur une base réelle et
assertent sur **le code HTTP et le corps exacts**, jamais sur une abstraction.

Les cas nommés du SIT sont rejoués tels quels : TC-021, TC-034, TC-036, TC-041, TC-044, TC-050,
TC-062, plus ANO-001 vérifiée en volume et ANO-018 vérifiée sur son effet réel.

`test/e2e_test.go` porte les quatre scénarios de bout en bout : le portage du §10 du guide
(ORANGE → YAS avec confirmation d'EXPRESSO) jusqu'à `TERMINE`, le même portage abouti **par pure
expiration sans aucun appel**, le même scénario rejoué en `FIDELITY=contract`, et la vérification
qu'aucune erreur ne porte de champ `code`.

## Le CLI régulateur `artp`

Le contrat place la validation et le rejet d'une demande de reverse **hors de l'API gateway** : ces
actes sont réservés à l'ARTP, après validation administrative. Le binaire `artp` les porte.

```bash
go build -o artp ./cmd/artp

export DATABASE_URL='postgres://numflex:numflex@localhost:5432/numflex?sslmode=disable'

./artp reverse lister          # les demandes de reverse et leur statut
./artp reverse valider <id>    # crée la Demande REVERSE, à CONFIRMATION
./artp reverse rejeter <id>
./artp seed                    # rejoue le seed (idempotent)
```

`artp` **ne joue pas les migrations** : le serveur est seul propriétaire du cycle de vie du schéma.
Séquence normale : `docker compose up` d'abord, `artp` ensuite.

Rappel du contrat : sur une demande REVERSE, la `CONFIRMATION` est attendue de **tous** les
opérateurs, destinataire compris, et la `COMPLETION` est **réservée à l'ARTP** — aucun opérateur ne
peut la déclencher par `/demandes/traitement`.

## Swagger

```bash
make swagger      # → http://localhost:8081/swagger.html
```

`docs/openapi.yaml` décrit les 35 opérations d'après le guide v2 : 33 routes gateway et les deux
routes d'authentification. `make swagger-build` régénère `openapi.json` et `swagger.html` à partir
de ce seul fichier.

**La documentation est servie hors de la gateway, sur un port distinct, et c'est délibéré** : le
sandbox ne doit exposer que les 33 routes du contrat — aucune route de doc, de santé ni de metrics,
sinon il ne présente plus la même surface que la plateforme réelle.

Deux conséquences à connaître :

- **« Try it out » exige que le CORS soit activé.** L'appel part d'une autre origine
  (`8081` → `8080`) ; sans en-tête `Access-Control-Allow-Origin`, le navigateur le bloque.
  `make run` et `docker compose up` posent `CORS_ALLOWED_ORIGINS=http://localhost:8081` pour vous.
  **Cette variable est une commodité de bac à sable, pas un trait du contrat** : la gateway réelle
  est consommée de serveur à serveur et aucun test du SIT n'a mesuré son comportement cross-origin.
  Laissez-la vide — le défaut — pour retrouver le comportement exact de la plateforme.
- **La spécification décrit le contrat**, donc le sandbox lancé en `FIDELITY=contract`. En
  `FIDELITY=real` — le défaut — les réponses d'erreur diffèrent ; chaque description signale
  l'écart par son identifiant SIT.

## Postman

`postman/` contient une collection calquée sur celle de l'ARTP et son environnement — les 33 routes
gateway et les deux routes d'authentification, groupées par section du guide. Le script de test de
`POST /api/authenticate` enregistre automatiquement `{{token}}`. Basculer du sandbox vers la recette
ARTP ne demande que de changer `{{baseUrl}}`.

**Le jeton n'est pas neutre : chaque étape est réservée à un opérateur précis.** La requête
d'authentification est pré-remplie avec `yas`, qui convient pour créer une demande, l'annuler,
traiter l'`ACTIVATION` et la `COMPLETION`, et lire `/demandes/in`. Il faut se réauthentifier en
`orange` pour tout ce qui revient à la source — acceptation, `DESACTIVATION`, `/demandes/a-accepter`,
`/demandes/out`, restitution, reverse — et en `orange` ou `expresso` pour la confirmation, attendue
de tous **sauf** le destinataire. La description de chaque requête concernée le rappelle. Un appel
émis avec le mauvais compte ne renvoie pas un refus lisible mais un `500` (ANO-003).

## Sources

- Guide d'utilisation — API Gateway NumFlex **v2**, ARTP, 2026-08-10 — le contrat en vigueur.
- `YAS-PORT-SIT-001` v0.3 — rapport SIT : les 22 anomalies et les 53 cas de test.
- `YAS-PORT-TBRD-001` v1.2 — Technical BRD portabilité.
- Spécification du sandbox : `docs/superpowers/specs/2026-08-29-numflex-sandbox-design.md`.
