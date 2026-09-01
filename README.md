# NumFlex Sandbox

Un double local de l'**API Gateway NumFlex de l'ARTP** (guide v2 du 2026-08-10), écrit pour que
l'intégration puisse être développée et testée sans dépendre de la disponibilité de la recette
ARTP.

## Ce que c'est

- Les **33 routes** du contrat gateway, plus les deux routes d'authentification. Rien d'autre :
  aucune route de santé, de metrics ou de debug, pour que la surface exposée soit exactement celle
  de la plateforme réelle.
- Une **machine à états complète** — `ACCEPTATION` → `DESACTIVATION` → `ACTIVATION` →
  `CONFIRMATION` → `COMPLETION` — avec ses habilitations, son moteur d'expiration et sa fenêtre de
  convergence réglable.
- Un **registre national de numéros** en PostgreSQL : le portage change réellement l'opérateur
  détenteur, et les contrôles d'éligibilité s'appuient dessus.
- Les **anomalies mesurées en recette**, reproduites à l'identique (voir plus bas).
- Les **réponses capturées** contre la plateforme le 2026-08-27 (collection *Num Flex API*),
  reproduites champ pour champ — sous-objet `client`, messages, `statutEtapeActuel`, précision des
  horodatages. Basculer `baseUrl` du sandbox vers l'ARTP ne doit rien changer pour un client.

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
`http://localhost:8080`. Le `.env` du dépôt, s'il existe, alimente le conteneur ; seules
`DATABASE_URL` et `CORS_ALLOWED_ORIGINS` sont imposées par le compose, la base ne s'appelant pas
`localhost` dans son réseau.

### L'image seule

```bash
make image        # ou : docker build -t numflex-sandbox .
```

Le seul réglage sans défaut est `DATABASE_URL` : le serveur refuse de démarrer sans elle.

#### Un démarrage complet

Le sandbox a besoin d'un PostgreSQL, et de rien d'autre : le conteneur de l'API n'écrit sur aucun
disque, tout l'état vit dans la base.

```bash
docker network create numflex-net       # une fois

# La base. C'est elle qui porte les données — l'API n'a besoin d'aucun montage.
# `-v <dossier hôte>:<chemin conteneur>` : le chemin est donné tel quel, il n'y a
# aucun volume à créer au préalable.
docker run -d --name numflex-db --network numflex-net \
  -e POSTGRES_USER=numflex -e POSTGRES_PASSWORD=numflex -e POSTGRES_DB=numflex \
  -v "$PWD/pgdata:/var/lib/postgresql/data" \
  postgres:16-alpine

# L'API. Le .env fournit le socle, -e corrige ce qui dépend du réseau Docker,
# les arguments finaux tranchent.
docker run -d --name numflex-api --network numflex-net \
  -p 8095:8095 \
  -v "$PWD/.env:/app/.env:ro" \
  -e DATABASE_URL='postgres://numflex:numflex@numflex-db:5432/numflex?sslmode=disable' \
  --read-only --cap-drop ALL --security-opt no-new-privileges \
  --restart unless-stopped \
  numflex-sandbox:latest \
  PORT=8095 FIDELITY=contract
```

| Élément | Ce qu'il fait, et pourquoi |
|---|---|
| `-v "$PWD/.env:/app/.env:ro"` | Le fichier de configuration. `/app` est le répertoire de travail de l'image, donc l'endroit où le serveur cherche `.env`. En lecture seule : il n'a rien à y écrire. |
| `-e DATABASE_URL=…` | Posée en variable parce qu'elle dépend du réseau Docker et non du dépôt — l'hôte de la base y est `numflex-db`, pas `localhost`. Elle l'emporte sur le `.env`. |
| `PORT=8095 FIDELITY=contract` | Les arguments, **après** le nom de l'image. Ils l'emportent sur tout le reste : de quoi dévier d'un `.env` partagé le temps d'un lancement, sans le modifier. |
| `-p 8095:8095` | Les deux côtés suivent `PORT` : dans le conteneur, le serveur écoute réellement sur 8095. |
| `--read-only` | Aucun système de fichiers inscriptible. Le serveur ne fait qu'ouvrir la base et lire ses migrations — aucun `tmpfs` à prévoir. |
| `--cap-drop ALL --security-opt no-new-privileges` | L'image tourne déjà sous l'UID 10001, sans shell ni binaire suid ; ces deux drapeaux ferment ce qui restait. |
| `--restart unless-stopped` | Migrations et seed sont idempotents : un redémarrage ne casse ni n'écrase l'état. |

**Aucun volume n'est requis côté API** — pas même pour les migrations, embarquées dans l'image sous
`/app/migrations`. Le seul montage de l'exemple est le `.env`, et il est facultatif : tout passer
en `-e` ou en arguments donne le même résultat.

#### Monter des dossiers de l'hôte

Un chemin en première position de `-v` suffit : Docker le monte tel quel, rien n'est à créer
d'avance. Le chemin doit être absolu — `$PWD/pgdata`, pas `./pgdata`.

```bash
mkdir -p "$PWD/pgdata"

docker run -d --name numflex-db --network numflex-net \
  -e POSTGRES_USER=numflex -e POSTGRES_PASSWORD=numflex -e POSTGRES_DB=numflex \
  -v "$PWD/pgdata:/var/lib/postgresql/data" \
  postgres:16-alpine
```

PostgreSQL initialise son cluster directement dans `pgdata/` — `PG_VERSION`, `base/`, `global/` y
apparaissent, et `docker inspect` rapporte des montages de type `bind`, sans aucun volume nommé.
Le dossier doit exister avant : Docker le crée sinon, mais vide et possédé par `root`.

Sur macOS et Windows, le dossier doit appartenir aux chemins partagés avec la machine virtuelle
Docker — `/Users`, `/private`, `/tmp` par défaut sur macOS — sinon le montage échoue au démarrage.

Pour mémoire, `docker volume create --driver local --opt type=none --opt device=<dossier> --opt
o=bind <nom>` donne un volume *nommé* adossé au même dossier : même résultat à l'exécution, avec en
plus un nom dans `docker volume ls`. Inutile si vous vous contentez du chemin.

**Plusieurs profils plutôt qu'un `.env`** : monter le dossier qui les contient, en lecture seule,
puis désigner le fichier voulu.

```bash
docker run -d --name numflex-api --network numflex-net -p 8097:8097 \
  -v "$PWD/config:/config:ro" \
  numflex-sandbox:latest --env-file /config/recette.env
```

`-e ENV_FILE=/config/recette.env` fait la même chose.

**Le CLI `artp`**, dans le même réseau. Il n'est pas le point d'entrée de l'image, donc on le
réclame :

```bash
docker run --rm --network numflex-net --entrypoint /usr/local/bin/artp \
  -e DATABASE_URL='postgres://numflex:numflex@numflex-db:5432/numflex?sslmode=disable' \
  numflex-sandbox:latest reverse lister
```

Pour publier :

```bash
docker login
make push                                    # docker.io/ouzdiop268/numflex-sandbox:<git describe>
make push REGISTRY=… VERSION=v0.4.0          # ailleurs, sous une version choisie
```

`make push` construit et pousse en une passe, pour `linux/amd64` et `linux/arm64` — un manifeste
multi-architecture ne pouvant pas être chargé dans le démon local, il n'y a pas de `make image`
préalable. Le constructeur `buildx` dédié est créé au premier appel. La cible refuse de publier
depuis un arbre de travail modifié : `ALLOW_DIRTY=1` pour passer outre, la version portant alors le
suffixe `-dirty`.

L'image finale part de `scratch` — deux binaires statiques, les migrations, les racines de
confiance TLS, rien d'autre : ni shell, ni gestionnaire de paquets, ni CVE de base à suivre. Elle
tourne sous l'UID 10001. Deux conséquences pratiques : `docker exec … sh` n'existe pas, et l'image
ne porte **aucun `HEALTHCHECK`** — le sandbox n'expose aucune route de santé, la plateforme réelle
n'en ayant pas, et la surface doit rester identique. Sonder de l'extérieur, par
`POST /api/authenticate`.

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
| `CONVERGENCE_MIN_SECONDS` | `0` | Fenêtre de convergence — voir ci-dessous |
| `CONVERGENCE_MAX_SECONDS` | `0` | `0` = transition appliquée dans la requête |
| `COMPLETION_LATENCY_MS` | `30500` | Latence simulée de `COMPLETION` |
| `CLOCK_SKEW_SECONDS` | `540` | Dérive d'horloge injectée dans les horodatages |
| `OTP_STATIC_CODE` | `123456` | Code OTP accepté |
| `OTP_TTL_SECONDS` | `300` | Validité de l'OTP |
| `OTP_MAX_ATTEMPTS` | `3` | Tentatives de saisie |
| `REVERSE_AUTO_VALIDATION_SECONDS` | `0` | `0` = validation par le CLI `artp` uniquement |
| `CORS_ALLOWED_ORIGINS` | — | Origines autorisées, séparées par des virgules ; vide = aucun en-tête CORS, comme la plateforme réelle. `*` autorise tout |
| `ENV_FILE` | `.env` | Chemin du fichier d'environnement à charger — voir ci-dessous |

### D'où viennent les valeurs

Tout se règle par variables d'environnement, et rien d'autre. Elles peuvent être posées de trois
façons, de la plus forte à la plus faible :

| Source | Exemple |
|---|---|
| **Arguments** du serveur | `docker run numflex-sandbox PORT=9090 FIDELITY=contract` |
| **Environnement** du processus | `docker run -e PORT=9090`, `environment:` de compose, `export PORT=9090` |
| **Fichier `.env`** | `docker run -v $PWD/.env:/app/.env:ro`, `--env-file .env`, ou `.env` à la racine en local |

Une source plus forte l'emporte : un `.env` monté fournit le socle, un `-e` corrige une valeur pour
un lancement, un argument tranche. Le fichier est cherché dans le répertoire courant — `/app` dans
le conteneur — sauf si `ENV_FILE` ou `--env-file <chemin>` en désigne un autre ; un `ENV_FILE`
demandé et introuvable est une erreur de démarrage, un `.env` implicite absent ne l'est pas.

`.env.example` liste toutes les variables avec leur défaut. Sa syntaxe est celle d'un `.env`
usuel — `CLEF=valeur`, `export` toléré, guillemets simples ou doubles, commentaires en début de
ligne. **Un commentaire occupe sa propre ligne** : hors guillemets, tout ce qui suit le `=`
appartient à la valeur, `#` compris, pour qu'un secret n'en soit jamais tronqué.

Le `.env` n'est jamais copié dans l'image (`.dockerignore`) : il se monte ou se passe au démarrage.

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

### Convergence : deux comportements mesurés, tous deux reproductibles

Les réponses capturées contre la plateforme le **2026-08-27** montrent des
transitions **synchrones** : `POST /demandes/acceptation` répond `DESACTIVATION`,
`/demandes/traitement` sur `DESACTIVATION` répond `ACTIVATION`, la dernière
confirmation répond `COMPLETION`. C'est le défaut du sandbox
(`CONVERGENCE_MAX_SECONDS=0`).

Le rapport SIT v0.3, plus ancien, avait mesuré l'inverse : la réponse portait
l'étape *précédant* la transition, qui survenait 1 à 6 min plus tard (R-10).
Donner une valeur non nulle à `CONVERGENCE_MIN/MAX_SECONDS` restaure ce
comportement — utile pour éprouver une intégration contre cette version-là.

Les deux sources sont des mesures ; c'est la plus récente qui fait le défaut.

### Profil déterministe (CI)

```
ETAPE_TIMEOUT_SECONDS=0 CONVERGENCE_MIN_SECONDS=0 CONVERGENCE_MAX_SECONDS=0
COMPLETION_LATENCY_MS=0 CLOCK_SKEW_SECONDS=0
```

Aucune expiration, convergence immédiate, pas de latence : un cycle complet en moins d'une seconde.
C'est le profil utilisé par `make test`.

## Anomalies reproduites

Toutes portent leur identifiant du rapport SIT.

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

Le binaire est aussi dans l'image, à côté du serveur. Comme il n'en est pas le point d'entrée, il
se réclame explicitement — et lit le même `.env` :

```bash
docker compose run --rm --entrypoint /usr/local/bin/artp api reverse lister
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
