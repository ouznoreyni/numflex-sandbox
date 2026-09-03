# NumFlex Sandbox

Un double local de l'**API Gateway NumFlex de l'ARTP** (guide v2 du 2026-08-10), écrit pour que
l'intégration puisse être développée et testée sans dépendre de la disponibilité de la recette
ARTP.

## Ce que c'est

- Les **33 routes** du contrat gateway, plus les deux routes d'authentification. Rien d'autre sous
  `/api/gateway/v1` : aucune route de santé, de metrics ou de debug, pour que la surface exposée
  soit exactement celle de la plateforme réelle. Les deux commodités de bac à sable — lire les
  tranches du vivier, purger ses données de test — vivent sous un autre préfixe,
  `/api/sandbox/v1`.
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

## Démarrer sans cloner le dépôt

**C'est le chemin le plus court, et il ne demande que Docker** — ni le code, ni Go, ni `make`. Les
images sont publiées sur Docker Hub.

```bash
docker run --rm -p 8080:8080 ouzdiop268/numflex-sandbox:latest
```

L'image `standalone` embarque son PostgreSQL : rien à installer à côté, rien à mettre en réseau. La
base est initialisée, les migrations jouées, le vivier de numéros ensemencé, puis l'API répond sur
`http://localhost:8080` et la page Swagger sur **`http://localhost:8080/swagger.html`**. Un seul
port. `--rm` jette tout à l'arrêt — ce qu'on veut pour un premier essai. Les images sont
multi-architecture : `amd64` et `arm64`, Apple Silicon compris.

**Comptez environ vingt-cinq secondes avant la première réponse** — `initdb`, les migrations et le
vivier — et autant à chaque lancement, puisque `--rm` jette la base. Rien d'autre à passer : ni
argument, ni volume, ni fichier de configuration.

Le vivier par défaut donne **cent mille numéros par tranche**, soit huit cent mille par opérateur :
de quoi explorer sans jamais l'épuiser. Pour que *tout* numéro bien formé d'une tranche existe —
`771000000` à `771999999`, et ainsi de suite — ajoutez `FULL_NUMBERS=true` :

```bash
docker run --rm -p 8080:8080 ouzdiop268/numflex-sandbox:latest FULL_NUMBERS=true
```

Seize millions de numéros au lieu de huit cent mille : **4 min 20 s** de démarrage et 1,9 Go de
table, mesurés. À réserver aux cas où le numéro à tester est imposé de l'extérieur, et à combiner
avec un volume persistant pour ne payer la facture qu'une fois.

### Le premier portage, en quatre appels

À coller tel quel dans un autre terminal, le conteneur tournant (`jq` sert seulement à lire les
réponses) :

```bash
# 1. s'authentifier — trois comptes existent : yas, orange, expresso
TOKEN=$(curl -s localhost:8080/api/authenticate \
  -H 'Content-Type: application/json' \
  -d '{"username":"yas","password":"yas2026"}' | jq -r .id_token)

# 2. lire le référentiel des opérateurs et retenir deux identifiants
ORANGE=$(curl -s localhost:8080/api/gateway/v1/operateurs -H "Authorization: Bearer $TOKEN" \
  | jq -r '.data[] | select(.nom=="ORANGE") | .id')
YAS=$(curl -s localhost:8080/api/gateway/v1/operateurs -H "Authorization: Bearer $TOKEN" \
  | jq -r '.data[] | select(.nom=="YAS") | .id')

# 3. déclencher l'OTP sur un numéro du vivier — 771000001 est chez ORANGE, jamais porté
curl -s localhost:8080/api/gateway/v1/otp/send -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"numero":"771000001"}'

# 4. créer la demande de portage ORANGE → YAS
curl -s localhost:8080/api/gateway/v1/demandes/particulier \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "{
    \"numero\": \"771000001\",
    \"otpCode\": \"123456\",
    \"operateurSourceId\": \"$ORANGE\",
    \"operateurDestinataireId\": \"$YAS\",
    \"typePortabilite\": \"PREPAID\",
    \"client\": {
      \"nom\": \"Diop\", \"prenom\": \"Awa\", \"dateNaissance\": \"1990-04-12\",
      \"lieuNaissance\": \"Dakar\", \"typePiece\": \"CNI\", \"numeroPiece\": \"1234567890123\"
    }
  }" | jq '{message, id: .data.id, etape: .data.etapeActuelle}'
```

```json
{ "message": "Demande particulier créée avec succès",
  "id": "6a98197f513c174baed51cba", "etape": "ACCEPTATION" }
```

La suite du cycle appartient à ORANGE : réauthentifiez-vous en `orange` pour `/demandes/a-accepter`
puis l'acceptation. **Le jeton n'est pas neutre, chaque étape est réservée à un opérateur précis** —
c'est détaillé plus bas, à la section Postman.

### Cinq pièges du premier essai

- **Le champ de l'OTP s'appelle `otpCode`, pas `code`.** Le code est toujours `123456`
  (`OTP_STATIC_CODE`) : aucun SMS n'est envoyé, et la réponse d'`otp/send` n'atteste rien
  (ANO-021).
- **Un refus métier sort en `500`, pas en `4xx`.** Demande introuvable, opérateur non habilité,
  étape non atteinte : tous en `500` avec `RuntimeException: …` dans `detail`. Ce n'est pas une
  panne, c'est ANO-003, reproduit exprès.
- **Un numéro inventé n'existe pas, et le sandbox le dit mal.** Le vivier est fermé : hors des
  seize millions de numéros ensemencés, aucun MSISDN n'est connu du registre, et la création répond
  `RuntimeException: Le numéro n'appartient pas à l'opérateur source indiqué`
  (`OPERATEUR_SOURCE_INCORRECT`). Un numéro absent ne peut appartenir à aucun opérateur source :
  les quatre créations — `particulier`, `entreprise`, `restitution`, `reverse` — le rejettent
  ainsi, plutôt que par un « numéro introuvable ». Rien ne prévient à l'étape d'avant :
  `otp/send` accepte n'importe quel numéro sans consulter le registre. Le même message sort quand
  le numéro existe mais qu'`operateurSourceId` ne désigne pas son détenteur actuel — `771000001`
  déclaré source EXPRESSO, par exemple. Pour lever le doute,
  `GET /api/sandbox/v1/numeros/tranches?operateur=ORANGE` dit quels numéros existent. Si ce détenteur est le destinataire déclaré,
  c'est `NUMERO_DEJA_CHEZ_DESTINATAIRE` qui l'emporte.
- **Seule l'image `standalone` sert la documentation.** `:latest` part de `scratch` et
  n'embarque aucune page : `/swagger.html` y répond `404`. Dans les deux images,
  `/api/gateway/v1` garde exactement ses 33 routes — la page vit à la racine, à côté du contrat,
  jamais dedans.
- **Par défaut le sandbox est lent, et c'est voulu** : `COMPLETION` répond en ~30 s (ANO-005), une
  étape expire seule au bout de ~349 s (ANO-006), les horodatages dérivent de 9 min (ANO-015). Pour
  explorer sans subir ça, voir le profil calme ci-dessous.

### Garder les données entre deux lancements

```bash
mkdir -p "$PWD/data"
docker run -d --name numflex -p 8080:8080 \
  -v "$PWD/data:/data" \
  ouzdiop268/numflex-sandbox:latest PGDATA=/data
```

Le cluster PostgreSQL s'initialise dans `./data`. Détruire le conteneur et le relancer sur le même
dossier retrouve les demandes créées. Le chemin doit être **absolu** — `$PWD/data`, pas `./data` —
et sur macOS appartenir aux dossiers partagés avec la VM Docker (`/Users`, `/private`, `/tmp`).

Pour repartir de zéro : `docker rm -f numflex && rm -rf ./data`.

### Régler le sandbox sans fichier de configuration

Tout se pose en `-e` ou en argument après le nom de l'image, l'argument l'emportant :

```bash
# écouter ailleurs que sur 8080
docker run --rm -p 9000:9000 ouzdiop268/numflex-sandbox:latest PORT=9000

# profil calme : pas d'expiration, pas de latence, pas de dérive d'horloge
docker run --rm -p 8080:8080 ouzdiop268/numflex-sandbox:latest \
  STEP_TIMEOUT_SECONDS=0 COMPLETION_LATENCY_MS=0 CLOCK_SKEW_SECONDS=0

# servir le contrat tel qu'écrit plutôt que la recette telle que mesurée
docker run --rm -p 8080:8080 ouzdiop268/numflex-sandbox:latest FIDELITY=contract
```

Le tableau complet des variables est à la section [Configuration](#configuration). Les quatre qui
comptent pour un premier contact : `PORT`, `FIDELITY`, `STEP_TIMEOUT_SECONDS`,
`COMPLETION_LATENCY_MS`.

### L'image mince, avec votre propre PostgreSQL

Si vous avez déjà une base, `:slim` pèse ~11 Mo au lieu de ~44 : elle part de `scratch`, sans
shell ni gestionnaire de paquets. Elle attend `DATABASE_URL`, son seul réglage sans défaut, et joue
les migrations elle-même au démarrage.

```bash
docker run -d --name numflex -p 8080:8080 \
  -e DATABASE_URL='postgres://numflex:numflex@ma-base:5432/numflex?sslmode=disable' \
  ouzdiop268/numflex-sandbox:slim
```

| Image | Taille | Ce qu'elle contient |
|---|---|---|
| `ouzdiop268/numflex-sandbox:latest` | **~44 Mo** | Le serveur, PostgreSQL **et** la page Swagger, sur un seul port. Rien à orchestrer. C'est ce que `docker run` sans plus de précision vous donne. |
| `ouzdiop268/numflex-sandbox:slim` | **~11 Mo** | Le serveur seul, sur `scratch`. Base à fournir. |

Chaque publication pose aussi un tag de version figé : `:defcon-1` à côté de `:latest`,
`:slim-defcon-1` à côté de `:slim`. `latest` et `slim` suivent le dernier build ; les tags de
version, eux, ne bougent plus et correspondent à un commit.

Les tailles sont celles que Docker transfère. `docker images` en annonce de tout autres — sa colonne
`DISK USAGE` compte les attestations de build et le cache, et affichait 457 Mo pour cette image de
44. `docker image inspect --format '{{.Size}}'` donne le bon chiffre, et c'est ce que le Makefile
imprime maintenant.

L'image tout-en-un est une commodité de démonstration, **pas** un durcissement : shell,
gestionnaire de paquets, démarrage sous root le temps de l'`initdb`. Pour un déploiement, c'est
`:slim` et une base à part.

### La documentation de l'API

**L'image `standalone` la sert elle-même, sur le même port que l'API** :

```
http://localhost:8080/swagger.html      la page Swagger, spécification inlinée
http://localhost:8080/openapi.yaml      la spécification seule
http://localhost:8080/openapi.json
```

Le « Try it out » y fonctionne sans rien configurer : le sandbox répond
`Access-Control-Allow-Origin: *` par défaut.

**Ces trois chemins sont à la racine, jamais sous `/api/gateway/v1`**, qui garde exactement ses 33
routes. Ils n'existent que parce que l'image embarque le dossier `docs/` — l'image mince `:latest`,
qui n'embarque rien, n'en enregistre aucun. `DOCS_ENABLED=false` les éteint et rend au conteneur la
surface exacte de la plateforme :

```bash
docker run --rm -p 8080:8080 ouzdiop268/numflex-sandbox:latest DOCS_ENABLED=false
```

Pas de reverse proxy dans le conteneur, et c'est un choix mesuré : un nginx devant ajouterait
`Server` et `Connection` aux réponses des 33 routes du contrat, qui n'en portent aujourd'hui que
trois — `Content-Type`, `Date`, `Content-Length`. La fidélité de ces réponses vaut plus que trois
lignes de plus dans la table de routage.

Avec l'image mince `:latest`, qui n'embarque aucune page, récupérez-la et servez-la vous-même :

```bash
mkdir -p numflex-doc && cd numflex-doc
curl -O https://raw.githubusercontent.com/ouznoreyni/numflex-sandbox/main/docs/swagger.html
python3 -m http.server 8081 --bind 127.0.0.1
```

Ouvrir le fichier directement (`open swagger.html`) suffit pour **lire** la spécification, mais le
« Try it out » y échouera : depuis une origine `file://`, les navigateurs bloquent les appels vers
`http://localhost` quels que soient les en-têtes CORS.

```bash
# la spécification seule, ou la collection Postman
curl -O https://raw.githubusercontent.com/ouznoreyni/numflex-sandbox/main/docs/openapi.yaml
curl -O https://raw.githubusercontent.com/ouznoreyni/numflex-sandbox/main/postman/numflex-sandbox.postman_collection.json
```

Le **CLI régulateur `artp`** est dans les deux images, à côté du serveur. Comme il n'en est pas le
point d'entrée, on le réclame — en lui repassant `DATABASE_URL` : `docker exec` n'hérite pas des
variables que l'entrypoint a exportées au démarrage.

```bash
docker exec -e DATABASE_URL='postgres://numflex:numflex@127.0.0.1:5432/numflex?sslmode=disable' \
  numflex artp reverse list
```

---

## Démarrer depuis le dépôt

```bash
docker compose up
```

Postgres démarre, les migrations sont appliquées, le seed est joué, l'API écoute sur
`http://localhost:8080`. Le `.env` du dépôt, s'il existe, alimente le conteneur ; seule
`DATABASE_URL` est imposée par le compose, la base ne s'appelant pas `localhost` dans son réseau.

### L'image seule

```bash
make image        # ou : docker build --target runtime -t numflex-sandbox .
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
  numflex-sandbox:slim \
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

#### Tout-en-un, base comprise

Une seconde cible du Dockerfile, `standalone`, embarque PostgreSQL à côté du serveur : le
`docker-compose` de ce dépôt réduit à une seule image. Rien à orchestrer, rien à mettre en réseau.

```bash
make image-standalone     # ou : docker build --target standalone -t numflex-sandbox:latest .
make push                 # construit ET publie, multi-arch, en une passe
```

Le répertoire de données de la base se règle **comme n'importe quel autre réglage**, avec la même
précédence que tout le reste — argument, environnement, `.env`, défaut :

```bash
# 1. en argument
docker run --rm -p 8080:8080 -v "$PWD/data:/data" \
  numflex-sandbox:standalone PGDATA=/data

# 2. en variable d'environnement
docker run --rm -p 8080:8080 -v "$PWD/data:/data" \
  -e PGDATA=/data numflex-sandbox:standalone

# 3. dans un .env monté à l'emplacement par défaut — le serveur lit le même fichier
printf 'PGDATA=/data\nFIDELITY=contract\n' > .env
docker run --rm -p 8080:8080 -v "$PWD/data:/data" -v "$PWD/.env:/app/.env:ro" \
  numflex-sandbox:standalone

# 4. dans un .env ailleurs, désigné en argument
docker run --rm -p 8080:8080 -v "$PWD/data:/data" -v "$PWD/prod.env:/cfg.env:ro" \
  numflex-sandbox:standalone --env-file /cfg.env

# 5. rien du tout — données éphémères, elles meurent avec le conteneur
docker run --rm -p 8080:8080 numflex-sandbox:standalone
```

Un chemin absolu seul en premier argument est un raccourci : `… numflex-sandbox:standalone /data`
vaut `PGDATA=/data`, et s'il désigne un fichier existant, c'est le `.env`. C'est le seul argument
que le serveur ne saurait pas lire lui-même — il n'accepte que `--env-file` et `CLE=valeur`.

`make run-standalone` fait tout ça pour toi :

```bash
make run-standalone
make run-standalone DATA=/srv/numflex PORT=9000 ENV_FILE=./prod.env
```

`DATABASE_URL` est la seule variable que cette image tranche d'autorité : la base vit dans le
conteneur, une valeur venue du `.env` pointerait vers une base qu'elle ne gère pas.

C'est une commodité de démonstration, **pas** un durcissement, et il faut l'assumer : 44 Mo au
lieu de 11, un shell et un gestionnaire de paquets dans l'image, un démarrage sous root le temps
de l'`initdb`. Autrement dit tout ce que l'image `runtime` refuse. Pour un déploiement, c'est
`make push` et une base à part. La base n'écoute que sur `127.0.0.1` dans le conteneur et 5432
n'est jamais publié : l'API reste la seule porte.

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
  numflex-sandbox:slim --env-file /config/recette.env
```

`-e ENV_FILE=/config/recette.env` fait la même chose.

**Le CLI `artp`**, dans le même réseau. Il n'est pas le point d'entrée de l'image, donc on le
réclame :

```bash
docker run --rm --network numflex-net --entrypoint /usr/local/bin/artp \
  -e DATABASE_URL='postgres://numflex:numflex@numflex-db:5432/numflex?sslmode=disable' \
  numflex-sandbox:slim reverse list
```

Pour publier :

```bash
docker login
make push                                  # …/numflex-sandbox:latest        (tout-en-un)
make push VERSION=defcon-1                 # :latest **et** :defcon-1
make push-slim VERSION=defcon-1            # :slim **et** :slim-defcon-1
make push-all VERSION=defcon-1             # les deux images, en un appel
make push REGISTRY=harbor.example.com/numflex   # ailleurs
```

`make push` construit et pousse en une passe, pour `linux/amd64` et `linux/arm64` — un manifeste
multi-architecture ne pouvant pas être chargé dans le démon local, il n'y a pas de `make image`
préalable. Le constructeur `buildx` dédié est créé au premier appel.

`latest` et `slim` sont toujours produits et toujours publiés ; `VERSION=…` ajoute un second tag
figé. La garde d'arbre propre **ne vaut que pour ce tag de version** : lui doit rester
reproductible, donc correspondre à un commit, tandis que `latest` est par nature un pointeur mouvant
et se publie depuis un arbre modifié. `ALLOW_DIRTY=1` lève la garde.

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

**Huit cent mille numéros portables pour ORANGE, autant pour YAS**, 1 622 000 lignes en base au
premier démarrage — ou seize millions si `FULL_NUMBERS=true` remplit les tranches. Le préfixe d'une
tranche tient sur trois chiffres et sa terminaison sur six : `771000001` s'y lit `771` + `000001`,
exactement le numéro qu'il était quand les tranches comptaient cinq chiffres. Une tranche part
toujours de `000000` ; ce qui change, c'est où elle s'arrête.

Jamais portés — huit tranches par opérateur, plus les deux tranches historiques `761` et `701`,
conservées pour que les numéros publiés avant l'élargissement restent valides :

| Opérateur | Tranches | Par tranche | Total portable |
|---|---|---|---|
| ORANGE | `771000000`–`771099999` … `778000000`–`778099999` | 100 000 | **800 000** |
| YAS | `781000000`–`781099999` … `788000000`–`788099999` | 100 000 | **800 000** |
| EXPRESSO | `711` … `718`, terminaisons `000000`–`000999` | 1 000 | 8 000 |
| Historiques | `761000000`–`761000999`, `701000000`–`701000999` | 1 000 | 2 000 |

Avec `FULL_NUMBERS=true`, les deux premières lignes passent à `771000000`–`771999999`, un million
par tranche et huit millions par opérateur : tout numéro bien formé d'une tranche existe alors.
EXPRESSO garde ses mille par tranche dans les deux cas — il sert à exercer le portage entre deux
tiers (UC-08), pas à être consommé en volume. `FULL_NUMBERS` et `POOL_NUMBERS_PER_OPERATOR` ne
règlent que les deux premières lignes.

Déjà portés — le **groupe `900`**, une tranche par opérateur, où les quatre scénarios sont empilés
en blocs de mille :

| Bloc | Situation | Rend testable |
|---|---|---|
| `…000000` → `…000999` | porté il y a 30 jours | `DELAI_PORTAGE_NON_RESPECTE` / ANO-002 |
| `…001000` → `…001999` | porté il y a 8 mois | Restitution nominale |
| `…002000` → `…002999` | porté il y a 2 mois | `DELAI_RESTITUTION_NON_RESPECTE` / ANO-020 |
| `…003000` → `…003999` | porté puis déjà restitué | `NUMERO_DEJA_RESTITUE` |

| Tranche | Détenteur actuel | Opérateur d'origine |
|---|---|---|
| `779…` | ORANGE | YAS |
| `789…` | YAS | ORANGE |
| `719…` | EXPRESSO | ORANGE |

`789001001` est donc détenu par YAS, venu d'ORANGE, porté il y a huit mois : ORANGE peut en
demander la restitution. `779000001` est chez ORANGE depuis trente jours : il se heurte au délai de
trois mois. Ces quatre mille numéros par opérateur sont du **matériel de rejet**, pas du vivier
portable — deux de leurs blocs (8 mois, déjà restitué) dépassent tout de même les 3 mois et se
portent normalement.

Ces numéros sont les seuls que le sandbox connaisse : tout autre MSISDN, même bien formé — une
tranche hors liste, ou une terminaison au-delà de ce que la tranche porte — est rejeté à la
création en `OPERATEUR_SOURCE_INCORRECT`, voir les pièges du premier essai.
`GET /api/sandbox/v1/numeros/tranches?operateur=ORANGE` dit exactement où s'arrête chaque tranche
de l'instance qui tourne.

**Ce que coûte ce volume.** Mesuré dans l'image `standalone`, sur Apple Silicon, `initdb` et
migrations comprises :

| | Lignes | Table `numero` | Démarrage à froid |
|---|---|---|---|
| Défaut | 1 622 000 | 193 Mo | **~25 s** |
| `FULL_NUMBERS=true` | 16 022 000 | 1 905 Mo | **4 min 20 s** |

Le seed insère une tranche par instruction (`INSERT … SELECT generate_series`) et **saute une
tranche déjà installée** : une tranche est posée par une instruction unique, donc entière ou pas du
tout, et la présence de son dernier numéro suffit à le savoir. Redémarrage sur la même base,
mesuré : **2 s**. Un volume persistant (`-v $PWD/data:/data`) ne repaie donc jamais le seed ; un
`--rm` le repaie à chaque lancement — vingt-cinq secondes par défaut, quatre minutes et demie avec
les tranches pleines.

```bash
# les tranches pleines, payées une seule fois grâce au volume
docker run -d -p 8080:8080 -v "$PWD/data:/data" \
  ouzdiop268/numflex-sandbox:latest PGDATA=/data FULL_NUMBERS=true
```

La suite de tests, elle, n'utilise jamais ce volume : `testsupport.NewTestDB` tronque et reseed à
**chaque** test, et sème `seed.TestVolumes` — mille par tranche, soit le premier millier de chacune
et tous les numéros cités ici, en un dixième de seconde.

## Configuration

| Variable | Défaut | Effet |
|---|---|---|
| `PORT` | `8080` | Port d'écoute HTTP |
| `DATABASE_URL` | — | **Obligatoire**, aucun défaut |
| `JWT_SECRET` | `numflex-sandbox-dev-secret` | Secret de signature HS512 |
| `JWT_TTL_HOURS` | `24` | Validité du jeton |
| `FIDELITY` | `real` | `real` \| `contract` — voir ci-dessous |
| `STEP_TIMEOUT_SECONDS` | `349` | Expiration d'une étape ; `0` = pas d'expiration |
| `ENGINE_TICK_SECONDS` | `10` | Cadence du moteur |
| `CONVERGENCE_MIN_SECONDS` | `0` | Fenêtre de convergence — voir ci-dessous |
| `CONVERGENCE_MAX_SECONDS` | `0` | `0` = transition appliquée dans la requête |
| `COMPLETION_LATENCY_MS` | `30500` | Latence simulée de `COMPLETION` |
| `CLOCK_SKEW_SECONDS` | `540` | Dérive d'horloge injectée dans les horodatages |
| `OTP_STATIC_CODE` | `123456` | Code OTP accepté |
| `OTP_TTL_SECONDS` | `300` | Validité de l'OTP |
| `OTP_MAX_ATTEMPTS` | `3` | Tentatives de saisie |
| `REVERSE_AUTO_VALIDATION_SECONDS` | `0` | `0` = validation par le CLI `artp` uniquement |
| `DOCS_ENABLED` | `true` | Sert `/swagger.html`, `/openapi.yaml` et `/openapi.json` à la racine — hors contrat, `/api/gateway/v1` garde ses 33 routes. `false` rend la surface exacte de la plateforme |
| `FULL_NUMBERS` | `false` | Remplit chaque tranche portable entière, `000000` à `999999`. Coûte 4 min 20 s de démarrage à froid et 1,9 Go ; à `false`, une tranche porte ses cent mille premiers numéros et le conteneur répond en ~25 s |
| `POOL_NUMBERS_PER_OPERATOR` | `800000` | Numéros jamais portés semés pour ORANGE et pour YAS, répartis sur leurs huit tranches. Entre `8` et `8000000`. Absente, elle suit `FULL_NUMBERS` ; posée, elle l'emporte sur lui |
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
`TestSameScenarioInContractMode`.

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
STEP_TIMEOUT_SECONDS=0 CONVERGENCE_MIN_SECONDS=0 CONVERGENCE_MAX_SECONDS=0
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
| ANO-009 | Le référentiel des motifs de rejet expose le champ `motif`, non `libelle` |
| ANO-010 | `client.lieuNaissance` est documenté facultatif, mais **rejeté s'il est absent** |
| ANO-011 | `POST /otp/send` omet le champ `data` de l'enveloppe au lieu de le porter à `null` |
| ANO-013 | Une étape franchie par une action porte `statutEtapeActuel: TERMINE`, non `VALIDE` |
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

1. **Préfixe de routage EXPRESSO** (`internal/framework/seed/seed.go`) — `191` (ORANGE) et
   `192` (YAS) sont documentés ; `193` pour EXPRESSO est déduit de la série.
2. **Statut `REJETE`** (`internal/entity/porting_request.go`) — ni documenté au cycle de vie, ni observé.
   Une demande refusée doit bien porter un état terminal distinct de `TERMINE`.
3. **Répartition des rôles en restitution**
   (`internal/usecase/creation/create_restitution_request.go`) — le guide ne tranche pas qui est
   source et qui est destinataire ; le sandbox fait de l'opérateur d'origine le destinataire.

D'autres `[HYP]` plus locaux existent (flotte intégralement rejetée, absence de garde de gel sur
l'annulation, messages d'erreur non mesurés) : `grep -rn '\[HYP\]' internal/`.

## Les deux routes hors contrat — `/api/sandbox/v1`

**Ces routes n'existent pas chez l'ARTP.** Elles vivent délibérément sous un autre préfixe, pour que
`/api/gateway/v1` reste exactement la surface de la plateforme réelle : un client qui bascule son
`baseUrl` vers la recette ARTP ne perd que ce qu'il sait ne pas exister là-bas. Toutes deux exigent
un jeton, et figurent dans la page Swagger sous le tag **Sandbox**.

### Lire les tranches du vivier

```bash
GET /api/sandbox/v1/numeros/tranches?operateur=ORANGE    Authorization: Bearer <jeton>
```

`operateur` vaut `ORANGE`, `YAS` ou `EXPRESSO`, insensible à la casse ; absent ou inconnu, la
réponse est un `400` en `problem+json` qui nomme les valeurs acceptées — hors contrat, donc hors
ANO-003.

```json
{ "success": true, "code": "SUCCESS", "message": "Tranches de l'opérateur ORANGE",
  "data": { "operateur": "ORANGE", "operateurId": "6a21745ce6c37b5b5b487ec1",
            "nombreTranches": 9, "totalNumeros": 8004000,
            "tranches": [ { "prefixe": "771", "premier": "771000000", "dernier": "771999999",
                            "total": 1000000, "nature": "JAMAIS_PORTE" } ] } }
```

C'est la réponse à la question que le registre pose mal : un MSISDN hors vivier est rejeté par
`Le numéro n'appartient pas à l'opérateur source indiqué`, exactement comme un numéro existant
déclaré sous le mauvais opérateur source — rien dans la réponse ne distingue les deux cas.

Le décompte est **lu en base**, pas déduit de `POOL_NUMBERS_PER_OPERATOR` : une tranche installée à
un autre volume dit sa vraie taille, et un numéro passé chez son destinataire après un portage
complet est compté chez lui. La `nature` sort des lignes elles-mêmes — une tranche dont les numéros
portent une date de portage est du matériel de rejet.

Compter coûte : mesuré, **2,7 s** sur le vivier plein — **4,6 s** au tout premier appel après
démarrage, cache froid — et quelques millisecondes sur un vivier réduit. Aucun index n'y change
rien : un `(operateur_actuel_id, msisdn)` a été mesuré à 902 Mo pour aucun gain, l'agrégat devant de
toute façon visiter chaque ligne. C'est une route d'introspection, pas une étape du cycle de
portage.

### Purger ses données de test

```bash
DELETE /api/sandbox/v1/demandes     Authorization: Bearer <jeton>
```

Elle est montée sans condition, comme la lecture des tranches : un bac à sable dont le bouton de
remise à zéro attend une variable d'environnement est un bac à sable que personne ne remet à
zéro. Un jeton reste exigé — sans lui, `401`.

```json
{ "success": true, "code": "SUCCESS", "message": "Demandes purgées avec succès",
  "data": { "demandesSupprimees": 12, "numerosRestaures": 9,
            "otpSupprimes": 4, "reverseSupprimees": 1 } }
```

**Le périmètre est `createur_operateur_id`**, pas le filtre de `/mes-demandes`. Une demande
appartient à deux opérateurs à la fois ; seul son créateur l'a fabriquée. Conséquence à connaître :
un Port-IN créé par un partenaire pour exercer votre Port-OUT ne se purge pas depuis votre jeton —
c'est au partenaire de le faire avec le sien.

**Ce qui part, en une transaction :** les demandes créées par l'appelant et tout ce qui en dépend
(`demande_numero`, `demande_client`, `etape_historique`, `confirmation`, par cascade) ; les OTP des
numéros concernés — sans quoi le même numéro ne pourrait pas être redemandé sans repasser par
`otp/send` ; les demandes de reverse de l'appelant, dont la clé étrangère bloquerait sinon la
suppression. Les incidents et le référentiel du seed ne sont pas touchés.

**Le registre national est restauré** pour chaque numéro concerné :

```
operateur_actuel_id  = operateur_origine_id
date_dernier_portage = NULL
deja_restitue        = false
```

C'est ce qui rend l'opération utile : sans cette remise en état, un numéro déjà porté resterait
bloqué par `DELAI_PORTAGE_NON_RESPECTE` pendant trois mois et le scénario ne pourrait pas être
rejoué. La règle est **« le numéro rentre chez lui »**, pas « le numéro retrouve son état de
seed » : pour une tranche ensemencée déjà portée — `77200`, détenue par ORANGE mais d'origine YAS —
la purge la ramène chez YAS, pas chez ORANGE.

## Structure

Le dépôt suit la Clean Architecture canonique : quatre couches, une seule règle de dépendance —
un paquet n'importe qu'un paquet de couche inférieure ou égale — et `test/architecture_test.go`
qui la vérifie sur le graphe d'imports réel, à chaque `make test`.

```
cmd/
  server/            le serveur HTTP : racine de composition
  artp/              le CLI régulateur
internal/
  entity/            couche 0 — règles métier pures, n'importe rien de ce module
  usecase/
    port/            couche 1 — les interfaces dont les interactors ont besoin
    <capacité>/      couche 1 — un interactor par cas d'usage (otp, creation, acceptance…)
  adapter/
    controller/      couche 2 — HTTP → modèle d'entrée, résultat → view model
    presenter/       couche 2 — view model, dans l'un des deux modes de fidélité
    gateway/postgres/ couche 2 — les gateways. Seul endroit qui nomme une colonne française
  framework/
    web/             couche 3 — moteur Gin, middlewares, câblage des 37 routes (36 sans la purge)
    persistence/     couche 3 — pool pgx, migrations, unité de travail
    engine/          couche 3 — le ticker : expiration, convergence, cycle du reverse
    clock/ config/ identifier/ seed/ token/
  testsupport/       base de test, doubles en mémoire, harnais de routeur
test/                scénarios de bout en bout, captures de conformité, garde d'architecture
migrations/          le schéma, en français — voir ADR 0001
scripts/             point d'entrée de l'image `standalone`, génération de la page Swagger
```

Le détail — le diagramme des couches, le trajet complet d'une requête de Gin à pgx et retour, et
la table des vocabulaires qui restent français parce qu'ils *sont* le contrat — est dans
[`docs/architecture.md`](docs/architecture.md). Les quatre décisions structurantes sont dans
[`docs/adr/`](docs/adr/) : colonnes SQL françaises, unité de travail, fidélité portée par les
presenters, tag de build des tests d'intégration.

### Français et anglais : la frontière est la destination du texte

Le dépôt est en anglais — identifiants, commentaires, messages d'assertion, sorties du CLI et des
journaux, jusqu'aux textes passés à `entity.InternalError`. Ce n'est pas une préférence de style :
c'est ce qui rend `grep` utilisable comme test. **Un mot français hors des cas ci-dessous est un
défaut.**

Reste français tout ce qu'un client de l'API peut observer, parce que le sandbox doit être
indiscernable de la plateforme :

| Ce qui reste français | Exemple |
|---|---|
| Chemins de route | `/api/gateway/v1/demandes/a-accepter` |
| Noms et valeurs JSON | `{"idDemande":…,"etapeActuelle":"ACCEPTATION"}` |
| Messages de réponse et de faute | `"Demande particulier créée avec succès"`, `RuntimeException: …`, `VALIDATION_ECHOUEE` |
| Tables et colonnes SQL | `demande.etape_actuelle` — voir [ADR 0001](docs/adr/0001-french-sql-columns.md) |
| Données de référence ensemencées | les motifs de rejet : `Identité non prouvée`, `Numéro Inactif` |

S'y ajoutent, dans des commentaires par ailleurs anglais, les **citations** entre guillemets : noms
des requêtes Postman capturées, extraits du guide ARTP, symboles supprimés que l'historique cite
encore. Traduire l'une de ces chaînes casse la fidélité au contrat, et les captures du 2026-08-27
avec elle.

## Tests

```bash
make test
```

Lève la base de test sur `5433`, joue toute la suite sous le profil déterministe, puis **supprime
le conteneur** — passant ou non : chaque test tronque et reseed cette base, elle ne conserve rien
qui vaille la peine d'être gardé, et le code de retour qui ressort est celui de `go test`, jamais
celui du démontage. La base applicative de `5432` ne bouge pas : c'est `make up` et `make run` qui
la possèdent.

Les tests d'intégration montent un serveur `httptest` sur une base réelle et assertent sur **le code
HTTP et le corps exacts**, jamais sur une abstraction.

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

./artp reverse list             # les demandes de reverse et leur statut
./artp reverse validate <id>    # crée la Demande REVERSE, à CONFIRMATION
./artp reverse reject <id>
./artp seed                     # rejoue le seed (idempotent)
```

Le binaire est aussi dans l'image, à côté du serveur. Comme il n'en est pas le point d'entrée, il
se réclame explicitement — et lit le même `.env` :

```bash
docker compose run --rm --entrypoint /usr/local/bin/artp api reverse list
```

`artp` **ne joue pas les migrations** : le serveur est seul propriétaire du cycle de vie du schéma.
Séquence normale : `docker compose up` d'abord, `artp` ensuite.

Rappel du contrat : sur une demande REVERSE, la `CONFIRMATION` est attendue de **tous** les
opérateurs, destinataire compris, et la `COMPLETION` est **réservée à l'ARTP** — aucun opérateur ne
peut la déclencher par `/demandes/traitement`.

## Swagger

![La page Swagger du sandbox : le bandeau des comptes de test, les 35 opérations groupées par section, et le sélecteur de serveur pointé sur http://localhost:8080](docs/images/swagger.png)

Le serveur sert la page lui-même, à la racine : **`http://localhost:8080/swagger.html`**. Pour la
servir seule, sans lancer l'API :

```bash
make swagger      # → http://localhost:8081/swagger.html
```

`docs/openapi.yaml` décrit les 35 opérations d'après le guide v2 : 33 routes gateway et les deux
routes d'authentification. `make swagger-build` régénère `openapi.json` et `swagger.html` à partir
de ce seul fichier.

**La documentation vit à la racine, hors de la gateway, et c'est délibéré** : `/api/gateway/v1` ne
doit exposer que les 33 routes du contrat — aucune route de doc, de santé ni de metrics, sinon le
sandbox ne présente plus la même surface que la plateforme réelle. Le serveur la sert sur le port de
l'API ; `DOCS_ENABLED=false` la retire, et l'image mince `:latest`, qui n'embarque aucune page, y
répond `404` quoi qu'il arrive.

Deux conséquences à connaître :

- **« Try it out » fonctionne sans rien configurer.** Servie par le serveur lui-même, la page
  appelle l'API sur sa propre origine : aucun en-tête CORS n'entre en jeu. Servie ailleurs —
  `make swagger` sur `8081`, ou le fichier ouvert depuis le disque — l'appel devient cross-origin,
  et le sandbox répond `Access-Control-Allow-Origin: *`, sans rien à configurer.
  **Ce CORS est une commodité de bac à sable, pas un trait du contrat** : la gateway réelle est
  consommée de serveur à serveur, n'émet aucun en-tête CORS, et aucun test du SIT n'a mesuré son
  comportement cross-origin. L'en-tête n'est émis que si la requête porte un `Origin`, donc pour un
  navigateur : un appel de serveur à serveur reçoit exactement les trois en-têtes de la plateforme.
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
