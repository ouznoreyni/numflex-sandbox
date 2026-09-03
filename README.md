<h1 align="center">NumFlex Sandbox</h1>

<p align="center">
  <strong>Un double local de l'API Gateway de portabilité mobile de l'ARTP Sénégal.</strong><br>
  Une commande, aucune dépendance, la machine à états complète — et les anomalies de la recette
  reproduites à l'identique.
</p>

<p align="center">
  <img alt="Taille de l'image tout-en-un" src="https://img.shields.io/docker/image-size/ouzdiop268/numflex-sandbox/latest?label=image%20tout-en-un&color=0db7ed">
  <img alt="Tirages Docker Hub" src="https://img.shields.io/docker/pulls/ouzdiop268/numflex-sandbox?color=0db7ed">
  <img alt="Go 1.25" src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white">
  <img alt="Contrat guide v2" src="https://img.shields.io/badge/contrat-guide%20ARTP%20v2-informational">
</p>

<p align="center"><a href="README.en.md">English version</a></p>

---

Intégrer la plateforme de portabilité de l'ARTP demande un environnement qu'on n'a pas toujours :
une recette partagée, des créneaux, des numéros qu'un collègue a déjà consommés. Ce sandbox rend
cet environnement local et jetable. Il expose **exactement** les 33 routes du contrat, exécute le
même cycle `ACCEPTATION → DESACTIVATION → ACTIVATION → CONFIRMATION → COMPLETION`, applique les
mêmes habilitations — et se trompe aux mêmes endroits que la vraie plateforme, parce qu'une
intégration qui contourne une anomalie en recette doit la contourner ici aussi.

## Sommaire

| | |
|---|---|
| **[Démarrer](#démarrer)** · [premier portage](#le-premier-portage-en-quatre-appels) · [pièges](#cinq-pièges-du-premier-essai) | Trente secondes, une commande |
| **[Images](#images-publiées)** | Tout-en-un `:latest`, mince `:slim` |
| **[Configuration](#configuration)** · [fidélité](#fidélité--real-reproduit-la-recette-mesurée) | Une variable, trois sources |
| **[Vivier de numéros](#vivier-de-numéros)** | Quels MSISDN existent, et jusqu'où |
| **[Surface de l'API](#surface-de-lapi)** · [comptes](#comptes) · [hors contrat](#les-deux-routes-hors-contrat) | 33 + 2 + 2 routes |
| **[Anomalies reproduites](#anomalies-reproduites)** | Ce qui est faux exprès |
| **[Documentation](#documentation-de-lapi)** | Swagger, OpenAPI, Postman |
| **[Déployer](#déployer)** · [publier](#publier-les-images) | Image mince, base à part |
| **[Développer](#développer)** · [tests](#tests) · [CLI `artp`](#le-cli-régulateur-artp) | Clean Architecture, suite d'intégration |

---

## Démarrer

```bash
docker run --rm -p 8080:8080 ouzdiop268/numflex-sandbox:latest
```

C'est tout : ni base à installer, ni réseau à créer, ni fichier de configuration. L'image embarque
son PostgreSQL, initialise le cluster, joue les migrations, ensemence le vivier de numéros, puis
sert l'API **et** sa documentation sur un seul port.

| | |
|---|---|
| API | <http://localhost:8080/api/gateway/v1> |
| Documentation | <http://localhost:8080/swagger.html> |
| Premier `200` | **~25 s** après le lancement — `--rm` le refait à chaque fois |

Le vivier par défaut donne **cent mille numéros par tranche**, huit cent mille par opérateur. Pour
que *tout* numéro bien formé d'une tranche existe — `771000000` à `771999999` — ajoutez
`FULL_NUMBERS=true` ; comptez alors 4 min 20 s de démarrage, à ne payer qu'une fois grâce à un
volume :

```bash
docker run -d -p 8080:8080 -v "$PWD/data:/data" \
  ouzdiop268/numflex-sandbox:latest PGDATA=/data FULL_NUMBERS=true
```

### Images publiées

| Image | Taille | Contenu |
|---|---|---|
| `ouzdiop268/numflex-sandbox:latest` | **~44 Mo** | Le serveur, PostgreSQL **et** la page Swagger. Rien à orchestrer. |
| `ouzdiop268/numflex-sandbox:slim` | **~11 Mo** | Le serveur seul, sur `scratch`. Base à fournir. |

`linux/amd64` et `linux/arm64`, Apple Silicon compris. Chaque publication pose en plus un tag de
version figé — `:defcon-1` à côté de `:latest`, `:slim-defcon-1` à côté de `:slim`. Les tags
mouvants suivent le dernier build ; les tags de version correspondent à un commit et ne bougent
plus.

L'image tout-en-un est une **commodité de démonstration, pas un durcissement** : elle embarque un
shell et un gestionnaire de paquets, et démarre sous root le temps de l'`initdb`. Pour un
déploiement, c'est `:slim` et une base à part. Dans les deux cas, PostgreSQL n'écoute que sur
`127.0.0.1` à l'intérieur du conteneur et 5432 n'est jamais publié : l'API reste la seule porte.

### Le premier portage, en quatre appels

À coller dans un autre terminal, le conteneur tournant. `jq` ne sert qu'à lire les réponses.

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

# 3. déclencher l'OTP — 771000001 est chez ORANGE, jamais porté
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
puis l'acceptation. **Le jeton n'est pas neutre — chaque étape est réservée à un opérateur
précis** ; voir [Comptes](#comptes).

### Cinq pièges du premier essai

<details>
<summary>Ce qui surprend tout le monde au premier contact — et pourquoi c'est voulu</summary>

<br>

**1. Le champ de l'OTP s'appelle `otpCode`, pas `code`.** Le code est toujours `123456`
(`OTP_STATIC_CODE`) : aucun SMS n'est envoyé, et la réponse d'`otp/send` n'atteste rien (ANO-021).

**2. Un refus métier sort en `500`, pas en `4xx`.** Demande introuvable, opérateur non habilité,
étape non atteinte : tous en `500` avec `RuntimeException: …` dans `detail`. Ce n'est pas une
panne, c'est ANO-003, reproduit exprès.

**3. Un numéro inventé n'existe pas, et le sandbox le dit mal.** Le vivier est fermé : hors des
numéros ensemencés, aucun MSISDN n'est connu du registre, et la création répond
`RuntimeException: Le numéro n'appartient pas à l'opérateur source indiqué`
(`OPERATEUR_SOURCE_INCORRECT`). Le même message sort quand le numéro existe mais qu'
`operateurSourceId` ne désigne pas son détenteur actuel — rien ne distingue les deux cas. Rien ne
prévient non plus à l'étape d'avant : `otp/send` accepte n'importe quel numéro sans consulter le
registre. Pour lever le doute :
`GET /api/sandbox/v1/numeros/tranches?operateur=ORANGE` dit exactement quels numéros existent.

**4. Seule l'image tout-en-un sert la documentation.** `:slim` part de `scratch` et n'embarque
aucune page : `/swagger.html` y répond `404`. Dans les deux images, `/api/gateway/v1` garde
exactement ses 33 routes — la page vit à la racine, à côté du contrat, jamais dedans.

**5. Par défaut le sandbox est lent, et c'est voulu** : `COMPLETION` répond en ~30 s (ANO-005), une
étape expire seule au bout de ~349 s (ANO-006), les horodatages dérivent de 9 min (ANO-015). Pour
explorer sans subir ça :

```bash
docker run --rm -p 8080:8080 ouzdiop268/numflex-sandbox:latest \
  STEP_TIMEOUT_SECONDS=0 COMPLETION_LATENCY_MS=0 CLOCK_SKEW_SECONDS=0
```

</details>

---

## Configuration

Tout se règle par variables d'environnement, et rien d'autre.

| Variable | Défaut | Effet |
|---|---|---|
| `PORT` | `8080` | Port d'écoute HTTP |
| `DATABASE_URL` | — | **Obligatoire** hors image tout-en-un, qui la tranche elle-même |
| `JWT_SECRET` | `numflex-sandbox-dev-secret` | Secret de signature HS512 |
| `JWT_TTL_HOURS` | `24` | Validité du jeton |
| `FIDELITY` | `real` | `real` \| `contract` — [voir plus bas](#fidélité--real-reproduit-la-recette-mesurée) |
| `STEP_TIMEOUT_SECONDS` | `349` | Expiration d'une étape ; `0` = pas d'expiration |
| `ENGINE_TICK_SECONDS` | `10` | Cadence du moteur |
| `CONVERGENCE_MIN_SECONDS` | `0` | Fenêtre de convergence — [voir plus bas](#convergence--deux-comportements-mesurés) |
| `CONVERGENCE_MAX_SECONDS` | `0` | `0` = transition appliquée dans la requête |
| `COMPLETION_LATENCY_MS` | `30500` | Latence simulée de `COMPLETION` (ANO-005) |
| `CLOCK_SKEW_SECONDS` | `540` | Dérive d'horloge injectée dans les horodatages (ANO-015) |
| `OTP_STATIC_CODE` | `123456` | Code OTP accepté |
| `OTP_TTL_SECONDS` | `300` | Validité de l'OTP |
| `OTP_MAX_ATTEMPTS` | `3` | Tentatives de saisie |
| `REVERSE_AUTO_VALIDATION_SECONDS` | `0` | `0` = validation par le CLI `artp` uniquement |
| `FULL_NUMBERS` | `false` | Remplit chaque tranche portable entière, `000000` à `999999` |
| `POOL_NUMBERS_PER_OPERATOR` | `800000` | Numéros jamais portés par opérateur, entre `8` et `8000000`. Absente, suit `FULL_NUMBERS` ; posée, l'emporte sur lui |
| `DOCS_ENABLED` | `true` | Sert `/swagger.html`, `/openapi.yaml`, `/openapi.json` à la racine |
| `ENV_FILE` | `.env` | Chemin du fichier d'environnement à charger |

`.env.example` les liste toutes, commentées.

### D'où viennent les valeurs

Trois sources, de la plus forte à la plus faible :

| Source | Exemple |
|---|---|
| **Arguments** du serveur | `docker run … numflex-sandbox:latest PORT=9090 FIDELITY=contract` |
| **Environnement** du processus | `docker run -e PORT=9090`, `environment:` de compose, `export PORT=9090` |
| **Fichier `.env`** | `-v $PWD/.env:/app/.env:ro`, `--env-file /cfg.env`, ou `.env` à la racine en local |

Une source plus forte l'emporte : un `.env` monté fournit le socle, un `-e` corrige une valeur pour
un lancement, un argument tranche. Le fichier est cherché dans le répertoire courant — `/app` dans
le conteneur — sauf si `ENV_FILE` ou `--env-file <chemin>` en désigne un autre ; un `ENV_FILE`
demandé et introuvable est une erreur de démarrage, un `.env` implicite absent ne l'est pas.

**Un commentaire occupe sa propre ligne** : hors guillemets, tout ce qui suit le `=` appartient à la
valeur, `#` compris, pour qu'un secret n'en soit jamais tronqué. Le `.env` n'est jamais copié dans
l'image (`.dockerignore`).

Dans l'image tout-en-un, un chemin absolu seul en premier argument est un raccourci :
`… numflex-sandbox:latest /data` vaut `PGDATA=/data`, et s'il désigne un fichier existant, c'est le
`.env`. C'est le seul argument que le serveur ne saurait pas lire lui-même — il n'accepte que
`--env-file` et `CLE=valeur`.

### Fidélité : `real` reproduit la recette mesurée

<details>
<summary>Deux modes, une seule machine à états</summary>

<br>

En mode **`real`** — le défaut — le sandbox se comporte comme la plateforme *mesurée*, pas comme la
plateforme *documentée*. Les erreurs métier sortent en `500`, aucune réponse d'erreur ne porte de
champ `code`, la réponse d'`otp/send` omet `data`, les horodatages dérivent de neuf minutes. C'est
le point du sandbox : une intégration qui passe ici passera en recette.

En mode **`contract`**, la même machine à états est servie selon le contrat tel qu'écrit — codes
HTTP corrects, enveloppe systématique, catalogue de codes d'erreur renseigné. Utile pour vérifier
qu'un client ne s'est pas rendu dépendant d'une anomalie.

**La machine à états est identique dans les deux modes ; seule la présentation change** — c'est ce
que verrouille `TestSameScenarioInContractMode`.

</details>

### Convergence : deux comportements mesurés

<details>
<summary>Pourquoi le défaut applique la transition dans la requête</summary>

<br>

Les réponses capturées contre la plateforme le **2026-08-27** montrent des transitions
**synchrones** : `POST /demandes/acceptation` répond `DESACTIVATION`, `/demandes/traitement` sur
`DESACTIVATION` répond `ACTIVATION`, la dernière confirmation répond `COMPLETION`. C'est le défaut
du sandbox (`CONVERGENCE_MAX_SECONDS=0`).

Le rapport SIT v0.3, plus ancien, avait mesuré l'inverse : la réponse portait l'étape *précédant* la
transition, qui survenait 1 à 6 min plus tard (R-10). Donner une valeur non nulle à
`CONVERGENCE_MIN/MAX_SECONDS` restaure ce comportement — utile pour éprouver une intégration contre
cette version-là.

Les deux sources sont des mesures ; c'est la plus récente qui fait le défaut.

</details>

### Profil déterministe (CI)

```
STEP_TIMEOUT_SECONDS=0 CONVERGENCE_MIN_SECONDS=0 CONVERGENCE_MAX_SECONDS=0
COMPLETION_LATENCY_MS=0 CLOCK_SKEW_SECONDS=0
```

Aucune expiration, convergence immédiate, pas de latence : un cycle complet en moins d'une seconde.
C'est le profil qu'utilise `make test`.

---

## Vivier de numéros

Le registre est **fermé** : seuls les numéros ensemencés existent, et tout autre MSISDN est rejeté à
la création. Le préfixe d'une tranche tient sur trois chiffres et sa terminaison sur six —
`771000001` s'y lit `771` + `000001`. Une tranche part toujours de `000000` ; ce qui change, c'est
où elle s'arrête.

| Opérateur | Tranches jamais portées | Par tranche | Total portable |
|---|---|---|---|
| ORANGE | `771000000`–`771099999` … `778000000`–`778099999` | 100 000 | **800 000** |
| YAS | `781000000`–`781099999` … `788000000`–`788099999` | 100 000 | **800 000** |
| EXPRESSO | `711` … `718`, terminaisons `000000`–`000999` | 1 000 | 8 000 |
| Historiques | `761000000`–`761000999`, `701000000`–`701000999` | 1 000 | 2 000 |

Avec `FULL_NUMBERS=true`, les deux premières lignes passent à `771000000`–`771999999` : un million
par tranche, huit millions par opérateur, et tout numéro bien formé d'une tranche existe. EXPRESSO
garde ses mille par tranche dans les deux cas — il sert à exercer le portage entre deux tiers
(UC-08), pas à être consommé en volume.

<details>
<summary>Le groupe <code>900</code> : du matériel de rejet, pas du vivier</summary>

<br>

Une tranche par opérateur où **tous** les numéros ont déjà été portés, les quatre scénarios empilés
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

`789001001` est détenu par YAS, venu d'ORANGE, porté il y a huit mois : ORANGE peut en demander la
restitution. `779000001` est chez ORANGE depuis trente jours : il se heurte au délai de trois mois.
Deux de ces blocs (8 mois, déjà restitué) dépassent tout de même les 3 mois et se portent
normalement.

</details>

**Ce que coûte le volume**, mesuré dans l'image tout-en-un sur Apple Silicon, `initdb` et migrations
comprises :

| | Lignes | Table `numero` | Démarrage à froid |
|---|---|---|---|
| Défaut | 1 622 000 | 193 Mo | **~25 s** |
| `FULL_NUMBERS=true` | 16 022 000 | 1 905 Mo | **4 min 20 s** |

Le seed insère une tranche par instruction (`INSERT … SELECT generate_series`) et **saute une
tranche déjà installée** — elle est posée entière ou pas du tout, et la présence de son dernier
numéro suffit à le savoir. Redémarrage sur la même base : **2 s**. Un volume persistant ne repaie
donc jamais le seed.

---

## Surface de l'API

**37 routes en tout, et pas une de plus** : les 33 du contrat gateway, les 2 d'authentification, les
2 du bac à sable. Aucune route de santé, de metrics ou de debug — la surface exposée sous
`/api/gateway/v1` doit être exactement celle de la plateforme réelle, et un test de table de routage
le vérifie à chaque build.

| Groupe | Routes |
|---|---|
| Authentification | `POST`/`GET /api/authenticate` |
| Référentiels (§7.1) | `/operateurs`, `/motifs-rejet`, `/types-demande`, `/processus`, `/types-incident` |
| OTP (§7.2) | `/otp/send`, `/otp/verify` |
| Création (§7.3–7.6) | `/demandes/particulier`, `/demandes/entreprise`, `/demandes/restitution`, `/reverse-requests` |
| Consultation (§7.7) | `/demandes/mes-demandes`, `/a-accepter`, `/a-traiter`, `/a-confirmer`, `/deja-confirmees`, `/in`, `/out`, + les trois détails par `:id` |
| Workflow (§7.8–7.11) | `/demandes/acceptation`, `/demandes/:id/acceptation`, `/demandes/a-confirmer`, `/demandes/traitement`, `/demandes/:id/annuler` |
| Incidents (§7.12) | `/incidents/gateway`, `/incidents/interne`, leurs `/:id/resoudre` et `/mes-incidents` |
| **Hors contrat** | `/api/sandbox/v1/numeros/tranches`, `/api/sandbox/v1/demandes` |

### Comptes

| username | mot de passe | opérateur | rôles |
|---|---|---|---|
| `orange` | `orange2026` | ORANGE | `ROLE_OPERATEUR_ADMIN`, `ROLE_USER` |
| `yas` | `yas2026` | YAS | `ROLE_OPERATEUR_ADMIN`, `ROLE_USER` |
| `expresso` | `expresso2026` | EXPRESSO | `ROLE_OPERATEUR_ADMIN`, `ROLE_USER` |

Le jeton renvoyé (`id_token`) est un JWT HS512 valable 24 h, à présenter en `Authorization: Bearer`.

**Chaque étape est réservée à un opérateur précis.** `yas` crée une demande, l'annule, traite
l'`ACTIVATION` et la `COMPLETION`, lit `/demandes/in`. `orange` — la source — accepte, traite la
`DESACTIVATION`, lit `/demandes/a-accepter` et `/demandes/out`, demande une restitution ou un
reverse. La confirmation est attendue de tous **sauf** le destinataire. Un appel émis avec le
mauvais compte ne renvoie pas un refus lisible, mais un `500` (ANO-003).

### Les deux routes hors contrat

Elles n'existent pas chez l'ARTP et vivent sous un autre préfixe, pour que `/api/gateway/v1` reste
exactement la surface de la plateforme : un client qui bascule son `baseUrl` vers la recette ARTP ne
perd que ce qu'il sait ne pas exister là-bas. Toutes deux exigent un jeton et figurent dans la page
Swagger sous le tag **Sandbox**.

```bash
GET    /api/sandbox/v1/numeros/tranches?operateur=ORANGE
DELETE /api/sandbox/v1/demandes
```

<details>
<summary><code>GET /numeros/tranches</code> — quels numéros existent, et jusqu'où</summary>

<br>

`operateur` vaut `ORANGE`, `YAS` ou `EXPRESSO`, insensible à la casse ; absent ou inconnu, la
réponse est un `400` en `problem+json` qui nomme les valeurs acceptées — hors contrat, donc hors
ANO-003.

```json
{ "success": true, "code": "SUCCESS", "message": "Tranches de l'opérateur ORANGE",
  "data": { "operateur": "ORANGE", "operateurId": "6a21745ce6c37b5b5b487ec1",
            "nombreTranches": 9, "totalNumeros": 804000,
            "tranches": [ { "prefixe": "771", "premier": "771000000", "dernier": "771099999",
                            "total": 100000, "nature": "JAMAIS_PORTE" } ] } }
```

C'est la réponse à la question que le registre pose mal : un MSISDN hors vivier est rejeté par
`Le numéro n'appartient pas à l'opérateur source indiqué`, exactement comme un numéro existant
déclaré sous le mauvais opérateur source.

Le décompte est **lu en base**, pas déduit de la configuration : une tranche installée à un autre
volume dit sa vraie taille, et un numéro passé chez son destinataire après un portage complet est
compté chez lui. La `nature` sort des lignes elles-mêmes — une tranche dont les numéros portent une
date de portage est du matériel de rejet.

Compter coûte : **2,7 s** sur le vivier plein (4,6 s au tout premier appel, cache froid), quelques
millisecondes sur un vivier réduit. Aucun index n'y change rien — un `(operateur_actuel_id, msisdn)`
a été mesuré à 902 Mo pour aucun gain, l'agrégat devant de toute façon visiter chaque ligne.

</details>

<details>
<summary><code>DELETE /demandes</code> — purger ses données de test</summary>

<br>

Supprime toutes les demandes **créées par l'opérateur du jeton** et remet le registre national dans
l'état d'avant.

```json
{ "success": true, "code": "SUCCESS", "message": "Demandes purgées avec succès",
  "data": { "demandesSupprimees": 12, "numerosRestaures": 9,
            "otpSupprimes": 4, "reverseSupprimees": 1 } }
```

**Le périmètre est `createur_operateur_id`**, pas le filtre de `/mes-demandes`. Une demande
appartient à deux opérateurs à la fois ; seul son créateur l'a fabriquée. Conséquence : un Port-IN
créé par un partenaire pour exercer votre Port-OUT ne se purge pas depuis votre jeton.

**Ce qui part, en une transaction** : les demandes créées par l'appelant et tout ce qui en dépend
(`demande_numero`, `demande_client`, `etape_historique`, `confirmation`, par cascade) ; les OTP des
numéros concernés — sans quoi le même numéro ne pourrait pas être redemandé sans repasser par
`otp/send` ; les demandes de reverse de l'appelant, dont la clé étrangère bloquerait sinon la
suppression. Les incidents et le référentiel ne sont pas touchés.

**Le registre est restauré** pour chaque numéro concerné : retour à l'opérateur d'origine, date de
dernier portage et indicateur de restitution effacés. C'est ce qui rend un scénario rejouable —
sans cette restitution, un numéro déjà porté resterait bloqué trois mois par
`DELAI_PORTAGE_NON_RESPECTE`.

</details>

---

## Anomalies reproduites

Le sandbox ne corrige pas la plateforme : il la reproduit. Toutes ces anomalies portent leur
identifiant du rapport SIT.

<details>
<summary>Les vingt anomalies, et ce qu'elles font</summary>

<br>

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

</details>

<details>
<summary>Trois hypothèses assumées</summary>

<br>

Ni documentées au guide v2, ni mesurées en recette. Elles sont marquées `[HYP]` dans le code, à
l'endroit exact où la décision est prise.

1. **Préfixe de routage EXPRESSO** (`internal/framework/seed/seed.go`) — `191` (ORANGE) et `192`
   (YAS) sont documentés ; `193` pour EXPRESSO est déduit de la série.
2. **Statut `REJETE`** (`internal/entity/porting_request.go`) — ni documenté au cycle de vie, ni
   observé. Une demande refusée doit bien porter un état terminal distinct de `TERMINE`.
3. **Répartition des rôles en restitution**
   (`internal/usecase/creation/create_restitution_request.go`) — le guide ne tranche pas qui est
   source et qui est destinataire ; le sandbox fait de l'opérateur d'origine le destinataire.

D'autres `[HYP]` plus locaux existent : `grep -rn '\[HYP\]' internal/`.

</details>

### Ce que ce n'est pas

- Ce n'est pas la plateforme ARTP, et ce n'est pas une spécification de ce qu'elle *devrait* faire.
  Quand la recette est incohérente, le sandbox l'est aussi.
- Aucun SMS n'est envoyé. Le code OTP est **statique**.
- Aucune notification de l'abonné (ANO-022) — ce manque est **reproduit**, pas corrigé.
- Pas de back-office, pas d'UI, pas d'API réseau aval.

---

## Documentation de l'API

L'image tout-en-un sert la page elle-même, **sur le port de l'API** :

```
http://localhost:8080/swagger.html      la page Swagger, spécification inlinée
http://localhost:8080/openapi.yaml      la spécification seule
http://localhost:8080/openapi.json
```

![La page Swagger du sandbox : le bandeau des comptes de test, les opérations groupées par section, et le sélecteur de serveur pointé sur http://localhost:8080](docs/images/swagger.png)

Le « Try it out » y fonctionne sans rien configurer. Ces trois chemins sont **à la racine, jamais
sous `/api/gateway/v1`** : ils n'existent que parce que l'image embarque le dossier `docs/`, et
`DOCS_ENABLED=false` les retire. Pas de reverse proxy dans le conteneur, et c'est un choix mesuré —
un nginx devant ajouterait `Server` et `Connection` aux réponses des 33 routes du contrat, qui n'en
portent aujourd'hui que trois.

`docs/openapi.yaml` est la **seule source** ; `make swagger-build` en régénère `openapi.json` et
`swagger.html`. `make swagger` sert la page seule sur `8081`, sans lancer l'API.

**Postman** : `postman/` contient une collection calquée sur celle de l'ARTP, groupée par section du
guide, plus un dossier `sandbox`. Le script de test de `POST /api/authenticate` enregistre
automatiquement `{{token}}` ; basculer vers la recette ARTP ne demande que de changer `{{baseUrl}}`.

```bash
curl -O https://raw.githubusercontent.com/ouznoreyni/numflex-sandbox/main/docs/openapi.yaml
curl -O https://raw.githubusercontent.com/ouznoreyni/numflex-sandbox/main/postman/numflex-sandbox.postman_collection.json
```

---

## Déployer

L'image `:slim` part de `scratch` : deux binaires statiques, les migrations, les racines de
confiance TLS, rien d'autre — ni shell, ni gestionnaire de paquets, ni CVE de base à suivre. Elle
tourne sous l'UID 10001, attend `DATABASE_URL` et joue les migrations elle-même au démarrage.

```bash
docker network create numflex-net

docker run -d --name numflex-db --network numflex-net \
  -e POSTGRES_USER=numflex -e POSTGRES_PASSWORD=numflex -e POSTGRES_DB=numflex \
  -v "$PWD/pgdata:/var/lib/postgresql/data" \
  postgres:16-alpine

docker run -d --name numflex-api --network numflex-net -p 8095:8095 \
  -v "$PWD/.env:/app/.env:ro" \
  -e DATABASE_URL='postgres://numflex:numflex@numflex-db:5432/numflex?sslmode=disable' \
  --read-only --cap-drop ALL --security-opt no-new-privileges \
  --restart unless-stopped \
  ouzdiop268/numflex-sandbox:slim \
  PORT=8095 FIDELITY=contract
```

| Détail | Pourquoi |
|---|---|
| `-v "$PWD/.env:/app/.env:ro"` | `/app` est le répertoire de travail : c'est là que le serveur cherche `.env`. En lecture seule, il n'a rien à y écrire |
| `-e DATABASE_URL=…` | Dépend du réseau Docker, pas du dépôt — l'hôte de la base y est `numflex-db`. L'emporte sur le `.env` |
| `PORT=8095 FIDELITY=contract` | Arguments, **après** le nom de l'image : ils l'emportent sur tout le reste |
| `--read-only` | Aucun système de fichiers inscriptible. Le serveur ne fait qu'ouvrir la base et lire ses migrations |
| `--cap-drop ALL` | L'image tourne déjà sans shell ni binaire suid ; ce drapeau ferme ce qui restait |
| `--restart unless-stopped` | Migrations et seed sont idempotents : un redémarrage n'écrase rien |

**Aucun volume n'est requis côté API**, pas même pour les migrations, embarquées dans l'image. Un
chemin de `-v` doit être **absolu** ; sur macOS il doit appartenir aux dossiers partagés avec la VM
Docker (`/Users`, `/private`, `/tmp`).

L'image ne porte **aucun `HEALTHCHECK`** — le sandbox n'expose aucune route de santé, la plateforme
réelle n'en ayant pas. Sonder de l'extérieur, par `POST /api/authenticate`.

### Publier les images

```bash
docker login
make push                        # …/numflex-sandbox:latest        (tout-en-un)
make push VERSION=defcon-1       # :latest **et** :defcon-1
make push-slim VERSION=defcon-1  # :slim **et** :slim-defcon-1
make push-all VERSION=defcon-1   # les deux images, en un appel
make push REGISTRY=harbor.example.com/numflex
```

Construction et publication en une passe, pour `linux/amd64` et `linux/arm64` — un manifeste
multi-architecture ne pouvant pas être chargé dans le démon local, il n'y a pas de `make image`
préalable. Le constructeur `buildx` dédié est créé au premier appel.

`latest` et `slim` sont toujours produits et publiés ; `VERSION=…` ajoute un tag figé. **La garde
d'arbre propre ne vaut que pour ce tag de version** : lui doit rester reproductible, donc
correspondre à un commit, tandis qu'un pointeur mouvant se publie depuis un arbre modifié.
`ALLOW_DIRTY=1` lève la garde.

---

## Développer

```bash
docker compose up          # Postgres + API, migrations et seed compris
make run                   # le serveur en local, contre le Postgres de compose
make test                  # la suite complète, base de test jetée après
make test-unit             # sans base ni Docker, quelques secondes
make image                 # numflex-sandbox:slim
make image-standalone      # numflex-sandbox:latest
make run-standalone        # construit le tout-en-un et le lance
make run-standalone FULL=1 DATA=$PWD/data PORT=9000
make swagger               # la page seule sur 8081, sans lancer l'API
make swagger-build         # régénère openapi.json et swagger.html
```

Plusieurs profils de configuration plutôt qu'un seul `.env` : monter le dossier qui les contient en
lecture seule, puis désigner le fichier voulu par `--env-file /config/recette.env` — ou
`-e ENV_FILE=/config/recette.env`, qui fait la même chose.

### Structure

Clean Architecture canonique : quatre couches, une seule règle de dépendance — un paquet n'importe
qu'un paquet de couche inférieure ou égale — et `test/architecture_test.go` qui la vérifie sur le
graphe d'imports réel, à chaque `make test`.

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
    web/             couche 3 — moteur Gin, middlewares, câblage des 37 routes
    persistence/     couche 3 — pool pgx, migrations, unité de travail
    engine/          couche 3 — le ticker : expiration, convergence, cycle du reverse
    clock/ config/ identifier/ seed/ token/
  testsupport/       base de test, doubles en mémoire, harnais de routeur
test/                scénarios de bout en bout, captures de conformité, garde d'architecture
migrations/          le schéma, en français — voir ADR 0001
scripts/             point d'entrée de l'image tout-en-un, génération de la page Swagger
```

Le diagramme des couches et le trajet complet d'une requête sont dans
[`docs/architecture.md`](docs/architecture.md). Les quatre décisions structurantes sont dans
[`docs/adr/`](docs/adr/) : colonnes SQL françaises, unité de travail, fidélité portée par les
presenters, tag de build des tests d'intégration.

<details>
<summary>Français et anglais : la frontière est la destination du texte</summary>

<br>

Le dépôt est en anglais — identifiants, commentaires, messages d'assertion, sorties du CLI et des
journaux. Ce n'est pas une préférence de style : c'est ce qui rend `grep` utilisable comme test.
**Un mot français hors des cas ci-dessous est un défaut.**

Reste français tout ce qu'un client de l'API peut observer, parce que le sandbox doit être
indiscernable de la plateforme :

| Ce qui reste français | Exemple |
|---|---|
| Chemins de route | `/api/gateway/v1/demandes/a-accepter` |
| Noms et valeurs JSON | `{"idDemande":…,"etapeActuelle":"ACCEPTATION"}` |
| Messages de réponse et de faute | `"Demande particulier créée avec succès"`, `VALIDATION_ECHOUEE` |
| Tables et colonnes SQL | `demande.etape_actuelle` — voir [ADR 0001](docs/adr/0001-french-sql-columns.md) |
| Données de référence ensemencées | `Identité non prouvée`, `Numéro Inactif` |

S'y ajoutent les **citations** entre guillemets dans des commentaires par ailleurs anglais : noms
des requêtes Postman capturées, extraits du guide ARTP. Traduire l'une de ces chaînes casse la
fidélité au contrat.

</details>

### Tests

```bash
make test
```

Lève la base de test sur `5433`, joue toute la suite sous le profil déterministe, puis **supprime le
conteneur** — passant ou non : chaque test tronque et reseed cette base, elle ne conserve rien qui
vaille la peine d'être gardé, et le code de retour qui ressort est celui de `go test`. La base
applicative de `5432` ne bouge pas : c'est `make up` et `make run` qui la possèdent.

Les tests d'intégration montent un serveur `httptest` sur une base réelle et assertent sur **le code
HTTP et le corps exacts**, jamais sur une abstraction. Les cas nommés du SIT sont rejoués tels
quels : TC-021, TC-034, TC-036, TC-041, TC-044, TC-050, TC-062, plus ANO-001 vérifiée en volume et
ANO-018 vérifiée sur son effet réel.

`test/e2e_test.go` porte quatre scénarios de bout en bout : le portage du §10 du guide
(ORANGE → YAS avec confirmation d'EXPRESSO) jusqu'à `TERMINE`, le même portage abouti **par pure
expiration sans aucun appel**, le même scénario en `FIDELITY=contract`, et la vérification qu'aucune
erreur ne porte de champ `code`.

`make test-unit` joue la suite sans base ni Docker, en quelques secondes.

### Le CLI régulateur `artp`

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

Il est aussi dans les deux images, à côté du serveur. Comme il n'en est pas le point d'entrée, on le
réclame — en lui repassant `DATABASE_URL`, `docker exec` n'héritant pas des variables exportées par
l'entrypoint :

```bash
docker exec -e DATABASE_URL='postgres://numflex:numflex@127.0.0.1:5432/numflex?sslmode=disable' \
  numflex artp reverse list
```

`artp` **ne joue pas les migrations** : le serveur est seul propriétaire du cycle de vie du schéma.

Rappel du contrat : sur une demande REVERSE, la `CONFIRMATION` est attendue de **tous** les
opérateurs, destinataire compris, et la `COMPLETION` est **réservée à l'ARTP** — aucun opérateur ne
peut la déclencher par `/demandes/traitement`.
