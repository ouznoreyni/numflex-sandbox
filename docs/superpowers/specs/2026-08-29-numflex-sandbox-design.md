# NumFlex Sandbox — design

| | |
|---|---|
| Date | 2026-08-29 |
| Statut | Validé — prêt pour plan d'implémentation |
| Objet | Réimplémentation locale de l'API Gateway NumFlex de l'ARTP, fidèle au contrat **et** au comportement mesuré en recette |
| Pile | Go 1.22+ · Gin · PostgreSQL 16 · Docker Compose |

## 1. Pourquoi

`rec-numflex.artp.sn` redirige toute requête vers `e-services.artp.sn` en HTTP 302 depuis le
2026-08-21 : la recette ARTP n'est plus servie. Le développement du backend YAS
(`numflex-backend`) n'a donc plus de cible.

Ce sandbox rend cette cible. Il n'a pas vocation à être une plateforme de portabilité : il a
vocation à **se comporter comme celle de l'ARTP, défauts compris**. Un backend qui passe au vert
contre un sandbox idéalisé casse en recette — c'est précisément ce que le rapport SIT
(`YAS-PORT-SIT-001` v0.3) documente sur 22 anomalies.

**Sources faisant foi**, dans cet ordre :

1. Le comportement **mesuré** au SIT v2 (265 appels, 2026-08-21) — pour le mode `real`.
2. Le *Guide d'utilisation API Gateway NumFlex v2* du 2026-08-10 — pour le mode `contract`.
3. Aucune extrapolation. Tout ce qui n'est ni mesuré ni écrit est marqué **[HYP]** dans ce
   document et dans le code.

## 2. Décisions cadrantes

| # | Décision | Motif |
|---|---|---|
| D-1 | Go + Gin + Postgres | Choix de l'équipe |
| D-2 | Deux modes de fidélité commutables, **défaut `real`** | Développer contre le réel, vérifier l'adaptateur R-12 contre le contrat |
| D-3 | Moteur d'expiration actif, `349 s` par défaut, réglable, `0` = off | ANO-006 est la contrainte qui fonde toute l'architecture à moteur de YAS (ADR-002, FR-008) |
| D-4 | **Aucun endpoint HTTP hors les routes du contrat** | Contrainte utilisateur. Les actes ARTP passent par le CLI `artp` et le moteur ; jamais par HTTP |
| D-5 | Code OTP **statique** (`123456` par défaut) | Pas de SMS dans le sandbox. Expiration et compteur de tentatives restent réels |
| D-6 | `CONFIRMATION` d'un PORTAGE : **tous les opérateurs de la place sauf le destinataire** | Mesuré au SIT — ORANGE confirme, l'étape reste `EN_COURS` ; EXPRESSO, ni source ni destinataire, la solde. Fonde R-13 et UC-08 |
| D-7 | Identifiants générés en **ObjectId hex 24 caractères** | La recette renvoie des ObjectId MongoDB, pas les `dem-abc123` illustratifs du guide |

## 3. Architecture

```
numflex-sandbox/
├── cmd/
│   ├── server/          binaire API Gateway — expose UNIQUEMENT les routes du contrat
│   └── artp/            binaire régulateur (CLI) — actes hors périmètre API
├── internal/
│   ├── config/          lecture d'environnement, valeurs par défaut, validation
│   ├── apperr/          type d'erreur interne unique + catalogue §9 du guide
│   ├── httpx/           deux moteurs de rendu : enveloppe ARTP | problem+json JHipster
│   ├── auth/            JWT HS512 24 h, middleware Bearer, résolution opérateur
│   ├── domain/          règles pures, sans I/O : workflow, éligibilité, habilitations
│   ├── store/           pgx v5, repositories par agrégat
│   ├── api/             handlers Gin, un fichier par section du guide (7.1 → 7.12)
│   ├── engine/          ticker : expiration, convergence différée, actes ARTP automatiques
│   ├── oid/             générateur d'ObjectId
│   └── seed/            référentiels, comptes, vivier de numéros
├── migrations/          golang-migrate, SQL nu
├── test/                tests d'intégration rejouant les cas SIT
├── postman/             collection calquée sur celle de l'ARTP
├── docker-compose.yml
└── Makefile
```

**Règle de dépendance** : `api → domain → apperr`. `domain` ne connaît ni Gin, ni pgx, ni le mode
de fidélité. Le mode de fidélité n'existe que dans `httpx`. Une règle métier ne doit jamais
consulter `config.Fidelity`.

## 4. Les routes — périmètre exact

Aucune autre route n'existe. Toute autre URL renvoie le 404 du mode courant — pour une requête
**authentifiée**. L'authentification s'exécute avant le routage, comme les filtres Spring
Security de la plateforme réelle : une requête non authentifiée sous `/api/gateway/v1` reçoit
donc son 401 même si le chemin n'existe pas.

### Authentification — hors préfixe gateway

| Méthode | Route |
|---|---|
| POST | `/api/authenticate` |
| GET | `/api/authenticate` → `204 No Content` si jeton valide |

### Gateway — préfixe `/api/gateway/v1`

| # | Méthode | Route | §guide |
|---|---|---|---|
| 1 | GET | `/operateurs` | 7.1 |
| 2 | GET | `/motifs-rejet` | 7.1 |
| 3 | GET | `/types-demande` | 7.1 |
| 4 | GET | `/processus` | 7.1 |
| 5 | GET | `/types-incident` | 7.1 |
| 6 | POST | `/otp/send` | 7.2 |
| 7 | POST | `/otp/verify` | 7.2 |
| 8 | POST | `/demandes/particulier` | 7.3 |
| 9 | POST | `/demandes/entreprise` | 7.4 |
| 10 | POST | `/demandes/restitution` | 7.5 |
| 11 | POST | `/reverse-requests` | 7.6 |
| 12 | GET | `/reverse-requests/mes-demandes` | 7.6 |
| 13 | GET | `/demandes/mes-demandes` | 7.7 |
| 14 | GET | `/demandes/a-accepter` | 7.7 |
| 15 | GET | `/demandes/a-accepter/{id}` | 7.7 |
| 16 | GET | `/demandes/a-traiter` | 7.7 |
| 17 | GET | `/demandes/a-traiter/{id}` | 7.7 |
| 18 | GET | `/demandes/a-confirmer` | 7.7 |
| 19 | GET | `/demandes/a-confirmer/{id}` | 7.7 |
| 20 | GET | `/demandes/deja-confirmees` | 7.7 |
| 21 | GET | `/demandes/in` | 7.7 |
| 22 | GET | `/demandes/out` | 7.7 |
| 23 | POST | `/demandes/acceptation` | 7.8 |
| 24 | POST | `/demandes/{id}/acceptation` | 7.8 |
| 25 | POST | `/demandes/a-confirmer` | 7.9 |
| 26 | POST | `/demandes/traitement` | 7.10 |
| 27 | POST | `/demandes/{id}/annuler` | 7.11 |
| 28 | POST | `/incidents/gateway` | 7.12 |
| 29 | POST | `/incidents/gateway/{id}/resoudre` | 7.12 |
| 30 | GET | `/incidents/gateway/mes-incidents` | 7.12 |
| 31 | POST | `/incidents/interne` | 7.12 |
| 32 | POST | `/incidents/interne/{id}/resoudre` | 7.12 |
| 33 | GET | `/incidents/interne/mes-incidents` | 7.12 |

**Pagination** : `13`–`22` n'acceptent aucun paramètre et n'en renvoient aucun (pas de `page`,
`size`, ni total) — conforme à la mesure. `12`, `30` et `33` acceptent `page` et `size`.

## 5. La couche d'erreur

### 5.1 Type interne unique

```go
type Error struct {
    Code    string // code du catalogue §9 — jamais émis en mode real
    Message string // message français exact du guide quand il en fournit un
    Kind    Kind   // Validation | Etat | Acces | Introuvable | Interne
    Fields  []FieldError // renseigné seulement si Kind == Validation
}
```

`domain` ne renvoie que ce type. Il ignore complètement comment il sera rendu.

### 5.2 Rendu `FIDELITY=real` — défaut

Reproduit ANO-001, ANO-003, ANO-004, ANO-008, ANO-016.

**`Kind == Validation` avec `Fields` → HTTP 400**

```json
{
  "type": "https://www.jhipster.tech/problem/constraint-violation",
  "title": "Method argument not valid",
  "status": 400,
  "path": "/api/gateway/v1/demandes/particulier",
  "message": "error.validation",
  "fieldErrors": [
    { "objectName": "demandeParticulierDTO", "field": "client.lieuNaissance",
      "message": "ne doit pas être vide" }
  ]
}
```

`Content-Type: application/problem+json`.

**`Kind == Validation` sans `Fields` → HTTP 400, autre forme**

Une pile Spring/JHipster ne répond jamais `constraint-violation` avec un `fieldErrors` vide :
ce corps est produit par l'échec d'une validation de bean, qui produit toujours au moins un
champ. Une erreur de validation sans détail de champ prend donc la forme générique, en
conservant le statut 400 :

```json
{
  "type": "https://www.jhipster.tech/problem/problem-with-message",
  "title": "Bad Request",
  "status": 400,
  "detail": "Le corps de la requête n'est pas un JSON valide",
  "path": "/api/gateway/v1/demandes/particulier",
  "message": "error.http.400"
}
```

Le préfixe `RuntimeException: ` n'apparaît **pas** ici : c'est la fuite mesurée sur les 500.

**Tout autre `Kind` → HTTP 500**

```json
{
  "type": "https://www.jhipster.tech/problem/problem-with-message",
  "title": "Internal Server Error",
  "status": 500,
  "detail": "RuntimeException: Demande introuvable",
  "path": "/api/gateway/v1/demandes/traitement",
  "message": "error.http.500"
}
```

**Aucune de ces réponses ne porte de champ `code`.** C'est le point 4 des invariants et la raison
d'être du sandbox : ton backend doit fonctionner sans lui.

**Authentification**

| Situation | Réponse |
|---|---|
| En-tête `Authorization` **absent** | `401` + enveloppe ARTP `ACCES_INTERDIT`, `Content-Type: application/json;charset=UTF-8` |
| Jeton **invalide ou expiré** | `401`, **corps vide**, **sans `Content-Type`** (ANO-008) |
| Identifiants incorrects sur `/api/authenticate` | `401` `problem+json` JHipster (ANO-016) |

Le message de `ACCES_INTERDIT` est, au caractère près :
`Token JWT absent, invalide ou expiré. Veuillez vous authentifier à nouveau.`

**Deux anomalies métier à reproduire spécifiquement**

- **ANO-002** — re-portage à moins de 3 mois : `500` avec
  `"detail": "Unexpected runtime exception"` — et non un `DELAI_PORTAGE_NON_RESPECTE` propre.
- **ANO-020** — restitution à moins de 6 mois : `500` dont le `detail` contient une erreur 400
  **sérialisée en chaîne** portant `error.numeroRestitutionTooEarly`.

### 5.3 Rendu `FIDELITY=contract`

Enveloppe ARTP partout, code du §9 renseigné, HTTP sémantique :

| `Kind` | HTTP |
|---|---|
| `Validation` | 400 `VALIDATION_ECHOUEE` |
| `Introuvable` | 404 `DEMANDE_NON_TROUVEE` |
| `Acces` | 403 `DEMANDE_ACCES_REFUSE` / `ACCES_INTERDIT` |
| `Etat` | 409 `ETAPE_INVALIDE`, `MOTIF_REJET_OBLIGATOIRE`, … |
| `Interne` | 500 `ERREUR_INTERNE` |

Les 23 codes du §9 sont tous implémentés et atteignables dans ce mode. Basculer un jeu de tests
de `real` à `contract` sans le modifier est le test de la couche d'adaptation R-12.

### 5.4 Enveloppe de succès — identique dans les deux modes

```json
{ "success": true, "code": "SUCCESS", "message": "…", "data": {} }
```

Les `message` sont ceux du guide, mot pour mot :
`Opérateurs récupérés avec succès`, `Demande créée avec succès`, `Demande flotte créée`,
`Étape traitée avec succès`, `Demande annulée avec succès`, `Incident déclaré avec succès`,
`Demande de reverse soumise avec succès`, `Types d'incident récupérés avec succès`,
`Demandes à confirmer récupérées avec succès`, `Demandes IN récupérées avec succès`.

**ANO-011** : la réponse de `POST /otp/send` **omet le champ `data`** en mode `real` — il n'est ni
présent ni `null`. En mode `contract`, `data: null`.

## 6. Le moteur

Un unique goroutine `engine.Run(ctx)` réveillée toutes les `ENGINE_TICK_SECONDS` (défaut `10`).

### 6.1 Expiration des étapes — ANO-006

Toute étape dont `date_debut > ETAPE_TIMEOUT_SECONDS` avance seule :
`statut_etape_actuel = EXPIRE`, passage à l'étape suivante, sans qu'aucun opérateur n'ait appelé.
L'expiration de `COMPLETION` clôt la demande : `statutDemande = TERMINE`,
`statutEtapeActuel = EXPIRE`, `dateFinalisation` renseignée.

Une expiration **effectue réellement la transition du numéro** au registre national : c'est le
constat central du SIT — la plateforme déclare un numéro porté sans qu'aucun HLR n'ait été touché.

`ETAPE_TIMEOUT_SECONDS=0` désactive complètement l'expiration.

### 6.2 Convergence différée — R-10

`POST /demandes/traitement` et `POST /demandes/a-confirmer` :

1. valident l'habilitation et l'état, écrivent l'action en base ;
2. **répondent immédiatement `success: true` en portant l'étape *précédant* la transition** ;
3. la transition réelle est appliquée par le moteur après un délai tiré uniformément dans
   `[CONVERGENCE_MIN_SECONDS, CONVERGENCE_MAX_SECONDS]` (défaut `60`–`360`).

Mettre les deux à `0` rend le sandbox déterministe : la transition est appliquée dans la
transaction, mais **la réponse continue de porter l'état périmé** — ce comportement-là n'est pas
une temporisation, c'est le contrat de réponse mesuré.

### 6.3 Latence de `COMPLETION` — ANO-005

`COMPLETION` dort `COMPLETION_LATENCY_MS` (défaut `30500`) avant de répondre. Aucun en-tête
`Idempotency-Key` n'est lu ; un rejeu tombe sur une erreur d'état.

### 6.4 Dérive d'horloge — ANO-015

Tous les horodatages **renvoyés par l'API** sont décalés de `+CLOCK_SKEW_SECONDS` (défaut `540`,
soit 9 minutes). Les horodatages **en base** restent justes : la dérive est un artefact de
présentation, comme observé.

### 6.5 Gel de la place — BR-012

Tant qu'un incident **interne** est `EN_COURS`, chez n'importe quel opérateur :
`/demandes/acceptation`, `/demandes/{id}/acceptation`, `/demandes/a-confirmer`,
`/demandes/traitement` sont refusés, et le moteur suspend l'expiration.
Le comportement de refus n'est pas documenté — **[HYP]** : `Kind = Etat`, code
`ERREUR_INTERNE`, message
`Le traitement des demandes est gelé par un incident interne en cours.`

### 6.6 Actes de l'ARTP

Hors API par construction (D-4).

| Acte | Déclencheur |
|---|---|
| Validation / rejet d'un `reverse-request` | `artp reverse valider <id>` \| `artp reverse rejeter <id>`, ou automatiquement après `REVERSE_AUTO_VALIDATION_SECONDS` (défaut `0` = jamais) |
| Création de la `Demande` REVERSE à l'étape `CONFIRMATION` | Conséquence automatique d'une validation |
| `COMPLETION` d'un REVERSE | Moteur, dès que tous les opérateurs ont confirmé |

`POST /demandes/traitement` sur un REVERSE à `COMPLETION` renvoie toujours, au caractère près :

```json
{ "success": false, "code": "DEMANDE_ACCES_REFUSE",
  "message": "La complétion (COMPLETION) d'une demande REVERSE est réservée à l'ARTP, une fois que tous les opérateurs ont confirmé." }
```

(rendu selon le mode de fidélité — donc en `500 problem+json` sans `code` en mode `real`).

## 7. Modèle de données

PostgreSQL. Clés primaires `TEXT` portant un ObjectId hex de 24 caractères.

| Table | Rôle |
|---|---|
| `operateur` | `id`, `nom`, `prefixe_routage` |
| `utilisateur` | `id`, `username`, `password_hash` (bcrypt), `operateur_id`, `roles[]` |
| `motif_rejet` | `id`, `motif` — champ `motif`, **pas** `libelle` (ANO-009) |
| `type_demande` | `id`, `type` |
| `processus` | `id`, `type` |
| `type_incident` | `id`, `libelle`, `fige_systeme` |
| `numero` | **le registre national** : `msisdn` PK, `operateur_actuel_id`, `operateur_origine_id`, `date_dernier_portage`, `deja_restitue`, `actif` |
| `demande` | état complet + `createur_operateur_id`, `routage_info`, `date_debut_etape`, `transition_prevue_a` |
| `demande_numero` | un enregistrement par numéro d'une flotte : `statut`, `motif_rejet_id`, `exclu`, `raison_exclusion`, `code_erreur_exclusion` |
| `demande_client` | identité, particulier comme entreprise |
| `etape_historique` | une ligne par étape franchie : `etape`, `statut`, `operateur_id`, `commentaire`, `origine` (`ACTION` \| `EXPIRATION`) |
| `confirmation` | `(demande_id, operateur_id)` unique — anti-rejeu TC-041 |
| `otp` | `numero`, `code`, `expire_a`, `tentatives`, `consomme` |
| `reverse_request` | `id`, `numero`, `operateur_id`, `statut`, `date_demande`, `date_decision` |
| `incident` | `id`, `operateur_id`, `type_incident_id`, `description`, `statut`, dates, `commentaire_resolution` |

Contrainte d'unicité partielle sur `incident` : un seul incident interne `EN_COURS` par opérateur.

`numero` est ce qui rend les règles d'éligibilité réelles plutôt que simulées : sans registre, ni
`DELAI_PORTAGE_NON_RESPECTE`, ni `NUMERO_NON_PORTE`, ni `OPERATEUR_SOURCE_INCORRECT` ne sont
calculables.

## 8. Machine à états

```
ACCEPTATION → DESACTIVATION → ACTIVATION → CONFIRMATION → COMPLETION → (terminal)
```

| Étape | Endpoint | Habilitation |
|---|---|---|
| `ACCEPTATION` | `POST /demandes/acceptation` (particulier, restitution) · `POST /demandes/{id}/acceptation` (flotte) | Source |
| `DESACTIVATION` | `POST /demandes/traitement` | Source |
| `ACTIVATION` | `POST /demandes/traitement` | Destinataire |
| `CONFIRMATION` | `POST /demandes/a-confirmer` | PORTAGE : tous sauf destinataire · RESTITUTION/REVERSE : tous |
| `COMPLETION` | `POST /demandes/traitement` | Destinataire — sauf REVERSE (ARTP) |

Une REVERSE naît directement à `CONFIRMATION`.

**`statutDemande`** : `EN_COURS`, `TERMINE`, `ANNULE`, `REJETE` **[HYP — `REJETE` n'est ni
documenté ni mesuré]**.

**`statutEtapeActuel`** : `EN_COURS`, `TERMINE` (soldée par action), `EXPIRE` (soldée par
expiration), `VALIDE` (`COMPLETION` nominale, d'après l'exemple `/demandes/in` du §7.7).
ANO-013 relève que ces états ne figurent pas au cycle de vie documenté ; le sandbox les émet tels
que mesurés.

**`routageInfo`** — PORTAGE : préfixe de la source dès la création, recalculé numéro par numéro au
passage à `CONFIRMATION` (préfixe destinataire pour les numéros portés, source pour les rejetés).
RESTITUTION / REVERSE : absent jusqu'à `COMPLETION`.

**`etape` sur `/traitement`** (ANO-018) : accepté, **ignoré en silence**, l'étape courante est
exécutée. Ni rejet, ni avertissement, dans les deux modes de fidélité.

## 9. Règles métier par endpoint

### 9.1 OTP

- `POST /otp/send` : enregistre l'OTP statique pour le numéro, `expire_a = now + 5 min`,
  `tentatives = 0`. Réponse sans champ `data` en mode `real`.
- `POST /otp/verify` : **pré-vérifie sans consommer** (TC-021) — le code reste utilisable pour
  créer la demande. Incrémente `tentatives` en cas d'échec.
- Erreurs : `OTP_INVALID`, `OTP_EXPIRED`, `OTP_ALREADY_USED`, `OTP_MAX_ATTEMPTS`. En mode `real`
  elles sortent en `500` avec les messages libres mesurés (ANO-014) :
  `Aucun OTP actif pour ce numéro`, `Le code OTP a expiré`.

### 9.2 Création particulier

Contrôles, dans l'ordre. **Cet ordre est une décision de conception, non une mesure** — ni le
guide ni la recette ne le fixent :

1. Validation de forme → `400 fieldErrors`. `client.lieuNaissance` est **obligatoire** malgré le
   guide (ANO-010, TC-050).
2. `operateurDestinataireId` == opérateur du jeton, sinon `DEMANDE_ACCES_REFUSE`. **Avant l'OTP** :
   sans cela, un opérateur pourrait sonder la validité d'un OTP portant sur le numéro d'un tiers
   et en consommer les tentatives.
3. OTP valide et non consommé.
4. Le numéro n'est pas déjà chez le destinataire → `NUMERO_DEJA_CHEZ_DESTINATAIRE`. **Avant le
   contrôle de source** : les deux conditions sont vraies simultanément dans ce cas, et celle-ci
   est le diagnostic le plus spécifique.
5. Le numéro appartient à `operateurSourceId`, sinon `OPERATEUR_SOURCE_INCORRECT`.
6. Aucune demande en cours pour ce numéro → `DEMANDE_EN_COURS_POUR_NUMERO`.
7. Dernier portage > 3 mois → sinon **ANO-002** (`500 Unexpected runtime exception`).

Consomme l'OTP, crée la demande à `ACCEPTATION`/`EN_COURS`, renseigne `routageInfo` au préfixe
source. `201`.

### 9.3 Création entreprise (flotte)

- `numerosFlotte` vide → `FLOTTE_VIDE`.
- Numéros chez des opérateurs différents → `FLOTTE_OPERATEURS_MIXTES`.
- Chaque numéro subit les contrôles 4→7 ci-dessus ; un échec **exclut le numéro** au lieu de faire
  échouer la demande, et alimente `numerosExclus[]` avec `raison` et `codeErreur`.
- Tous exclus → `AUCUN_NUMERO_ELIGIBLE`, **rien n'est créé**.
- Sinon `201` avec `numerosPortesCount`, `numerosExclusCount`, `numerosExclus[]`, `avertissement`
  — champs obligatoires jusqu'à l'agent YAS (BR-006, invariant 11).
- Un seul OTP, sur `numeroPorteurFlotte`, couvre toute la flotte.

### 9.4 Restitution

Conditions : numéro porté (`NUMERO_NON_PORTE`), jamais restitué (`NUMERO_DEJA_RESTITUE`), 6 mois
écoulés — sinon **ANO-020**. L'appelant doit être l'opérateur d'origine du numéro et en devient le
destinataire ; le détenteur actuel est la source. **[HYP — la répartition des rôles n'est pas
explicitée par le guide.]**

### 9.5 Acceptation

- Particulier / restitution : `idDemande` (et non `numero` — rupture v2), `accepte`,
  `motifRejetId` obligatoire si `accepte = false` (`MOTIF_REJET_OBLIGATOIRE`, TC-044).
- Flotte : `numerosRejetes[]` permet un rejet **partiel** ; les numéros rejetés restent chez la
  source et prennent le routage source à `CONFIRMATION`.
- Le destinataire qui tente d'accepter sa propre demande est refusé (TC-034).

### 9.6 Confirmation

- Ensemble attendu, PORTAGE : **tous les opérateurs de la place sauf le destinataire** (D-6).
  RESTITUTION / REVERSE : tous, destinataire compris.
- Anti-rejeu : seconde confirmation par le même opérateur refusée (TC-041).
- Tant qu'il manque une confirmation : `etapeActuelle: CONFIRMATION`,
  `statutEtapeActuel: EN_COURS`.
- `GET /demandes/a-confirmer/{id}` par le destinataire d'un PORTAGE → erreur (mesuré : 500).
- **ANO-019** reproduite en mode `real` : `deja-confirmees` **ne trace pas** la confirmation de
  l'opérateur *source* — elle ne remonte que pour les tiers. Corrigée en mode `contract`.

### 9.7 Traitement

Étape déduite de l'état. `ACCEPTATION` et `CONFIRMATION` renvoient `ETAPE_INVALIDE` avec le
message exact du guide :
`L'étape CONFIRMATION se traite via POST /api/gateway/v1/demandes/a-confirmer.`
Une étape traitée par le mauvais opérateur, ou hors séquence, est refusée (TC-036).

### 9.8 Annulation

Créateur (donc destinataire) uniquement, et seulement à `ACCEPTATION`/`EN_COURS`. Messages exacts :

- `Cette demande ne peut plus être annulée (étape actuelle : DESACTIVATION).`
- `Seul l'opérateur ayant créé la demande (opérateur destinataire) peut l'annuler.`

### 9.9 Incidents

Catégorie déterminée par le segment d'URL, jamais par un `typeIncidentId` dans le corps. Seul le
déclarant résout. Mauvais segment → `VALIDATION_ECHOUEE` désignant le bon endpoint. Un seul
incident interne ouvert par opérateur.

## 10. Seed

### Opérateurs — identifiants de recette, exacts

| id | nom | préfixe |
|---|---|---|
| `6a21745ce6c37b5b5b487ec1` | ORANGE | 191 |
| `6a2174c3e6c37b5b5b487ec4` | YAS | 192 |
| `6a217510e6c37b5b5b487ec7` | EXPRESSO | 193 **[HYP — non mesuré]** |

### Motifs de rejet

| id | motif |
|---|---|
| `6a2175c5e6c37b5b5b487edb` | Dernier portage inférieur à 3 mois |
| `6a2175cfe6c37b5b5b487edc` | Erreur sur les infos |
| `6a2175d9e6c37b5b5b487edd` | Données manquantes |
| `6a2175e7e6c37b5b5b487ede` | Numéro Inactif |
| `6a2175f3e6c37b5b5b487edf` | Identité non prouvée |
| `6a2175fde6c37b5b5b487ee0` | Engagement en cours dans une demande |

### Types de demande et processus

`6a217518e6c37b5b5b487ec8` PORTAGE · `6a21751be6c37b5b5b487ec9` RESTITUTION ·
`6a21751fe6c37b5b5b487eca` REVERSE
`6a217686e6c37b5b5b487ee8` PREPAID · `6a217689e6c37b5b5b487ee9` POSTPAID

### Types d'incident

`65abc456def001` Gateway (`figeSysteme: false`) · `65abc456def002` Technique
(`figeSysteme: true`) — identifiants du guide, seuls disponibles.

### Comptes

| username | password | opérateur | rôles |
|---|---|---|---|
| `orange` | `orange2026` | ORANGE | `ROLE_OPERATEUR_ADMIN`, `ROLE_USER` |
| `yas` | `yas2026` | YAS | `ROLE_OPERATEUR_ADMIN`, `ROLE_USER` |
| `expresso` | `expresso2026` | EXPRESSO | `ROLE_OPERATEUR_ADMIN`, `ROLE_USER` |

### Vivier de numéros

Réparti pour rendre chaque règle exerçable dès le premier démarrage :

| Tranche | Situation | Rend testable |
|---|---|---|
| `77100xxxx` | ORANGE, jamais porté | Portage nominal ORANGE → YAS |
| `76100xxxx` | YAS, jamais porté | Portage sortant YAS → ORANGE |
| `70100xxxx` | EXPRESSO, jamais porté | Portage impliquant un tiers |
| `77200xxxx` | ORANGE, porté il y a 30 jours | `DELAI_PORTAGE_NON_RESPECTE` / ANO-002 |
| `77300xxxx` | YAS, porté il y a 8 mois depuis ORANGE | Restitution nominale |
| `77400xxxx` | YAS, porté il y a 2 mois depuis ORANGE | `DELAI_RESTITUTION_NON_RESPECTE` / ANO-020 |
| `77500xxxx` | YAS, porté puis déjà restitué | `NUMERO_DEJA_RESTITUE` |

## 11. Configuration

| Variable | Défaut | Effet |
|---|---|---|
| `PORT` | `8080` | |
| `DATABASE_URL` | — | |
| `JWT_SECRET` | dev | HS512 |
| `JWT_TTL_HOURS` | `24` | |
| `FIDELITY` | `real` | `real` \| `contract` |
| `ETAPE_TIMEOUT_SECONDS` | `349` | `0` = pas d'expiration |
| `ENGINE_TICK_SECONDS` | `10` | |
| `CONVERGENCE_MIN_SECONDS` | `60` | |
| `CONVERGENCE_MAX_SECONDS` | `360` | |
| `COMPLETION_LATENCY_MS` | `30500` | |
| `CLOCK_SKEW_SECONDS` | `540` | |
| `OTP_STATIC_CODE` | `123456` | |
| `OTP_TTL_SECONDS` | `300` | |
| `OTP_MAX_ATTEMPTS` | `3` | |
| `REVERSE_AUTO_VALIDATION_SECONDS` | `0` | `0` = validation par CLI uniquement |

**Profil CI** — `ETAPE_TIMEOUT_SECONDS=0`, `CONVERGENCE_*=0`, `COMPLETION_LATENCY_MS=0`,
`CLOCK_SKEW_SECONDS=0` : sandbox déterministe, un cycle complet en moins d'une seconde.

## 12. Tests

**Domaine** — tests en table, sans base : séquence des étapes, habilitations, ensemble des
confirmations attendues, exclusions de flotte, calcul de `routageInfo`, éligibilité.

**Intégration** — Postgres réel, serveur `httptest`, assertions sur **le code HTTP et le corps
exacts**, jamais sur une abstraction. Les cas nommés du SIT sont rejoués :

| Cas | Attendu en mode `real` |
|---|---|
| TC-021 | `verify` ne consomme pas ; la création qui suit réussit |
| TC-034 | Destinataire acceptant sa propre demande → **500** |
| TC-036 | `ACTIVATION` avant `DESACTIVATION` → **500** |
| TC-041 | Seconde confirmation du même opérateur → **500** |
| TC-044 | Rejet sans `motifRejetId` → **500** |
| TC-050 | `lieuNaissance` absent → **400** avec `fieldErrors` |
| TC-062 | Aucun appel, `ETAPE_TIMEOUT_SECONDS=2` → les 5 étapes franchies seules |
| ANO-001 | Sur 20 erreurs provoquées, **aucune** ne porte de champ `code` |
| ANO-018 | `{"idDemande":…,"etape":"CONFIRMATION"}` sur une demande à `DESACTIVATION` → `DESACTIVATION` exécutée, 200 |

**Bout en bout** — le scénario du §10 du guide, ORANGE → YAS avec confirmation d'EXPRESSO,
jusqu'à `TERMINE` et apparition dans `/demandes/in` et `/demandes/out`.

**Bascule de fidélité** — le même jeu de bout en bout rejoué en `FIDELITY=contract` : les codes
HTTP et l'enveloppe changent, la machine à états ne change pas.

## 13. Livraison

`docker compose up` démarre Postgres, applique les migrations, exécute le seed, expose l'API sur
`http://localhost:8080`. Une collection Postman calquée sur celle de l'ARTP est fournie dans
`postman/`, avec un environnement `baseUrl = http://localhost:8080` — la bascule depuis la recette
ARTP ne demande que de changer cette variable.

## 14. Hors périmètre

- Envoi réel de SMS — le code OTP est statique (D-5).
- Notification de l'abonné (ANO-022) : ce manque est **reproduit**, pas corrigé.
- Interface d'administration, back-office, UI.
- Toute route HTTP absente du §4.
