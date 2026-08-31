# NumFlex Sandbox — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reimplémenter localement l'API Gateway NumFlex de l'ARTP, fidèle au guide v2 **et** au comportement réellement mesuré en recette, pour servir de cible de développement au backend YAS.

**Architecture:** Un binaire serveur Gin exposant exactement les 33 routes du contrat et rien d'autre, une couche d'erreur unique rendue selon deux modes de fidélité commutables (`real` reproduit les anomalies mesurées, `contract` respecte le guide), et un moteur en goroutine qui fait avancer les demandes sans qu'aucun opérateur n'agisse. PostgreSQL porte un registre national des numéros qui rend les règles d'éligibilité réellement calculables.

**Tech Stack:** Go 1.24+, Gin, pgx/v5, golang-migrate, golang-jwt/v5, bcrypt, testify, PostgreSQL 16, Docker Compose.

**Spec:** `docs/superpowers/specs/2026-08-29-numflex-sandbox-design.md`

## Global Constraints

- **Module Go** : `github.com/yas/numflex-sandbox`. La directive `go` de `go.mod` suit ce
  qu'exigent les dépendances courantes — elle vaut `1.24` après la Task 1 et monte à `1.25`
  dès qu'une tâche importe gin, pgx ou x/crypto. Ne jamais épingler une dépendance à une
  version ancienne dans le seul but de tenir un plancher plus bas.
- **Aucune route HTTP hors des 33 du §4 de la spec.** Pas d'endpoint d'administration, de santé, de metrics, de debug. Ajouter une route hors liste est un échec de tâche.
- **`internal/domain` ne consulte jamais le mode de fidélité** et n'importe ni `gin`, ni `pgx`, ni `httpx`. Le mode de fidélité n'existe que dans `internal/httpx`.
- **Toutes les erreurs renvoyées par `domain` et `store` sont des `*apperr.Error`.** Aucun `errors.New` nu ne remonte jusqu'au handler.
- **Les messages français sont copiés au caractère près** depuis la spec (accents, ponctuation, majuscules compris). Un message approximatif est un échec de tâche.
- **Les identifiants générés sont des ObjectId hex de 24 caractères minuscules.** Jamais d'UUID, jamais de `dem-abc123`.
- **Défauts de configuration** : `FIDELITY=real`, `ETAPE_TIMEOUT_SECONDS=349`, `ENGINE_TICK_SECONDS=10`, `CONVERGENCE_MIN_SECONDS=60`, `CONVERGENCE_MAX_SECONDS=360`, `COMPLETION_LATENCY_MS=30500`, `CLOCK_SKEW_SECONDS=540`, `OTP_STATIC_CODE=123456`, `OTP_TTL_SECONDS=300`, `OTP_MAX_ATTEMPTS=3`, `REVERSE_AUTO_VALIDATION_SECONDS=0`, `JWT_TTL_HOURS=24`.
- **Les tests d'intégration tournent avec le profil CI** : `ETAPE_TIMEOUT_SECONDS=0`, `CONVERGENCE_MIN_SECONDS=0`, `CONVERGENCE_MAX_SECONDS=0`, `COMPLETION_LATENCY_MS=0`, `CLOCK_SKEW_SECONDS=0`, sauf mention contraire explicite dans la tâche.
- **Base de test** : `postgres://numflex:numflex@localhost:5433/numflex_test?sslmode=disable`, servie par le service `postgres-test` du `docker-compose.yml`. Chaque test d'intégration tronque toutes les tables métier puis rejoue le seed.
- **Commits** : un par tâche minimum, message en français **accentué**, préfixe conventionnel
  (`feat:`, `test:`, `chore:`). Rédiger le message via `git commit -F -` et un heredoc plutôt
  que `git commit -m`, pour que les accents et apostrophes survivent au shell.

---

## Structure des fichiers

| Fichier | Responsabilité | Tâche |
|---|---|---|
| `go.mod`, `docker-compose.yml`, `Makefile`, `.env.example` | Socle projet | T1 |
| `internal/config/config.go` | Lecture d'environnement, défauts, validation | T1 |
| `internal/oid/oid.go` | Génération d'ObjectId hex 24 | T1 |
| `migrations/0001_init.up.sql` / `.down.sql` | Schéma complet | T2 |
| `internal/store/db.go` | Pool pgx, application des migrations | T2 |
| `internal/store/testdb.go` | Helper de test : base propre + seed | T2 |
| `internal/seed/seed.go` | Référentiels, comptes, vivier de numéros | T3 |
| `internal/apperr/apperr.go` | Type `Error`, `Kind`, `FieldError` | T4 |
| `internal/apperr/catalogue.go` | Les 23 codes du §9 avec leurs messages exacts | T4 |
| `internal/httpx/envelope.go` | Enveloppe ARTP de succès | T4 |
| `internal/httpx/renderer.go` | Les deux rendus d'erreur | T4 |
| `internal/auth/jwt.go` | Émission et vérification HS512 | T5 |
| `internal/auth/middleware.go` | `Authorization: Bearer`, résolution opérateur | T5 |
| `internal/api/auth.go` | `POST` / `GET /api/authenticate` | T5 |
| `internal/api/referentiels.go` | Les 5 GET de référence | T6 |
| `internal/api/otp.go` | `otp/send`, `otp/verify` | T7 |
| `internal/domain/etapes.go` | Machine à états, responsables, confirmateurs | T8 |
| `internal/domain/demande.go` | Structures métier partagées | T8 |
| `internal/engine/engine.go` | Ticker : expiration, convergence, actes ARTP | T9 |
| `internal/domain/eligibilite.go` | Contrôles d'éligibilité sur le registre | T10 |
| `internal/api/demandes_creation.go` | `particulier`, `entreprise`, `restitution` | T10, T11, T12 |
| `internal/api/demandes_lecture.go` | Les 10 GET de consultation | T13 |
| `internal/api/acceptation.go` | Les 2 endpoints d'acceptation | T14 |
| `internal/api/confirmation.go` | `POST /demandes/a-confirmer` | T15 |
| `internal/api/traitement.go` | `POST /demandes/traitement` | T16 |
| `internal/api/annulation.go` | `POST /demandes/{id}/annuler` | T17 |
| `internal/api/incidents.go` | Les 6 routes d'incident + gel | T18 |
| `internal/api/reverse.go` | Les 2 routes reverse-requests | T19 |
| `cmd/artp/main.go` | CLI régulateur | T19 |
| `internal/api/router.go` | Déclaration des 33 routes, et d'elles seules | T5 puis étendu |
| `cmd/server/main.go` | Câblage | T1 puis étendu |
| `test/e2e_test.go` | Scénario complet, deux modes | T20 |
| `postman/` | Collection + environnement | T20 |

---

## Task 1 : Socle projet, configuration, ObjectId

**Files:**
- Create: `go.mod`, `docker-compose.yml`, `Makefile`, `.env.example`, `cmd/server/main.go`
- Create: `internal/config/config.go`, `internal/config/config_test.go`
- Create: `internal/oid/oid.go`, `internal/oid/oid_test.go`

**Interfaces:**
- Consumes: rien.
- Produces: `config.Load() (*config.Config, error)` ; `config.Config` avec les champs `Port string`, `DatabaseURL string`, `JWTSecret string`, `JWTTTL time.Duration`, `Fidelity config.Fidelity`, `EtapeTimeout time.Duration`, `EngineTick time.Duration`, `ConvergenceMin time.Duration`, `ConvergenceMax time.Duration`, `CompletionLatency time.Duration`, `ClockSkew time.Duration`, `OTPStaticCode string`, `OTPTTL time.Duration`, `OTPMaxAttempts int`, `ReverseAutoValidation time.Duration`. Constantes `config.FidelityReal` et `config.FidelityContract`. `oid.New() string`.

- [ ] **Step 1 : Initialiser le module et les dépendances**

```bash
go mod init github.com/yas/numflex-sandbox
go get github.com/gin-gonic/gin@latest
go get github.com/jackc/pgx/v5@latest
go get github.com/golang-jwt/jwt/v5@latest
go get github.com/golang-migrate/migrate/v4@latest
go get github.com/stretchr/testify@latest
go get golang.org/x/crypto/bcrypt
```

- [ ] **Step 2 : Écrire le test de configuration**

Create `internal/config/config_test.go` :

```go
package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadDefauts(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")

	c, err := Load()
	require.NoError(t, err)

	require.Equal(t, "8080", c.Port)
	require.Equal(t, FidelityReal, c.Fidelity)
	require.Equal(t, 349*time.Second, c.EtapeTimeout)
	require.Equal(t, 10*time.Second, c.EngineTick)
	require.Equal(t, 60*time.Second, c.ConvergenceMin)
	require.Equal(t, 360*time.Second, c.ConvergenceMax)
	require.Equal(t, 30500*time.Millisecond, c.CompletionLatency)
	require.Equal(t, 540*time.Second, c.ClockSkew)
	require.Equal(t, "123456", c.OTPStaticCode)
	require.Equal(t, 300*time.Second, c.OTPTTL)
	require.Equal(t, 3, c.OTPMaxAttempts)
	require.Equal(t, 24*time.Hour, c.JWTTTL)
	require.Equal(t, time.Duration(0), c.ReverseAutoValidation)
}

func TestLoadProfilCI(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("ETAPE_TIMEOUT_SECONDS", "0")
	t.Setenv("CONVERGENCE_MIN_SECONDS", "0")
	t.Setenv("CONVERGENCE_MAX_SECONDS", "0")
	t.Setenv("COMPLETION_LATENCY_MS", "0")
	t.Setenv("CLOCK_SKEW_SECONDS", "0")

	c, err := Load()
	require.NoError(t, err)
	require.Equal(t, time.Duration(0), c.EtapeTimeout)
	require.Equal(t, time.Duration(0), c.CompletionLatency)
	require.Equal(t, time.Duration(0), c.ClockSkew)
}

func TestLoadFideliteInvalide(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("FIDELITY", "presque")

	_, err := Load()
	require.Error(t, err)
}

func TestLoadDatabaseURLObligatoire(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	_, err := Load()
	require.Error(t, err)
}

func TestLoadConvergenceIncoherente(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("CONVERGENCE_MIN_SECONDS", "300")
	t.Setenv("CONVERGENCE_MAX_SECONDS", "60")

	_, err := Load()
	require.Error(t, err)
}
```

- [ ] **Step 3 : Lancer le test, vérifier qu'il échoue**

Run: `go test ./internal/config/...`
Expected: FAIL — `undefined: Load`.

- [ ] **Step 4 : Implémenter la configuration**

Create `internal/config/config.go` :

```go
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Fidelity string

const (
	FidelityReal     Fidelity = "real"
	FidelityContract Fidelity = "contract"
)

type Config struct {
	Port                  string
	DatabaseURL           string
	JWTSecret             string
	JWTTTL                time.Duration
	Fidelity              Fidelity
	EtapeTimeout          time.Duration
	EngineTick            time.Duration
	ConvergenceMin        time.Duration
	ConvergenceMax        time.Duration
	CompletionLatency     time.Duration
	ClockSkew             time.Duration
	OTPStaticCode         string
	OTPTTL                time.Duration
	OTPMaxAttempts        int
	ReverseAutoValidation time.Duration
}

func Load() (*Config, error) {
	c := &Config{
		Port:          str("PORT", "8080"),
		DatabaseURL:   str("DATABASE_URL", ""),
		JWTSecret:     str("JWT_SECRET", "numflex-sandbox-dev-secret"),
		Fidelity:      Fidelity(str("FIDELITY", string(FidelityReal))),
		OTPStaticCode: str("OTP_STATIC_CODE", "123456"),
	}

	var err error
	if c.JWTTTL, err = dur("JWT_TTL_HOURS", 24, time.Hour); err != nil {
		return nil, err
	}
	if c.EtapeTimeout, err = dur("ETAPE_TIMEOUT_SECONDS", 349, time.Second); err != nil {
		return nil, err
	}
	if c.EngineTick, err = dur("ENGINE_TICK_SECONDS", 10, time.Second); err != nil {
		return nil, err
	}
	if c.ConvergenceMin, err = dur("CONVERGENCE_MIN_SECONDS", 60, time.Second); err != nil {
		return nil, err
	}
	if c.ConvergenceMax, err = dur("CONVERGENCE_MAX_SECONDS", 360, time.Second); err != nil {
		return nil, err
	}
	if c.CompletionLatency, err = dur("COMPLETION_LATENCY_MS", 30500, time.Millisecond); err != nil {
		return nil, err
	}
	if c.ClockSkew, err = dur("CLOCK_SKEW_SECONDS", 540, time.Second); err != nil {
		return nil, err
	}
	if c.OTPTTL, err = dur("OTP_TTL_SECONDS", 300, time.Second); err != nil {
		return nil, err
	}
	if c.ReverseAutoValidation, err = dur("REVERSE_AUTO_VALIDATION_SECONDS", 0, time.Second); err != nil {
		return nil, err
	}
	if c.OTPMaxAttempts, err = num("OTP_MAX_ATTEMPTS", 3); err != nil {
		return nil, err
	}

	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL est obligatoire")
	}
	if c.Fidelity != FidelityReal && c.Fidelity != FidelityContract {
		return nil, fmt.Errorf("FIDELITY doit valoir %q ou %q, reçu %q",
			FidelityReal, FidelityContract, c.Fidelity)
	}
	if c.ConvergenceMax < c.ConvergenceMin {
		return nil, fmt.Errorf("CONVERGENCE_MAX_SECONDS ne peut être inférieur à CONVERGENCE_MIN_SECONDS")
	}
	if c.EngineTick <= 0 {
		return nil, fmt.Errorf("ENGINE_TICK_SECONDS doit être strictement positif")
	}
	return c, nil
}

func str(clef, defaut string) string {
	if v, ok := os.LookupEnv(clef); ok && v != "" {
		return v
	}
	return defaut
}

func num(clef string, defaut int) (int, error) {
	v, ok := os.LookupEnv(clef)
	if !ok || v == "" {
		return defaut, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s : entier attendu, reçu %q", clef, v)
	}
	return n, nil
}

func dur(clef string, defaut int, unite time.Duration) (time.Duration, error) {
	n, err := num(clef, defaut)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, fmt.Errorf("%s ne peut être négatif", clef)
	}
	return time.Duration(n) * unite, nil
}
```

- [ ] **Step 5 : Lancer le test, vérifier qu'il passe**

Run: `go test ./internal/config/...`
Expected: PASS.

- [ ] **Step 6 : Écrire le test du générateur d'ObjectId**

Create `internal/oid/oid_test.go` :

```go
package oid

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

var motif = regexp.MustCompile(`^[0-9a-f]{24}$`)

func TestNewFormat(t *testing.T) {
	for i := 0; i < 100; i++ {
		id := New()
		require.Truef(t, motif.MatchString(id), "identifiant non conforme : %q", id)
	}
}

func TestNewUnicite(t *testing.T) {
	vus := make(map[string]bool, 10000)
	for i := 0; i < 10000; i++ {
		id := New()
		require.Falsef(t, vus[id], "collision sur %q", id)
		vus[id] = true
	}
}
```

- [ ] **Step 7 : Lancer le test, vérifier qu'il échoue**

Run: `go test ./internal/oid/...`
Expected: FAIL — `undefined: New`.

- [ ] **Step 8 : Implémenter le générateur**

Create `internal/oid/oid.go` :

```go
// Package oid produit des identifiants au format ObjectId MongoDB — 24 caractères
// hexadécimaux. La plateforme NumFlex de recette renvoie ce format ; le guide v2
// n'affiche que des exemples illustratifs du type "dem-abc123", jamais rencontrés.
package oid

import (
	crand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"sync/atomic"
	"time"
)

var (
	processus [5]byte
	compteur  uint32
)

func init() {
	if _, err := crand.Read(processus[:]); err != nil {
		panic("oid : source d'aléa indisponible : " + err.Error())
	}
	var amorce [4]byte
	if _, err := crand.Read(amorce[:]); err != nil {
		panic("oid : source d'aléa indisponible : " + err.Error())
	}
	compteur = binary.BigEndian.Uint32(amorce[:])
}

// New retourne un ObjectId : 4 octets d'horodatage, 5 octets propres au processus,
// 3 octets de compteur incrémental.
func New() string {
	var b [12]byte
	binary.BigEndian.PutUint32(b[0:4], uint32(time.Now().Unix()))
	copy(b[4:9], processus[:])
	c := atomic.AddUint32(&compteur, 1)
	b[9] = byte(c >> 16)
	b[10] = byte(c >> 8)
	b[11] = byte(c)
	return hex.EncodeToString(b[:])
}
```

- [ ] **Step 9 : Lancer le test, vérifier qu'il passe**

Run: `go test ./internal/oid/...`
Expected: PASS.

- [ ] **Step 10 : Écrire le socle d'exécution**

Create `docker-compose.yml` :

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: numflex
      POSTGRES_PASSWORD: numflex
      POSTGRES_DB: numflex
    ports: ["5432:5432"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U numflex"]
      interval: 2s
      timeout: 3s
      retries: 20

  postgres-test:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: numflex
      POSTGRES_PASSWORD: numflex
      POSTGRES_DB: numflex_test
    ports: ["5433:5432"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U numflex"]
      interval: 2s
      timeout: 3s
      retries: 20

  api:
    build: .
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DATABASE_URL: postgres://numflex:numflex@postgres:5432/numflex?sslmode=disable
      FIDELITY: real
    ports: ["8080:8080"]
```

Create `Dockerfile` :

```dockerfile
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/server ./cmd/server && go build -o /out/artp ./cmd/artp

FROM alpine:3.20
COPY --from=build /out/server /usr/local/bin/server
COPY --from=build /out/artp /usr/local/bin/artp
COPY migrations /migrations
EXPOSE 8080
CMD ["server"]
```

Create `Makefile` :

```makefile
DB_TEST := postgres://numflex:numflex@localhost:5433/numflex_test?sslmode=disable

up:
	docker compose up -d postgres postgres-test

test: up
	DATABASE_URL="$(DB_TEST)" \
	ETAPE_TIMEOUT_SECONDS=0 CONVERGENCE_MIN_SECONDS=0 CONVERGENCE_MAX_SECONDS=0 \
	COMPLETION_LATENCY_MS=0 CLOCK_SKEW_SECONDS=0 \
	go test ./... -count=1

run: up
	DATABASE_URL="postgres://numflex:numflex@localhost:5432/numflex?sslmode=disable" \
	go run ./cmd/server

.PHONY: up test run
```

Create `.env.example` reprenant toutes les variables du §11 de la spec avec leurs défauts.

Create `cmd/server/main.go` — pour l'instant, charge la configuration et journalise :

```go
package main

import (
	"log"

	"github.com/yas/numflex-sandbox/internal/config"
)

func main() {
	c, err := config.Load()
	if err != nil {
		log.Fatalf("configuration : %v", err)
	}
	log.Printf("numflex-sandbox — fidélité=%s expiration=%s port=%s",
		c.Fidelity, c.EtapeTimeout, c.Port)
}
```

- [ ] **Step 11 : Vérifier que tout compile et que la base démarre**

Run: `make up && go build ./... && go test ./...`
Expected: build OK, tests `config` et `oid` PASS.

- [ ] **Step 12 : Commit**

```bash
git add -A
git commit -m "feat: socle projet, configuration et generateur d ObjectId"
```

---

## Task 2 : Schéma PostgreSQL et couche d'accès

**Files:**
- Create: `migrations/0001_init.up.sql`, `migrations/0001_init.down.sql`
- Create: `internal/store/db.go`, `internal/store/testdb.go`
- Create: `internal/store/db_test.go`

**Interfaces:**
- Consumes: `config.Config.DatabaseURL`.
- Produces: `store.Open(ctx context.Context, url string) (*store.DB, error)` ; `store.DB` exposant `Pool *pgxpool.Pool` et `Close()` ; `store.Migrate(url string) error` ; `store.NewTestDB(t *testing.T) *store.DB` qui ouvre la base de test, applique les migrations, tronque toutes les tables métier et rejoue le seed.

- [ ] **Step 1 : Écrire la migration**

Create `migrations/0001_init.up.sql` :

```sql
CREATE TABLE operateur (
    id              TEXT PRIMARY KEY,
    nom             TEXT NOT NULL UNIQUE,
    prefixe_routage TEXT NOT NULL
);

CREATE TABLE utilisateur (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    operateur_id  TEXT NOT NULL REFERENCES operateur(id),
    roles         TEXT[] NOT NULL
);

CREATE TABLE motif_rejet (
    id    TEXT PRIMARY KEY,
    motif TEXT NOT NULL
);

CREATE TABLE type_demande (
    id   TEXT PRIMARY KEY,
    type TEXT NOT NULL UNIQUE
);

CREATE TABLE processus (
    id   TEXT PRIMARY KEY,
    type TEXT NOT NULL UNIQUE
);

CREATE TABLE type_incident (
    id           TEXT PRIMARY KEY,
    libelle      TEXT NOT NULL,
    fige_systeme BOOLEAN NOT NULL
);

-- Registre national des numéros. C'est lui qui rend calculables
-- DELAI_PORTAGE_NON_RESPECTE, NUMERO_NON_PORTE, OPERATEUR_SOURCE_INCORRECT
-- et NUMERO_DEJA_CHEZ_DESTINATAIRE.
CREATE TABLE numero (
    msisdn               TEXT PRIMARY KEY,
    operateur_actuel_id  TEXT NOT NULL REFERENCES operateur(id),
    operateur_origine_id TEXT NOT NULL REFERENCES operateur(id),
    date_dernier_portage TIMESTAMPTZ,
    deja_restitue        BOOLEAN NOT NULL DEFAULT FALSE,
    actif                BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE demande (
    id                        TEXT PRIMARY KEY,
    numero                    TEXT NOT NULL,
    type_abonne               TEXT NOT NULL,
    type_demande              TEXT NOT NULL,
    statut_demande            TEXT NOT NULL,
    etape_actuelle            TEXT NOT NULL,
    statut_etape_actuel       TEXT NOT NULL,
    operateur_source_id       TEXT NOT NULL REFERENCES operateur(id),
    operateur_destinataire_id TEXT NOT NULL REFERENCES operateur(id),
    createur_operateur_id     TEXT NOT NULL REFERENCES operateur(id),
    processus                 TEXT,
    routage_info              TEXT,
    date_demande              TIMESTAMPTZ NOT NULL,
    date_debut_etape          TIMESTAMPTZ NOT NULL,
    transition_prevue_a       TIMESTAMPTZ,
    date_finalisation         TIMESTAMPTZ,
    motif_rejet_id            TEXT REFERENCES motif_rejet(id),
    commentaire               TEXT
);

CREATE INDEX demande_etape_idx ON demande (statut_demande, etape_actuelle);
CREATE INDEX demande_numero_idx ON demande (numero);

CREATE TABLE demande_numero (
    demande_id            TEXT NOT NULL REFERENCES demande(id) ON DELETE CASCADE,
    numero                TEXT NOT NULL,
    statut                TEXT NOT NULL,
    motif_rejet_id        TEXT REFERENCES motif_rejet(id),
    exclu                 BOOLEAN NOT NULL DEFAULT FALSE,
    raison_exclusion      TEXT,
    code_erreur_exclusion TEXT,
    routage_info          TEXT,
    PRIMARY KEY (demande_id, numero)
);

CREATE TABLE demande_client (
    demande_id     TEXT PRIMARY KEY REFERENCES demande(id) ON DELETE CASCADE,
    nom            TEXT,
    prenom         TEXT,
    date_naissance DATE,
    lieu_naissance TEXT,
    type_piece     TEXT,
    numero_piece   TEXT,
    raison_sociale TEXT,
    num_rc         TEXT
);

CREATE TABLE etape_historique (
    id           BIGSERIAL PRIMARY KEY,
    demande_id   TEXT NOT NULL REFERENCES demande(id) ON DELETE CASCADE,
    etape        TEXT NOT NULL,
    statut       TEXT NOT NULL,
    operateur_id TEXT REFERENCES operateur(id),
    origine      TEXT NOT NULL,
    commentaire  TEXT,
    date_debut   TIMESTAMPTZ NOT NULL,
    date_fin     TIMESTAMPTZ
);

CREATE TABLE confirmation (
    demande_id   TEXT NOT NULL REFERENCES demande(id) ON DELETE CASCADE,
    operateur_id TEXT NOT NULL REFERENCES operateur(id),
    commentaire  TEXT,
    date_conf    TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (demande_id, operateur_id)
);

CREATE TABLE otp (
    numero     TEXT PRIMARY KEY,
    code       TEXT NOT NULL,
    expire_a   TIMESTAMPTZ NOT NULL,
    tentatives INT NOT NULL DEFAULT 0,
    consomme   BOOLEAN NOT NULL DEFAULT FALSE,
    cree_le    TIMESTAMPTZ NOT NULL
);

CREATE TABLE reverse_request (
    id            TEXT PRIMARY KEY,
    numero        TEXT NOT NULL,
    operateur_id  TEXT NOT NULL REFERENCES operateur(id),
    statut        TEXT NOT NULL,
    date_demande  TIMESTAMPTZ NOT NULL,
    date_decision TIMESTAMPTZ,
    demande_id    TEXT REFERENCES demande(id)
);

CREATE TABLE incident (
    id                    TEXT PRIMARY KEY,
    operateur_id          TEXT NOT NULL REFERENCES operateur(id),
    type_incident_id      TEXT NOT NULL REFERENCES type_incident(id),
    -- Dénormalisé depuis type_incident : un index partiel ne peut pas suivre
    -- une jointure, et la contrainte du §7.12 ne vise que les incidents internes.
    fige_systeme          BOOLEAN NOT NULL,
    description           TEXT NOT NULL,
    statut                TEXT NOT NULL,
    date_ouverture        TIMESTAMPTZ NOT NULL,
    date_resolution       TIMESTAMPTZ,
    commentaire_resolution TEXT
);

-- Un seul incident INTERNE ouvert à la fois par opérateur — §7.12. Les incidents
-- gateway ne sont pas limités.
CREATE UNIQUE INDEX incident_interne_unique_ouvert
    ON incident (operateur_id)
    WHERE statut = 'EN_COURS' AND fige_systeme;
```

Create `migrations/0001_init.down.sql` : `DROP TABLE` de toutes les tables ci-dessus dans l'ordre inverse des dépendances.

- [ ] **Step 2 : Écrire le test de la couche d'accès**

Create `internal/store/db_test.go` :

```go
package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrationsEtSchema(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	tables := []string{
		"operateur", "utilisateur", "motif_rejet", "type_demande", "processus",
		"type_incident", "numero", "demande", "demande_numero", "demande_client",
		"etape_historique", "confirmation", "otp", "reverse_request", "incident",
	}
	for _, tbl := range tables {
		var n int
		err := db.Pool.QueryRow(ctx,
			"SELECT count(*) FROM information_schema.tables WHERE table_name = $1", tbl).Scan(&n)
		require.NoError(t, err)
		require.Equalf(t, 1, n, "table %s absente", tbl)
	}
}

func TestNewTestDBEstIsolee(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	_, err := db.Pool.Exec(ctx,
		`INSERT INTO otp (numero, code, expire_a, tentatives, consomme, cree_le)
		 VALUES ('770000001', '123456', now(), 0, false, now())`)
	require.NoError(t, err)

	db2 := NewTestDB(t)
	var n int
	require.NoError(t, db2.Pool.QueryRow(ctx, "SELECT count(*) FROM otp").Scan(&n))
	require.Equal(t, 0, n, "NewTestDB doit repartir d'une base vide")
}
```

- [ ] **Step 3 : Lancer le test, vérifier qu'il échoue**

Run: `make test`
Expected: FAIL — `undefined: NewTestDB`.

- [ ] **Step 4 : Implémenter `store.Open`, `store.Migrate` et `store.NewTestDB`**

Create `internal/store/db.go` — ouvre un `pgxpool.Pool`, expose `Close()`, et applique les migrations depuis le répertoire `migrations` via `golang-migrate` avec la source `file://`. Le chemin des migrations est résolu en remontant depuis le répertoire courant jusqu'à trouver un dossier `migrations`, pour que les tests fonctionnent depuis n'importe quel paquet :

```go
package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func Open(ctx context.Context, url string) (*DB, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("ouverture du pool : %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("connexion à la base : %w", err)
	}
	return &DB{Pool: pool}, nil
}

func (d *DB) Close() { d.Pool.Close() }

func Migrate(url string) error {
	dir, err := RepertoireMigrations()
	if err != nil {
		return err
	}
	m, err := migrate.New("file://"+dir, url)
	if err != nil {
		return fmt.Errorf("initialisation des migrations : %w", err)
	}
	defer m.Close()
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("application des migrations : %w", err)
	}
	return nil
}

// RepertoireMigrations remonte l'arborescence jusqu'à trouver le dossier migrations,
// pour que les tests s'exécutent depuis n'importe quel paquet.
func RepertoireMigrations() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 8; i++ {
		candidat := filepath.Join(dir, "migrations")
		if st, err := os.Stat(candidat); err == nil && st.IsDir() {
			return candidat, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("répertoire migrations introuvable")
}
```

Create `internal/store/testdb.go` — `NewTestDB` ouvre `DATABASE_URL`, applique les migrations, tronque toutes les tables métier en une instruction et rejoue le seed. `TRUNCATE ... RESTART IDENTITY CASCADE` sur l'ensemble des tables garantit l'isolation entre tests.

```go
package store

import (
	"context"
	"os"
	"testing"
)

const truncateSQL = `TRUNCATE
	incident, reverse_request, otp, confirmation, etape_historique,
	demande_client, demande_numero, demande, numero, utilisateur,
	type_incident, processus, type_demande, motif_rejet, operateur
	RESTART IDENTITY CASCADE`

// NewTestDB rend une base migrée, vidée et ensemencée. Le seed est branché en
// Task 3 via SeedFn ; tant qu'il est nil, la base reste vide.
var SeedFn func(ctx context.Context, db *DB) error

func NewTestDB(t *testing.T) *DB {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL absent — lancer via make test")
	}
	if err := Migrate(url); err != nil {
		t.Fatalf("migrations : %v", err)
	}
	ctx := context.Background()
	db, err := Open(ctx, url)
	if err != nil {
		t.Fatalf("ouverture : %v", err)
	}
	t.Cleanup(db.Close)

	if _, err := db.Pool.Exec(ctx, truncateSQL); err != nil {
		t.Fatalf("truncate : %v", err)
	}
	if SeedFn != nil {
		if err := SeedFn(ctx, db); err != nil {
			t.Fatalf("seed : %v", err)
		}
	}
	return db
}
```

- [ ] **Step 5 : Lancer le test, vérifier qu'il passe**

Run: `make test`
Expected: PASS. `TestNewTestDBEstIsolee` prouve que le truncate fonctionne.

- [ ] **Step 6 : Commit**

```bash
git add -A
git commit -m "feat: schema PostgreSQL et couche d acces pgx"
```

---

## Task 3 : Seed — référentiels, comptes, vivier de numéros

**Files:**
- Create: `internal/seed/seed.go`, `internal/seed/seed_test.go`
- Modify: `internal/store/testdb.go` (brancher `SeedFn`)
- Modify: `cmd/server/main.go` (jouer le seed au démarrage si la base est vide)

**Interfaces:**
- Consumes: `store.DB`.
- Produces: `seed.Run(ctx context.Context, db *store.DB) error`, idempotent. Constantes exportées : `seed.OperateurOrange`, `seed.OperateurYAS`, `seed.OperateurExpresso` (identifiants), `seed.MotifDernierPortage3Mois`, `seed.MotifErreurInfos`, `seed.MotifDonneesManquantes`, `seed.MotifNumeroInactif`, `seed.MotifIdentiteNonProuvee`, `seed.MotifEngagementEnCours`, `seed.TypeDemandePortage`, `seed.TypeDemandeRestitution`, `seed.TypeDemandeReverse`, `seed.ProcessusPrepaid`, `seed.ProcessusPostpaid`, `seed.TypeIncidentGateway`, `seed.TypeIncidentTechnique`.

- [ ] **Step 1 : Écrire le test du seed**

Create `internal/seed/seed_test.go` :

```go
package seed

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/store"
	"golang.org/x/crypto/bcrypt"
)

func TestOperateursIdentifiantsExacts(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()

	attendus := map[string]string{
		"6a21745ce6c37b5b5b487ec1": "ORANGE",
		"6a2174c3e6c37b5b5b487ec4": "YAS",
		"6a217510e6c37b5b5b487ec7": "EXPRESSO",
	}
	for id, nom := range attendus {
		var got string
		require.NoErrorf(t, db.Pool.QueryRow(ctx,
			"SELECT nom FROM operateur WHERE id = $1", id).Scan(&got),
			"opérateur %s absent", id)
		require.Equal(t, nom, got)
	}

	var n int
	require.NoError(t, db.Pool.QueryRow(ctx, "SELECT count(*) FROM operateur").Scan(&n))
	require.Equal(t, 3, n)
}

func TestMotifsRejetIdentifiantsExacts(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()

	attendus := map[string]string{
		"6a2175c5e6c37b5b5b487edb": "Dernier portage inférieur à 3 mois",
		"6a2175cfe6c37b5b5b487edc": "Erreur sur les infos",
		"6a2175d9e6c37b5b5b487edd": "Données manquantes",
		"6a2175e7e6c37b5b5b487ede": "Numéro Inactif",
		"6a2175f3e6c37b5b5b487edf": "Identité non prouvée",
		"6a2175fde6c37b5b5b487ee0": "Engagement en cours dans une demande",
	}
	for id, motif := range attendus {
		var got string
		require.NoErrorf(t, db.Pool.QueryRow(ctx,
			"SELECT motif FROM motif_rejet WHERE id = $1", id).Scan(&got), "motif %s absent", id)
		require.Equal(t, motif, got)
	}
}

func TestComptes(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()

	comptes := map[string]struct {
		motDePasse string
		operateur  string
	}{
		"orange":   {"orange2026", "6a21745ce6c37b5b5b487ec1"},
		"yas":      {"yas2026", "6a2174c3e6c37b5b5b487ec4"},
		"expresso": {"expresso2026", "6a217510e6c37b5b5b487ec7"},
	}
	for username, attendu := range comptes {
		var hash, operateurID string
		var roles []string
		require.NoErrorf(t, db.Pool.QueryRow(ctx,
			"SELECT password_hash, operateur_id, roles FROM utilisateur WHERE username = $1",
			username).Scan(&hash, &operateurID, &roles), "compte %s absent", username)

		require.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte(attendu.motDePasse)))
		require.Equal(t, attendu.operateur, operateurID)
		require.ElementsMatch(t, []string{"ROLE_OPERATEUR_ADMIN", "ROLE_USER"}, roles)
	}
}

func TestVivierNumeros(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()

	cas := []struct {
		msisdn            string
		operateurActuel   string
		portage           bool
		dejaRestitue      bool
		agePortageMinJour int
		agePortageMaxJour int
	}{
		{"771000001", OperateurOrange, false, false, 0, 0},
		{"761000001", OperateurYAS, false, false, 0, 0},
		{"701000001", OperateurExpresso, false, false, 0, 0},
		{"772000001", OperateurOrange, true, false, 25, 35},
		{"773000001", OperateurYAS, true, false, 230, 250},
		{"774000001", OperateurYAS, true, false, 55, 65},
		{"775000001", OperateurYAS, true, true, 230, 250},
	}
	for _, c := range cas {
		var actuel string
		var date *time.Time
		var restitue bool
		require.NoErrorf(t, db.Pool.QueryRow(ctx,
			`SELECT operateur_actuel_id, date_dernier_portage, deja_restitue
			 FROM numero WHERE msisdn = $1`, c.msisdn).Scan(&actuel, &date, &restitue),
			"numéro %s absent du vivier", c.msisdn)

		require.Equal(t, c.operateurActuel, actuel, c.msisdn)
		require.Equal(t, c.dejaRestitue, restitue, c.msisdn)
		if !c.portage {
			require.Nilf(t, date, "%s ne doit pas porter de date de portage", c.msisdn)
			continue
		}
		require.NotNilf(t, date, "%s doit porter une date de portage", c.msisdn)
		age := int(time.Since(*date).Hours() / 24)
		require.GreaterOrEqual(t, age, c.agePortageMinJour, c.msisdn)
		require.LessOrEqual(t, age, c.agePortageMaxJour, c.msisdn)
	}
}

func TestSeedIdempotent(t *testing.T) {
	db := store.NewTestDB(t)
	ctx := context.Background()

	require.NoError(t, Run(ctx, db))
	require.NoError(t, Run(ctx, db))

	var n int
	require.NoError(t, db.Pool.QueryRow(ctx, "SELECT count(*) FROM operateur").Scan(&n))
	require.Equal(t, 3, n)
}
```

- [ ] **Step 2 : Lancer le test, vérifier qu'il échoue**

Run: `make test`
Expected: FAIL — `undefined: OperateurOrange`, `undefined: Run`.

- [ ] **Step 3 : Implémenter le seed**

Create `internal/seed/seed.go`. Toutes les insertions utilisent `ON CONFLICT (id) DO NOTHING` pour l'idempotence. Les numéros du vivier sont insérés par tranche de dix (`…0001` à `…0010`) pour laisser de la marge aux tests qui consomment un numéro.

```go
package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/yas/numflex-sandbox/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// Identifiants relevés en recette ARTP — ObjectId MongoDB, non modifiables.
const (
	OperateurOrange   = "6a21745ce6c37b5b5b487ec1"
	OperateurYAS      = "6a2174c3e6c37b5b5b487ec4"
	OperateurExpresso = "6a217510e6c37b5b5b487ec7"

	MotifDernierPortage3Mois = "6a2175c5e6c37b5b5b487edb"
	MotifErreurInfos         = "6a2175cfe6c37b5b5b487edc"
	MotifDonneesManquantes   = "6a2175d9e6c37b5b5b487edd"
	MotifNumeroInactif       = "6a2175e7e6c37b5b5b487ede"
	MotifIdentiteNonProuvee  = "6a2175f3e6c37b5b5b487edf"
	MotifEngagementEnCours   = "6a2175fde6c37b5b5b487ee0"

	TypeDemandePortage     = "6a217518e6c37b5b5b487ec8"
	TypeDemandeRestitution = "6a21751be6c37b5b5b487ec9"
	TypeDemandeReverse     = "6a21751fe6c37b5b5b487eca"

	ProcessusPrepaid  = "6a217686e6c37b5b5b487ee8"
	ProcessusPostpaid = "6a217689e6c37b5b5b487ee9"

	// Identifiants du guide v2 §7.1 — seules valeurs publiées.
	TypeIncidentGateway   = "65abc456def001"
	TypeIncidentTechnique = "65abc456def002"
)

func Run(ctx context.Context, db *store.DB) error {
	operateurs := []struct{ id, nom, prefixe string }{
		{OperateurOrange, "ORANGE", "191"},
		{OperateurYAS, "YAS", "192"},
		// [HYP] Préfixe EXPRESSO non observé en recette ; 191 et 192 le sont.
		{OperateurExpresso, "EXPRESSO", "193"},
	}
	for _, o := range operateurs {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO operateur (id, nom, prefixe_routage) VALUES ($1,$2,$3)
			 ON CONFLICT (id) DO NOTHING`, o.id, o.nom, o.prefixe); err != nil {
			return fmt.Errorf("seed opérateur %s : %w", o.nom, err)
		}
	}

	motifs := []struct{ id, motif string }{
		{MotifDernierPortage3Mois, "Dernier portage inférieur à 3 mois"},
		{MotifErreurInfos, "Erreur sur les infos"},
		{MotifDonneesManquantes, "Données manquantes"},
		{MotifNumeroInactif, "Numéro Inactif"},
		{MotifIdentiteNonProuvee, "Identité non prouvée"},
		{MotifEngagementEnCours, "Engagement en cours dans une demande"},
	}
	for _, m := range motifs {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO motif_rejet (id, motif) VALUES ($1,$2)
			 ON CONFLICT (id) DO NOTHING`, m.id, m.motif); err != nil {
			return fmt.Errorf("seed motif : %w", err)
		}
	}

	types := []struct{ id, t string }{
		{TypeDemandePortage, "PORTAGE"},
		{TypeDemandeRestitution, "RESTITUTION"},
		{TypeDemandeReverse, "REVERSE"},
	}
	for _, x := range types {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO type_demande (id, type) VALUES ($1,$2)
			 ON CONFLICT (id) DO NOTHING`, x.id, x.t); err != nil {
			return fmt.Errorf("seed type de demande : %w", err)
		}
	}

	procs := []struct{ id, t string }{
		{ProcessusPrepaid, "PREPAID"},
		{ProcessusPostpaid, "POSTPAID"},
	}
	for _, p := range procs {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO processus (id, type) VALUES ($1,$2)
			 ON CONFLICT (id) DO NOTHING`, p.id, p.t); err != nil {
			return fmt.Errorf("seed processus : %w", err)
		}
	}

	incidents := []struct {
		id, libelle string
		fige        bool
	}{
		{TypeIncidentGateway, "Gateway", false},
		{TypeIncidentTechnique, "Technique", true},
	}
	for _, i := range incidents {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO type_incident (id, libelle, fige_systeme) VALUES ($1,$2,$3)
			 ON CONFLICT (id) DO NOTHING`, i.id, i.libelle, i.fige); err != nil {
			return fmt.Errorf("seed type d'incident : %w", err)
		}
	}

	comptes := []struct{ username, motDePasse, operateur string }{
		{"orange", "orange2026", OperateurOrange},
		{"yas", "yas2026", OperateurYAS},
		{"expresso", "expresso2026", OperateurExpresso},
	}
	for _, c := range comptes {
		hash, err := bcrypt.GenerateFromPassword([]byte(c.motDePasse), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hachage %s : %w", c.username, err)
		}
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO utilisateur (id, username, password_hash, operateur_id, roles)
			 VALUES ($1,$2,$3,$4,$5) ON CONFLICT (username) DO NOTHING`,
			c.operateur+"-user", c.username, string(hash), c.operateur,
			[]string{"ROLE_OPERATEUR_ADMIN", "ROLE_USER"}); err != nil {
			return fmt.Errorf("seed compte %s : %w", c.username, err)
		}
	}

	return seedNumeros(ctx, db)
}

// seedNumeros installe le vivier décrit au §10 de la spec : dix numéros par tranche,
// chaque tranche rendant exerçable une règle précise dès le premier démarrage.
func seedNumeros(ctx context.Context, db *store.DB) error {
	jours := func(n int) *time.Time {
		t := time.Now().AddDate(0, 0, -n)
		return &t
	}

	tranches := []struct {
		prefixe   string
		actuel    string
		origine   string
		portage   *time.Time
		restitue  bool
	}{
		{"77100", OperateurOrange, OperateurOrange, nil, false},
		{"76100", OperateurYAS, OperateurYAS, nil, false},
		{"70100", OperateurExpresso, OperateurExpresso, nil, false},
		{"77200", OperateurOrange, OperateurYAS, jours(30), false},
		{"77300", OperateurYAS, OperateurOrange, jours(240), false},
		{"77400", OperateurYAS, OperateurOrange, jours(60), false},
		{"77500", OperateurYAS, OperateurOrange, jours(240), true},
	}

	for _, tr := range tranches {
		for i := 1; i <= 10; i++ {
			msisdn := fmt.Sprintf("%s%04d", tr.prefixe, i)
			if _, err := db.Pool.Exec(ctx,
				`INSERT INTO numero
				   (msisdn, operateur_actuel_id, operateur_origine_id,
				    date_dernier_portage, deja_restitue, actif)
				 VALUES ($1,$2,$3,$4,$5,true)
				 ON CONFLICT (msisdn) DO NOTHING`,
				msisdn, tr.actuel, tr.origine, tr.portage, tr.restitue); err != nil {
				return fmt.Errorf("seed numéro %s : %w", msisdn, err)
			}
		}
	}
	return nil
}
```

Note : `77100` suivi de `%04d` donne `771000001` — neuf chiffres, format national.

- [ ] **Step 4 : Brancher le seed sur les tests**

Modify `internal/store/testdb.go` — supprimer la variable `SeedFn` et l'appeler directement crée un cycle d'import (`store` → `seed` → `store`). Conserver `SeedFn` et l'affecter depuis un fichier `internal/seed/register.go` :

```go
package seed

import (
	"context"

	"github.com/yas/numflex-sandbox/internal/store"
)

func init() {
	store.SeedFn = func(ctx context.Context, db *store.DB) error {
		return Run(ctx, db)
	}
}
```

Les paquets de test qui ont besoin du seed importent `_ "github.com/yas/numflex-sandbox/internal/seed"`. Le test du paquet `seed` lui-même l'obtient par son propre `init`.

- [ ] **Step 5 : Lancer les tests, vérifier qu'ils passent**

Run: `make test`
Expected: PASS — les cinq tests du paquet `seed`.

- [ ] **Step 6 : Jouer le seed au démarrage du serveur**

Modify `cmd/server/main.go` — après `store.Open`, appeler `store.Migrate` puis `seed.Run`. Le seed étant idempotent, il est rejoué à chaque démarrage sans effet.

- [ ] **Step 7 : Commit**

```bash
git add -A
git commit -m "feat: seed des referentiels ARTP, des comptes et du vivier de numeros"
```

---

## Task 4 : Couche d'erreur et double rendu

C'est le cœur du sandbox. Tout le reste en dépend.

**Files:**
- Create: `internal/apperr/apperr.go`, `internal/apperr/catalogue.go`
- Create: `internal/httpx/envelope.go`, `internal/httpx/renderer.go`
- Create: `internal/httpx/renderer_test.go`

**Interfaces:**
- Consumes: `config.Fidelity`.
- Produces:
  - `apperr.Kind` avec `apperr.KindValidation`, `KindEtat`, `KindAcces`, `KindIntrouvable`, `KindInterne`.
  - `apperr.Error{Code, Message string; Kind Kind; Fields []FieldError; RealDetail string; RealStatus int}` implémentant `error`.
  - `apperr.FieldError{ObjectName, Field, Message string}`.
  - `apperr.New(kind Kind, code, message string) *Error`.
  - `apperr.Validation(fields ...FieldError) *Error`.
  - Le catalogue : `apperr.DemandeNonTrouvee()`, `apperr.DemandeAccesRefuse(msg string)`, `apperr.EtapeInvalide(msg string)`, `apperr.MotifRejetObligatoire()`, `apperr.AccesInterdit()`, `apperr.OperateurNonTrouve()`, `apperr.FormatJSONInvalide()`, `apperr.ErreurInterne(msg string)`, `apperr.OTPInvalid()`, `apperr.OTPExpired()`, `apperr.OTPAlreadyUsed()`, `apperr.OTPMaxAttempts()`, `apperr.NumeroDejaChezDestinataire()`, `apperr.OperateurSourceIncorrect()`, `apperr.DemandeEnCoursPourNumero()`, `apperr.DelaiPortageNonRespecte()`, `apperr.NumeroNonPorte()`, `apperr.NumeroDejaRestitue()`, `apperr.DelaiRestitutionNonRespecte()`, `apperr.FlotteVide()`, `apperr.FlotteOperateursMixtes()`, `apperr.AucunNumeroEligible()`, `apperr.ValidationEchouee(msg string)`.
  - `httpx.Renderer` construit par `httpx.NewRenderer(fid config.Fidelity, skew time.Duration) *Renderer`, avec `OK(c *gin.Context, status int, message string, data any)`, `OKSansData(c *gin.Context, status int, message string)`, `Fail(c *gin.Context, err error)`, `Skew(t time.Time) time.Time`.

- [ ] **Step 1 : Écrire le test du double rendu**

Create `internal/httpx/renderer_test.go` :

```go
package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/apperr"
	"github.com/yas/numflex-sandbox/internal/config"
)

func contexte(fid config.Fidelity, chemin string) (*gin.Context, *httptest.ResponseRecorder, *Renderer) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, chemin, nil)
	return c, rec, NewRenderer(fid, 0)
}

func TestRealErreurEtatSortEn500SansCode(t *testing.T) {
	c, rec, r := contexte(config.FidelityReal, "/api/gateway/v1/demandes/traitement")

	r.Fail(c, apperr.DemandeNonTrouvee())

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))

	var corps map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &corps))
	require.NotContains(t, corps, "code", "ANO-001 : aucune erreur ne porte de champ code en mode réel")
	require.NotContains(t, corps, "success")
	require.Equal(t, "https://www.jhipster.tech/problem/problem-with-message", corps["type"])
	require.Equal(t, "Internal Server Error", corps["title"])
	require.Equal(t, float64(500), corps["status"])
	require.Equal(t, "error.http.500", corps["message"])
	require.Equal(t, "/api/gateway/v1/demandes/traitement", corps["path"])
	require.Equal(t, "RuntimeException: Demande introuvable", corps["detail"])
}

func TestRealErreurValidationSortEn400AvecFieldErrors(t *testing.T) {
	c, rec, r := contexte(config.FidelityReal, "/api/gateway/v1/demandes/particulier")

	r.Fail(c, apperr.Validation(apperr.FieldError{
		ObjectName: "demandeParticulierDTO",
		Field:      "client.lieuNaissance",
		Message:    "ne doit pas être vide",
	}))

	require.Equal(t, http.StatusBadRequest, rec.Code)

	var corps map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &corps))
	require.NotContains(t, corps, "code")
	require.Equal(t, "https://www.jhipster.tech/problem/constraint-violation", corps["type"])
	require.Equal(t, "Method argument not valid", corps["title"])
	require.Equal(t, "error.validation", corps["message"])

	champs := corps["fieldErrors"].([]any)
	require.Len(t, champs, 1)
	premier := champs[0].(map[string]any)
	require.Equal(t, "demandeParticulierDTO", premier["objectName"])
	require.Equal(t, "client.lieuNaissance", premier["field"])
	require.Equal(t, "ne doit pas être vide", premier["message"])
}

func TestRealDetailPersonnalise(t *testing.T) {
	// ANO-002 : le refus de re-portage à moins de 3 mois se présente comme une panne.
	c, rec, r := contexte(config.FidelityReal, "/api/gateway/v1/demandes/particulier")

	r.Fail(c, apperr.DelaiPortageNonRespecte())

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	var corps map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &corps))
	require.Equal(t, "Unexpected runtime exception", corps["detail"])
}

func TestContractErreurEtatSortEnveloppee(t *testing.T) {
	c, rec, r := contexte(config.FidelityContract, "/api/gateway/v1/demandes/traitement")

	r.Fail(c, apperr.DemandeNonTrouvee())

	require.Equal(t, http.StatusNotFound, rec.Code)

	var corps map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &corps))
	require.Equal(t, false, corps["success"])
	require.Equal(t, "DEMANDE_NON_TROUVEE", corps["code"])
	require.Equal(t, "Demande introuvable", corps["message"])
	require.NotContains(t, corps, "type")
}

func TestContractCorrespondanceKindStatut(t *testing.T) {
	cas := []struct {
		err    *apperr.Error
		statut int
		code   string
	}{
		{apperr.Validation(apperr.FieldError{Field: "numero", Message: "obligatoire"}), 400, "VALIDATION_ECHOUEE"},
		{apperr.DemandeNonTrouvee(), 404, "DEMANDE_NON_TROUVEE"},
		{apperr.DemandeAccesRefuse("refusé"), 403, "DEMANDE_ACCES_REFUSE"},
		{apperr.EtapeInvalide("mauvaise étape"), 409, "ETAPE_INVALIDE"},
		{apperr.ErreurInterne("boum"), 500, "ERREUR_INTERNE"},
	}
	for _, x := range cas {
		c, rec, r := contexte(config.FidelityContract, "/x")
		r.Fail(c, x.err)
		require.Equalf(t, x.statut, rec.Code, "code %s", x.code)
		var corps map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &corps))
		require.Equal(t, x.code, corps["code"])
	}
}

func TestSucces(t *testing.T) {
	for _, fid := range []config.Fidelity{config.FidelityReal, config.FidelityContract} {
		c, rec, r := contexte(fid, "/x")
		r.OK(c, http.StatusCreated, "Demande créée avec succès", map[string]string{"id": "abc"})

		require.Equal(t, http.StatusCreated, rec.Code)
		var corps map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &corps))
		require.Equal(t, true, corps["success"])
		require.Equal(t, "SUCCESS", corps["code"])
		require.Equal(t, "Demande créée avec succès", corps["message"])
		require.Equal(t, map[string]any{"id": "abc"}, corps["data"])
	}
}

func TestOKSansDataOmetLeChampEnReel(t *testing.T) {
	// ANO-011 : la réponse de otp/send ne porte pas de champ data du tout.
	c, rec, r := contexte(config.FidelityReal, "/x")
	r.OKSansData(c, http.StatusOK, "OTP envoyé avec succès")

	var corps map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &corps))
	require.NotContains(t, corps, "data")
}

func TestOKSansDataRendDataNullEnContrat(t *testing.T) {
	c, rec, r := contexte(config.FidelityContract, "/x")
	r.OKSansData(c, http.StatusOK, "OTP envoyé avec succès")

	require.Contains(t, rec.Body.String(), `"data":null`)
}

func TestSkew(t *testing.T) {
	r := NewRenderer(config.FidelityReal, 540*time.Second)
	base := time.Date(2026, 8, 21, 11, 0, 0, 0, time.UTC)
	require.Equal(t, base.Add(9*time.Minute), r.Skew(base))

	sans := NewRenderer(config.FidelityReal, 0)
	require.Equal(t, base, sans.Skew(base))
}
```

- [ ] **Step 2 : Lancer le test, vérifier qu'il échoue**

Run: `go test ./internal/httpx/...`
Expected: FAIL — `undefined: NewRenderer`.

- [ ] **Step 3 : Implémenter le type d'erreur**

Create `internal/apperr/apperr.go` :

```go
// Package apperr porte le type d'erreur unique du sandbox. Les règles métier ne
// connaissent que ce type ; le rendu HTTP — enveloppe ARTP ou problem+json JHipster —
// est décidé ailleurs, dans internal/httpx, selon le mode de fidélité.
package apperr

type Kind int

const (
	KindValidation Kind = iota
	KindEtat
	KindAcces
	KindIntrouvable
	KindInterne
)

type FieldError struct {
	ObjectName string `json:"objectName"`
	Field      string `json:"field"`
	Message    string `json:"message"`
}

type Error struct {
	// Code du catalogue §9 du guide. En mode real il n'est jamais émis (ANO-001),
	// mais il reste renseigné : c'est lui qui pilote le mode contract.
	Code    string
	Message string
	Kind    Kind
	Fields  []FieldError

	// RealDetail remplace le champ detail du problem+json en mode real, pour les
	// anomalies dont le texte mesuré diffère du message métier (ANO-002, ANO-020).
	// Vide, le rendu utilise "RuntimeException: " + Message.
	RealDetail string
}

func (e *Error) Error() string { return e.Code + " : " + e.Message }

func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

func Validation(fields ...FieldError) *Error {
	return &Error{
		Kind:    KindValidation,
		Code:    "VALIDATION_ECHOUEE",
		Message: "Un ou plusieurs champs obligatoires sont manquants ou invalides",
		Fields:  fields,
	}
}
```

- [ ] **Step 4 : Implémenter le catalogue**

Create `internal/apperr/catalogue.go`. Chaque constructeur porte le code exact du §9 et le message français. Les deux anomalies mesurées portent un `RealDetail`.

```go
package apperr

// --- OTP (§9) ---------------------------------------------------------------
// ANO-014 : en recette ces situations sortent en texte libre. Les messages
// ci-dessous sont ceux qui ont été mesurés.

func OTPInvalid() *Error {
	return New(KindEtat, "OTP_INVALID", "Code OTP incorrect")
}

func OTPExpired() *Error {
	e := New(KindEtat, "OTP_EXPIRED", "Code expiré (délai de 5 minutes dépassé)")
	e.RealDetail = "Le code OTP a expiré"
	return e
}

func OTPAlreadyUsed() *Error {
	return New(KindEtat, "OTP_ALREADY_USED", "Code déjà utilisé")
}

func OTPMaxAttempts() *Error {
	return New(KindEtat, "OTP_MAX_ATTEMPTS", "Nombre maximum de tentatives atteint (3 essais)")
}

func OTPAbsent() *Error {
	e := New(KindEtat, "OTP_INVALID", "Aucun OTP actif pour ce numéro")
	e.RealDetail = "Aucun OTP actif pour ce numéro"
	return e
}

// --- Portage (§9) -----------------------------------------------------------

func NumeroDejaChezDestinataire() *Error {
	return New(KindEtat, "NUMERO_DEJA_CHEZ_DESTINATAIRE",
		"Le numéro est déjà chez l'opérateur destinataire")
}

func OperateurSourceIncorrect() *Error {
	return New(KindEtat, "OPERATEUR_SOURCE_INCORRECT",
		"Le numéro n'appartient pas à l'opérateur source indiqué")
}

func DemandeEnCoursPourNumero() *Error {
	return New(KindEtat, "DEMANDE_EN_COURS_POUR_NUMERO",
		"Une demande est déjà en cours pour ce numéro")
}

// DelaiPortageNonRespecte — ANO-002 : la couche de validation est franchie et
// l'exception survient dans la logique métier. Le client reçoit une panne serveur
// là où le catalogue prévoit un refus métier.
func DelaiPortageNonRespecte() *Error {
	e := New(KindEtat, "DELAI_PORTAGE_NON_RESPECTE",
		"Le délai minimum entre deux portages n'est pas respecté")
	e.RealDetail = "Unexpected runtime exception"
	return e
}

// --- Restitution / reverse (§9) ---------------------------------------------

func NumeroNonPorte() *Error {
	return New(KindEtat, "NUMERO_NON_PORTE",
		"Le numéro n'a pas été porté, pas de restitution/reverse possible")
}

func NumeroDejaRestitue() *Error {
	return New(KindEtat, "NUMERO_DEJA_RESTITUE", "Ce numéro a déjà été restitué")
}

// DelaiRestitutionNonRespecte — ANO-020 : une erreur 400 sérialisée en chaîne,
// encapsulée dans une 500. Le code exploitable existe mais reste enterré.
func DelaiRestitutionNonRespecte() *Error {
	e := New(KindEtat, "DELAI_RESTITUTION_NON_RESPECTE",
		"Le délai de 6 mois minimum n'est pas écoulé")
	e.RealDetail = `400 BAD_REQUEST "{"type":"https://www.jhipster.tech/problem/problem-with-message",` +
		`"title":"Bad Request","status":400,"detail":"error.numeroRestitutionTooEarly"}"`
	return e
}

// --- Workflow (§9) ----------------------------------------------------------

func EtapeInvalide(message string) *Error {
	return New(KindEtat, "ETAPE_INVALIDE", message)
}

func MotifRejetObligatoire() *Error {
	return New(KindEtat, "MOTIF_REJET_OBLIGATOIRE",
		"Un motif de rejet est obligatoire pour rejeter une demande")
}

func DemandeNonTrouvee() *Error {
	return New(KindIntrouvable, "DEMANDE_NON_TROUVEE", "Demande introuvable")
}

func DemandeAccesRefuse(message string) *Error {
	return New(KindAcces, "DEMANDE_ACCES_REFUSE", message)
}

// --- Flotte (§9) ------------------------------------------------------------

func FlotteVide() *Error {
	return New(KindValidation, "FLOTTE_VIDE", "La liste des numéros de flotte est vide")
}

func FlotteOperateursMixtes() *Error {
	return New(KindEtat, "FLOTTE_OPERATEURS_MIXTES",
		"Les numéros appartiennent à des opérateurs différents")
}

func AucunNumeroEligible() *Error {
	return New(KindEtat, "AUCUN_NUMERO_ELIGIBLE",
		"Aucun numéro de la flotte n'est éligible au portage")
}

// --- Accès et validation (§9) -----------------------------------------------

func AccesInterdit() *Error {
	return New(KindAcces, "ACCES_INTERDIT",
		"Token JWT absent, invalide ou expiré. Veuillez vous authentifier à nouveau.")
}

func OperateurNonTrouve() *Error {
	return New(KindAcces, "OPERATEUR_NON_TROUVE",
		"Votre compte n'est pas associé à un opérateur")
}

func ValidationEchouee(message string) *Error {
	return New(KindValidation, "VALIDATION_ECHOUEE", message)
}

func FormatJSONInvalide() *Error {
	return New(KindValidation, "FORMAT_JSON_INVALIDE",
		"Le corps de la requête n'est pas un JSON valide")
}

func ErreurInterne(message string) *Error {
	return New(KindInterne, "ERREUR_INTERNE", message)
}
```

- [ ] **Step 5 : Implémenter les deux rendus**

Create `internal/httpx/envelope.go` :

```go
package httpx

// Enveloppe de succès du §8 du guide, identique dans les deux modes de fidélité.
type Envelope struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

// EnvelopeSansData sert ANO-011 : la réponse de otp/send omet le champ data —
// il n'est ni présent, ni null.
type EnvelopeSansData struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
```

Create `internal/httpx/renderer.go` :

```go
package httpx

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yas/numflex-sandbox/internal/apperr"
	"github.com/yas/numflex-sandbox/internal/config"
)

type Renderer struct {
	fid  config.Fidelity
	skew time.Duration
}

func NewRenderer(fid config.Fidelity, skew time.Duration) *Renderer {
	return &Renderer{fid: fid, skew: skew}
}

func (r *Renderer) Fidelity() config.Fidelity { return r.fid }

// Skew applique la dérive d'horloge serveur mesurée en recette (ANO-015, ~9 min).
// Elle ne touche que les horodatages rendus ; ceux stockés en base restent justes.
func (r *Renderer) Skew(t time.Time) time.Time { return t.Add(r.skew) }

func (r *Renderer) OK(c *gin.Context, status int, message string, data any) {
	c.JSON(status, Envelope{Success: true, Code: "SUCCESS", Message: message, Data: data})
}

func (r *Renderer) OKSansData(c *gin.Context, status int, message string) {
	if r.fid == config.FidelityReal {
		c.JSON(status, EnvelopeSansData{Success: true, Code: "SUCCESS", Message: message})
		return
	}
	c.JSON(status, Envelope{Success: true, Code: "SUCCESS", Message: message, Data: nil})
}

type problem struct {
	Type        string               `json:"type"`
	Title       string               `json:"title"`
	Status      int                  `json:"status"`
	Detail      string               `json:"detail,omitempty"`
	Path        string               `json:"path"`
	Message     string               `json:"message"`
	FieldErrors []apperr.FieldError  `json:"fieldErrors,omitempty"`
}

func (r *Renderer) Fail(c *gin.Context, err error) {
	var e *apperr.Error
	if !errors.As(err, &e) {
		e = apperr.ErreurInterne(err.Error())
	}
	if r.fid == config.FidelityContract {
		r.failContrat(c, e)
		return
	}
	r.failReel(c, e)
}

// failReel reproduit ANO-001, ANO-003 et ANO-004 : aucune enveloppe, aucun champ
// code, les erreurs métier en 500, et le nom de la classe d'exception Java exposé.
func (r *Renderer) failReel(c *gin.Context, e *apperr.Error) {
	chemin := ""
	if c.Request != nil {
		chemin = c.Request.URL.Path
	}

	if e.Kind == apperr.KindValidation {
		c.Header("Content-Type", "application/problem+json")
		c.JSON(http.StatusBadRequest, problem{
			Type:        "https://www.jhipster.tech/problem/constraint-violation",
			Title:       "Method argument not valid",
			Status:      http.StatusBadRequest,
			Path:        chemin,
			Message:     "error.validation",
			FieldErrors: e.Fields,
		})
		c.Abort()
		return
	}

	detail := e.RealDetail
	if detail == "" {
		detail = "RuntimeException: " + e.Message
	}
	c.Header("Content-Type", "application/problem+json")
	c.JSON(http.StatusInternalServerError, problem{
		Type:    "https://www.jhipster.tech/problem/problem-with-message",
		Title:   "Internal Server Error",
		Status:  http.StatusInternalServerError,
		Detail:  detail,
		Path:    chemin,
		Message: "error.http.500",
	})
	c.Abort()
}

func (r *Renderer) failContrat(c *gin.Context, e *apperr.Error) {
	c.JSON(statutContrat(e.Kind), Envelope{
		Success: false,
		Code:    e.Code,
		Message: e.Message,
		Data:    nil,
	})
	c.Abort()
}

func statutContrat(k apperr.Kind) int {
	switch k {
	case apperr.KindValidation:
		return http.StatusBadRequest
	case apperr.KindIntrouvable:
		return http.StatusNotFound
	case apperr.KindAcces:
		return http.StatusForbidden
	case apperr.KindEtat:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
```

- [ ] **Step 6 : Lancer les tests, vérifier qu'ils passent**

Run: `go test ./internal/httpx/... -v`
Expected: PASS — les neuf tests.

- [ ] **Step 7 : Commit**

```bash
git add -A
git commit -m "feat: couche d erreur unique et double rendu real/contract"
```

---

## Task 5 : Authentification, middleware et routeur

**Files:**
- Create: `internal/auth/jwt.go`, `internal/auth/middleware.go`
- Create: `internal/api/auth.go`, `internal/api/router.go`, `internal/api/deps.go`
- Create: `internal/api/auth_test.go`, `internal/api/testutil_test.go`
- Modify: `cmd/server/main.go`

**Interfaces:**
- Consumes: `config.Config`, `store.DB`, `httpx.Renderer`, `apperr`.
- Produces:
  - `auth.Emettre(secret string, ttl time.Duration, username string, roles []string) (string, error)`
  - `auth.Verifier(secret, token string) (username string, err error)`
  - `auth.Middleware(d *api.Deps) gin.HandlerFunc` — placé dans `internal/api` sous le nom `api.Authentifier()` pour éviter un import circulaire.
  - `api.Deps{Cfg *config.Config; DB *store.DB; R *httpx.Renderer}`.
  - `api.Appelant(c *gin.Context) api.Identite` où `Identite{UtilisateurID, Username, OperateurID, OperateurNom string}`.
  - `api.NewRouter(d *Deps) *gin.Engine` — déclare toutes les routes, et uniquement celles du §4 de la spec.

- [ ] **Step 1 : Écrire les tests d'authentification**

Create `internal/api/testutil_test.go` :

```go
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/config"
	"github.com/yas/numflex-sandbox/internal/httpx"
	_ "github.com/yas/numflex-sandbox/internal/seed"
	"github.com/yas/numflex-sandbox/internal/store"
)

type harnais struct {
	t   *testing.T
	srv *httptest.Server
	cfg *config.Config
	db  *store.DB
}

// nouveauHarnais monte le serveur complet sur une base de test ensemencée,
// en profil déterministe sauf réglages explicites.
func nouveauHarnais(t *testing.T, ajuste ...func(*config.Config)) *harnais {
	t.Helper()
	db := store.NewTestDB(t)

	cfg := &config.Config{
		Port:           "0",
		JWTSecret:      "test-secret",
		JWTTTL:         24 * time.Hour,
		Fidelity:       config.FidelityReal,
		EngineTick:     10 * time.Millisecond,
		OTPStaticCode:  "123456",
		OTPTTL:         5 * time.Minute,
		OTPMaxAttempts: 3,
	}
	for _, f := range ajuste {
		f(cfg)
	}

	// Le champ Moteur reste nil jusqu'à la Task 9, qui met ce harnais à jour.
	d := &Deps{Cfg: cfg, DB: db, R: httpx.NewRenderer(cfg.Fidelity, cfg.ClockSkew)}
	srv := httptest.NewServer(NewRouter(d))
	t.Cleanup(srv.Close)

	return &harnais{t: t, srv: srv, cfg: cfg, db: db}
}

// jeton authentifie un compte du seed et retourne son id_token.
func (h *harnais) jeton(username, motDePasse string) string {
	h.t.Helper()
	rep := h.brut(http.MethodPost, "/api/authenticate", "", map[string]any{
		"username": username, "password": motDePasse, "rememberMe": false,
	})
	require.Equal(h.t, http.StatusOK, rep.StatusCode)

	var corps struct {
		IDToken string `json:"id_token"`
	}
	require.NoError(h.t, json.NewDecoder(rep.Body).Decode(&corps))
	require.NotEmpty(h.t, corps.IDToken)
	return corps.IDToken
}

func (h *harnais) brut(methode, chemin, jeton string, corps any) *http.Response {
	h.t.Helper()
	var body *bytes.Reader
	if corps != nil {
		b, err := json.Marshal(corps)
		require.NoError(h.t, err)
		body = bytes.NewReader(b)
	} else {
		body = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(methode, h.srv.URL+chemin, body)
	require.NoError(h.t, err)
	req.Header.Set("Content-Type", "application/json")
	if jeton != "" {
		req.Header.Set("Authorization", "Bearer "+jeton)
	}
	rep, err := http.DefaultClient.Do(req)
	require.NoError(h.t, err)
	h.t.Cleanup(func() { rep.Body.Close() })
	return rep
}

// appel exécute une requête authentifiée et décode le corps en map.
func (h *harnais) appel(methode, chemin, jeton string, corps any) (*http.Response, map[string]any) {
	h.t.Helper()
	rep := h.brut(methode, chemin, jeton, corps)
	var decode map[string]any
	_ = json.NewDecoder(rep.Body).Decode(&decode)
	return rep, decode
}

// liste exécute un GET authentifié dont data est un tableau.
func (h *harnais) liste(chemin, jeton string) []any {
	h.t.Helper()
	rep, corps := h.appel(http.MethodGet, chemin, jeton, nil)
	require.Equal(h.t, http.StatusOK, rep.StatusCode, chemin)
	data, ok := corps["data"].([]any)
	require.Truef(h.t, ok, "%s : data n'est pas un tableau (%v)", chemin, corps)
	return data
}
```

Create `internal/api/auth_test.go` :

```go
package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthentificationNominale(t *testing.T) {
	h := nouveauHarnais(t)
	for _, c := range []struct{ user, pass string }{
		{"orange", "orange2026"},
		{"yas", "yas2026"},
		{"expresso", "expresso2026"},
	} {
		require.NotEmpty(t, h.jeton(c.user, c.pass), c.user)
	}
}

func TestAuthentificationMauvaisMotDePasse(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/authenticate", "", map[string]any{
		"username": "yas", "password": "faux", "rememberMe": false,
	})
	// ANO-016 : l'échec d'authentification sort hors enveloppe.
	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
	require.NotContains(t, corps, "success")
	require.NotContains(t, corps, "code")
}

func TestGetAuthenticateAvecJetonValide(t *testing.T) {
	h := nouveauHarnais(t)
	rep := h.brut(http.MethodGet, "/api/authenticate", h.jeton("yas", "yas2026"), nil)
	require.Equal(t, http.StatusNoContent, rep.StatusCode)
}

func TestJetonAbsentRendEnveloppeAccesInterdit(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodGet, "/api/gateway/v1/operateurs", "", nil)

	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
	require.Equal(t, false, corps["success"])
	require.Equal(t, "ACCES_INTERDIT", corps["code"])
	require.Equal(t,
		"Token JWT absent, invalide ou expiré. Veuillez vous authentifier à nouveau.",
		corps["message"])
}

func TestJetonInvalideRendCorpsVideSansContentType(t *testing.T) {
	// ANO-008 : jeton invalide → 401, corps vide, aucun Content-Type.
	h := nouveauHarnais(t)
	rep := h.brut(http.MethodGet, "/api/gateway/v1/operateurs", "jeton.bidon.xxx", nil)

	require.Equal(t, http.StatusUnauthorized, rep.StatusCode)
	require.Equal(t, "", rep.Header.Get("Content-Type"))
	require.Equal(t, int64(0), rep.ContentLength)
}
```

- [ ] **Step 2 : Lancer les tests, vérifier qu'ils échouent**

Run: `make test`
Expected: FAIL — `undefined: NewRouter`, `undefined: Deps`.

- [ ] **Step 3 : Implémenter le JWT**

Create `internal/auth/jwt.go` :

```go
// Package auth émet et vérifie les jetons du sandbox. HS512, 24 h, comme mesuré
// en recette ARTP.
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func Emettre(secret string, ttl time.Duration, username string, roles []string) (string, error) {
	maintenant := time.Now()
	t := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"sub":  username,
		"auth": rolesEnChaine(roles),
		"iat":  maintenant.Unix(),
		"exp":  maintenant.Add(ttl).Unix(),
	})
	return t.SignedString([]byte(secret))
}

func Verifier(secret, jeton string) (string, error) {
	t, err := jwt.Parse(jeton, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("algorithme inattendu : %v", t.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS512"}))
	if err != nil {
		return "", err
	}
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("revendications illisibles")
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", fmt.Errorf("sujet absent")
	}
	return sub, nil
}

func rolesEnChaine(roles []string) string {
	out := ""
	for i, r := range roles {
		if i > 0 {
			out += " "
		}
		out += r
	}
	return out
}
```

- [ ] **Step 4 : Implémenter les dépendances, le middleware et les handlers d'authentification**

Create `internal/api/deps.go` :

```go
package api

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/yas/numflex-sandbox/internal/config"
	"github.com/yas/numflex-sandbox/internal/httpx"
	"github.com/yas/numflex-sandbox/internal/store"
)

type Deps struct {
	Cfg *config.Config
	DB  *store.DB
	R   *httpx.Renderer
	// Moteur est renseigné en Task 9. Déclaré ici en interface pour que le
	// paquet api ne dépende pas de l'ordre de livraison des tâches.
	Moteur Moteur
}

// Moteur : la part du comportement de la plateforme que les appels ne pilotent pas.
type Moteur interface {
	PlaceGelee(ctx context.Context) (bool, error)
	PlanifierTransition(ctx context.Context, demandeID string) error
}

// Identite décrit l'opérateur derrière le jeton présenté.
type Identite struct {
	UtilisateurID string
	Username      string
	OperateurID   string
	OperateurNom  string
}

const cleIdentite = "numflex.identite"

func Appelant(c *gin.Context) Identite {
	v, _ := c.Get(cleIdentite)
	id, _ := v.(Identite)
	return id
}
```

Create `internal/api/auth.go` :

```go
package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yas/numflex-sandbox/internal/apperr"
	"github.com/yas/numflex-sandbox/internal/auth"
	"github.com/yas/numflex-sandbox/internal/httpx"
	"golang.org/x/crypto/bcrypt"
)

type demandeAuth struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	RememberMe bool   `json:"rememberMe"`
}

func (d *Deps) postAuthenticate(c *gin.Context) {
	var req demandeAuth
	if err := c.ShouldBindJSON(&req); err != nil {
		d.R.Fail(c, apperr.FormatJSONInvalide())
		return
	}

	var hash string
	var roles []string
	err := d.DB.Pool.QueryRow(c, `SELECT password_hash, roles FROM utilisateur WHERE username = $1`,
		req.Username).Scan(&hash, &roles)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		// ANO-016 : hors enveloppe, en problem+json JHipster.
		c.Header("Content-Type", "application/problem+json")
		c.JSON(http.StatusUnauthorized, gin.H{
			"type":   "https://www.jhipster.tech/problem/problem-with-message",
			"title":  "Unauthorized",
			"status": http.StatusUnauthorized,
			"detail": "Bad credentials",
			"path":   c.Request.URL.Path,
			"message": "error.http.401",
		})
		c.Abort()
		return
	}

	jeton, err := auth.Emettre(d.Cfg.JWTSecret, d.Cfg.JWTTTL, req.Username, roles)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("émission du jeton impossible"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"id_token": jeton})
}

// getAuthenticate confirme l'authentification — 204 No Content.
func (d *Deps) getAuthenticate(c *gin.Context) { c.Status(http.StatusNoContent) }

// Authentifier applique les deux comportements mesurés en recette :
// jeton ABSENT → 401 avec l'enveloppe ARTP ACCES_INTERDIT (le seul code jamais émis) ;
// jeton PRÉSENT mais invalide → 401, corps vide, sans Content-Type (ANO-008).
func (d *Deps) Authentifier() gin.HandlerFunc {
	return func(c *gin.Context) {
		entete := c.GetHeader("Authorization")
		if !strings.HasPrefix(entete, "Bearer ") || strings.TrimSpace(entete[7:]) == "" {
			e := apperr.AccesInterdit()
			c.JSON(http.StatusUnauthorized, httpx.Envelope{
				Success: false, Code: e.Code, Message: e.Message, Data: nil,
			})
			c.Abort()
			return
		}

		username, err := auth.Verifier(d.Cfg.JWTSecret, strings.TrimSpace(entete[7:]))
		if err != nil {
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}

		var ident Identite
		err = d.DB.Pool.QueryRow(c,
			`SELECT u.id, u.username, o.id, o.nom
			   FROM utilisateur u JOIN operateur o ON o.id = u.operateur_id
			  WHERE u.username = $1`, username).
			Scan(&ident.UtilisateurID, &ident.Username, &ident.OperateurID, &ident.OperateurNom)
		if err != nil {
			d.R.Fail(c, apperr.OperateurNonTrouve())
			return
		}

		c.Set(cleIdentite, ident)
		c.Next()
	}
}
```

- [ ] **Step 5 : Implémenter le routeur**

Create `internal/api/router.go`. **Ce fichier est le garde-fou du périmètre : aucune route qui ne figure pas au §4 de la spec.** Les handlers non encore écrits sont déclarés au fil des tâches suivantes ; à cette étape, seules les routes d'authentification sont câblées et le groupe gateway est créé vide.

```go
package api

import "github.com/gin-gonic/gin"

// NewRouter déclare EXACTEMENT les 33 routes du contrat ARTP, plus les deux
// routes d'authentification. Aucune route de santé, de metrics ou de debug :
// le sandbox doit présenter la même surface que la plateforme réelle.
func NewRouter(d *Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.POST("/api/authenticate", d.postAuthenticate)
	r.GET("/api/authenticate", d.Authentifier(), d.getAuthenticate)

	g := r.Group("/api/gateway/v1", d.Authentifier())
	d.routesReferentiels(g)  // Task 6
	d.routesOTP(g)           // Task 7
	d.routesCreation(g)      // Tasks 10-12
	d.routesLecture(g)       // Task 13
	d.routesAcceptation(g)   // Task 14
	d.routesConfirmation(g)  // Task 15
	d.routesTraitement(g)    // Task 16
	d.routesAnnulation(g)    // Task 17
	d.routesIncidents(g)     // Task 18
	d.routesReverse(g)       // Task 19

	return r
}
```

À cette étape, créer chacune de ces dix méthodes `routesXxx(g *gin.RouterGroup)` dans son fichier cible avec un corps vide, pour que le projet compile. Chaque tâche suivante la remplit.

- [ ] **Step 6 : Lancer les tests, vérifier qu'ils passent**

Run: `make test`
Expected: PASS — les cinq tests d'authentification.

- [ ] **Step 7 : Câbler le serveur**

Modify `cmd/server/main.go` — charger la configuration, ouvrir la base, migrer, ensemencer, construire `Deps`, et servir `NewRouter` sur `cfg.Port`.

- [ ] **Step 8 : Commit**

```bash
git add -A
git commit -m "feat: authentification JWT, middleware et routeur du perimetre contractuel"
```

---

## Task 6 : Données de référence — les 5 GET

**Files:**
- Create: `internal/api/referentiels.go`, `internal/api/referentiels_test.go`

**Interfaces:**
- Consumes: `Deps`, seed.
- Produces: `(*Deps).routesReferentiels(g *gin.RouterGroup)` câblant `/operateurs`, `/motifs-rejet`, `/types-demande`, `/processus`, `/types-incident`.

- [ ] **Step 1 : Écrire le test**

Create `internal/api/referentiels_test.go` :

```go
package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOperateursRenvoieLesTroisIdentifiantsDeRecette(t *testing.T) {
	h := nouveauHarnais(t)
	data := h.liste("/api/gateway/v1/operateurs", h.jeton("yas", "yas2026"))

	require.Len(t, data, 3)
	vus := map[string]string{}
	for _, e := range data {
		m := e.(map[string]any)
		require.Len(t, m, 2, "un opérateur ne porte que id et nom")
		vus[m["id"].(string)] = m["nom"].(string)
	}
	require.Equal(t, map[string]string{
		"6a21745ce6c37b5b5b487ec1": "ORANGE",
		"6a2174c3e6c37b5b5b487ec4": "YAS",
		"6a217510e6c37b5b5b487ec7": "EXPRESSO",
	}, vus)
}

func TestOperateursMessageExact(t *testing.T) {
	h := nouveauHarnais(t)
	_, corps := h.appel("GET", "/api/gateway/v1/operateurs", h.jeton("yas", "yas2026"), nil)
	require.Equal(t, "Opérateurs récupérés avec succès", corps["message"])
	require.Equal(t, "SUCCESS", corps["code"])
	require.Equal(t, true, corps["success"])
}

func TestMotifsRejetExposeMotifPasLibelle(t *testing.T) {
	// ANO-009 : le champ s'appelle motif. La v2 le documente ainsi.
	h := nouveauHarnais(t)
	data := h.liste("/api/gateway/v1/motifs-rejet", h.jeton("orange", "orange2026"))

	require.Len(t, data, 6)
	premier := data[0].(map[string]any)
	require.Contains(t, premier, "motif")
	require.NotContains(t, premier, "libelle")
}

func TestTypesDemande(t *testing.T) {
	h := nouveauHarnais(t)
	data := h.liste("/api/gateway/v1/types-demande", h.jeton("yas", "yas2026"))

	types := []string{}
	for _, e := range data {
		types = append(types, e.(map[string]any)["type"].(string))
	}
	require.ElementsMatch(t, []string{"PORTAGE", "RESTITUTION", "REVERSE"}, types)
}

func TestProcessus(t *testing.T) {
	h := nouveauHarnais(t)
	data := h.liste("/api/gateway/v1/processus", h.jeton("yas", "yas2026"))

	types := []string{}
	for _, e := range data {
		types = append(types, e.(map[string]any)["type"].(string))
	}
	require.ElementsMatch(t, []string{"PREPAID", "POSTPAID"}, types)
}

func TestTypesIncidentPorteFigeSysteme(t *testing.T) {
	h := nouveauHarnais(t)
	data := h.liste("/api/gateway/v1/types-incident", h.jeton("yas", "yas2026"))

	require.Len(t, data, 2)
	par := map[string]bool{}
	for _, e := range data {
		m := e.(map[string]any)
		par[m["libelle"].(string)] = m["figeSysteme"].(bool)
	}
	require.Equal(t, map[string]bool{"Gateway": false, "Technique": true}, par)
}
```

- [ ] **Step 2 : Lancer les tests, vérifier qu'ils échouent**

Run: `make test`
Expected: FAIL — 404 sur les cinq routes.

- [ ] **Step 3 : Implémenter les référentiels**

Create `internal/api/referentiels.go`. Les DTO n'exposent que les champs du guide — un opérateur ne porte pas son préfixe de routage.

```go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yas/numflex-sandbox/internal/apperr"
)

func (d *Deps) routesReferentiels(g *gin.RouterGroup) {
	g.GET("/operateurs", d.getOperateurs)
	g.GET("/motifs-rejet", d.getMotifsRejet)
	g.GET("/types-demande", d.getTypesDemande)
	g.GET("/processus", d.getProcessus)
	g.GET("/types-incident", d.getTypesIncident)
}

type operateurDTO struct {
	ID  string `json:"id"`
	Nom string `json:"nom"`
}

func (d *Deps) getOperateurs(c *gin.Context) {
	rows, err := d.DB.Pool.Query(c, `SELECT id, nom FROM operateur ORDER BY nom`)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("lecture des opérateurs"))
		return
	}
	defer rows.Close()

	out := []operateurDTO{}
	for rows.Next() {
		var o operateurDTO
		if err := rows.Scan(&o.ID, &o.Nom); err != nil {
			d.R.Fail(c, apperr.ErreurInterne("lecture des opérateurs"))
			return
		}
		out = append(out, o)
	}
	d.R.OK(c, http.StatusOK, "Opérateurs récupérés avec succès", out)
}
```

Écrire sur le même modèle `getMotifsRejet` (`{id, motif}`, message `Motifs de rejet récupérés avec succès`), `getTypesDemande` (`{id, type}`, `Types de demande récupérés avec succès`), `getProcessus` (`{id, type}`, `Processus récupérés avec succès`), `getTypesIncident` (`{id, libelle, figeSysteme}`, `Types d'incident récupérés avec succès`).

- [ ] **Step 4 : Lancer les tests, vérifier qu'ils passent**

Run: `make test`
Expected: PASS — les six tests.

- [ ] **Step 5 : Commit**

```bash
git add -A
git commit -m "feat: endpoints de donnees de reference"
```

---

## Task 7 : OTP

**Files:**
- Create: `internal/api/otp.go`, `internal/api/otp_test.go`

**Interfaces:**
- Consumes: `Deps`, `config.OTPStaticCode`, `config.OTPTTL`, `config.OTPMaxAttempts`.
- Produces: `(*Deps).routesOTP(g *gin.RouterGroup)` ; fonction interne `(*Deps).verifierOTP(ctx context.Context, numero, code string) *apperr.Error` réutilisée par la création de demandes (Tasks 10-11), et `(*Deps).consommerOTP(ctx context.Context, numero string) error`.

- [ ] **Step 1 : Écrire les tests**

Create `internal/api/otp_test.go` :

```go
package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOTPSendOmetLeChampDataEnModeReel(t *testing.T) {
	// ANO-011 : le champ data est absent, pas null.
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/otp/send",
		h.jeton("yas", "yas2026"), map[string]any{"numero": "771000001"})

	require.Equal(t, http.StatusOK, rep.StatusCode)
	require.Equal(t, true, corps["success"])
	require.NotContains(t, corps, "data")
}

func TestOTPVerifyNeConsommePas(t *testing.T) {
	// TC-021 : verify pré-vérifie sans consommer — le code reste utilisable.
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")

	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	for i := 0; i < 3; i++ {
		rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/otp/verify", jeton,
			map[string]any{"numero": "771000001", "otpCode": "123456"})
		require.Equal(t, http.StatusOK, rep.StatusCode, "vérification %d", i)
	}

	var consomme bool
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT consomme FROM otp WHERE numero = $1", "771000001").Scan(&consomme))
	require.False(t, consomme)
}

func TestOTPVerifyCodeIncorrect(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/otp/verify", jeton,
		map[string]any{"numero": "771000001", "otpCode": "000000"})

	// ANO-003 : les erreurs d'état sortent en 500 en mode réel.
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.NotContains(t, corps, "code")
}

func TestOTPMaxTentatives(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	for i := 0; i < 3; i++ {
		h.appel(http.MethodPost, "/api/gateway/v1/otp/verify", jeton,
			map[string]any{"numero": "771000001", "otpCode": "000000"})
	}

	// La quatrième tentative est refusée même avec le bon code.
	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/otp/verify", jeton,
		map[string]any{"numero": "771000001", "otpCode": "123456"})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestOTPExpire(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	_, err := h.db.Pool.Exec(context.Background(),
		"UPDATE otp SET expire_a = now() - interval '1 minute' WHERE numero = $1", "771000001")
	require.NoError(t, err)

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/otp/verify", jeton,
		map[string]any{"numero": "771000001", "otpCode": "123456"})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "Le code OTP a expiré", corps["detail"])
}

func TestOTPAbsent(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/otp/verify",
		h.jeton("yas", "yas2026"),
		map[string]any{"numero": "779999999", "otpCode": "123456"})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "Aucun OTP actif pour ce numéro", corps["detail"])
}

func TestOTPNumeroInvalideEstUneErreurDeValidation(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/otp/send",
		h.jeton("yas", "yas2026"), map[string]any{"numero": "77"})

	require.Equal(t, http.StatusBadRequest, rep.StatusCode)
	require.Contains(t, corps, "fieldErrors")
}
```

- [ ] **Step 2 : Lancer les tests, vérifier qu'ils échouent**

Run: `make test`
Expected: FAIL — 404 sur `/otp/send`.

- [ ] **Step 3 : Implémenter l'OTP**

Create `internal/api/otp.go` :

```go
package api

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/yas/numflex-sandbox/internal/apperr"
)

var motifMSISDN = regexp.MustCompile(`^[0-9]{9}$`)

func (d *Deps) routesOTP(g *gin.RouterGroup) {
	g.POST("/otp/send", d.postOTPSend)
	g.POST("/otp/verify", d.postOTPVerify)
}

type reqOTPSend struct {
	Numero string `json:"numero"`
}

type reqOTPVerify struct {
	Numero  string `json:"numero"`
	OtpCode string `json:"otpCode"`
}

func (d *Deps) postOTPSend(c *gin.Context) {
	var req reqOTPSend
	if err := c.ShouldBindJSON(&req); err != nil {
		d.R.Fail(c, apperr.FormatJSONInvalide())
		return
	}
	if !motifMSISDN.MatchString(req.Numero) {
		d.R.Fail(c, apperr.Validation(apperr.FieldError{
			ObjectName: "otpSendDTO", Field: "numero",
			Message: "doit correspondre à \"^[0-9]{9}$\"",
		}))
		return
	}

	maintenant := time.Now()
	_, err := d.DB.Pool.Exec(c,
		`INSERT INTO otp (numero, code, expire_a, tentatives, consomme, cree_le)
		 VALUES ($1,$2,$3,0,false,$4)
		 ON CONFLICT (numero) DO UPDATE
		   SET code = EXCLUDED.code, expire_a = EXCLUDED.expire_a,
		       tentatives = 0, consomme = false, cree_le = EXCLUDED.cree_le`,
		req.Numero, d.Cfg.OTPStaticCode, maintenant.Add(d.Cfg.OTPTTL), maintenant)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("enregistrement de l'OTP"))
		return
	}

	// Le sandbox n'envoie pas de SMS : le code est statique et journalisé.
	// La réponse acquitte la soumission, pas la remise (ANO-021).
	d.R.OKSansData(c, http.StatusOK, "OTP envoyé avec succès")
}

func (d *Deps) postOTPVerify(c *gin.Context) {
	var req reqOTPVerify
	if err := c.ShouldBindJSON(&req); err != nil {
		d.R.Fail(c, apperr.FormatJSONInvalide())
		return
	}
	if e := d.verifierOTP(c, req.Numero, req.OtpCode); e != nil {
		d.R.Fail(c, e)
		return
	}
	d.R.OKSansData(c, http.StatusOK, "Code OTP vérifié avec succès")
}

// verifierOTP pré-vérifie sans consommer (TC-021) : le code reste utilisable pour
// créer la demande. Seules les tentatives ratées sont décomptées.
func (d *Deps) verifierOTP(ctx context.Context, numero, code string) *apperr.Error {
	var stocke string
	var expireA time.Time
	var tentatives int
	var consomme bool

	err := d.DB.Pool.QueryRow(ctx,
		`SELECT code, expire_a, tentatives, consomme FROM otp WHERE numero = $1`, numero).
		Scan(&stocke, &expireA, &tentatives, &consomme)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.OTPAbsent()
	}
	if err != nil {
		return apperr.ErreurInterne("lecture de l'OTP")
	}

	if consomme {
		return apperr.OTPAlreadyUsed()
	}
	if tentatives >= d.Cfg.OTPMaxAttempts {
		return apperr.OTPMaxAttempts()
	}
	if time.Now().After(expireA) {
		return apperr.OTPExpired()
	}
	if code != stocke {
		_, _ = d.DB.Pool.Exec(ctx,
			`UPDATE otp SET tentatives = tentatives + 1 WHERE numero = $1`, numero)
		return apperr.OTPInvalid()
	}
	return nil
}

func (d *Deps) consommerOTP(ctx context.Context, numero string) error {
	_, err := d.DB.Pool.Exec(ctx, `UPDATE otp SET consomme = true WHERE numero = $1`, numero)
	return err
}
```

- [ ] **Step 4 : Lancer les tests, vérifier qu'ils passent**

Run: `make test`
Expected: PASS — les sept tests.

- [ ] **Step 5 : Commit**

```bash
git add -A
git commit -m "feat: OTP statique avec pre-verification non consommatrice"
```

---

## Task 8 : Domaine — machine à états, responsables, confirmateurs

Paquet pur : ni Gin, ni pgx, ni fidélité. Tout ce qui suit se teste sans base.

**Files:**
- Create: `internal/domain/demande.go`, `internal/domain/etapes.go`
- Create: `internal/domain/etapes_test.go`

**Interfaces:**
- Consumes: `apperr`.
- Produces:
  - Types `domain.Etape`, `domain.StatutDemande`, `domain.StatutEtape`, `domain.TypeDemande`, `domain.TypeAbonne`, `domain.Role`.
  - Constantes `EtapeAcceptation`, `EtapeDesactivation`, `EtapeActivation`, `EtapeConfirmation`, `EtapeCompletion` ; `StatutEnCours`, `StatutTermine`, `StatutAnnule`, `StatutRejete` ; `EtapeEnCours`, `EtapeTerminee`, `EtapeExpiree`, `EtapeValidee` ; `TypePortage`, `TypeRestitution`, `TypeReverse` ; `AbonneParticulier`, `AbonneEntreprise` ; `RoleSource`, `RoleDestinataire`, `RoleTous`, `RoleARTP`.
  - `domain.Demande` : vue métier d'une demande (`ID`, `Numero`, `TypeDemande`, `TypeAbonne`, `StatutDemande`, `EtapeActuelle`, `StatutEtapeActuel`, `OperateurSourceID`, `OperateurDestinataireID`, `CreateurOperateurID`, `TransitionEnAttente bool`).
  - `domain.EtapeSuivante(e Etape) (Etape, bool)`
  - `domain.ResponsableEtape(e Etape, td TypeDemande) Role`
  - `domain.EndpointEtape(e Etape) string`
  - `domain.PeutTraiter(d Demande, operateurID string) *apperr.Error`
  - `domain.ConfirmateursAttendus(d Demande, tousOperateurs []string) []string`
  - `domain.PeutAnnuler(d Demande, operateurID string) *apperr.Error`
  - `domain.PeutAccepter(d Demande, operateurID string) *apperr.Error`

- [ ] **Step 1 : Écrire les tests du domaine**

Create `internal/domain/etapes_test.go` :

```go
package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/apperr"
)

const (
	orange   = "6a21745ce6c37b5b5b487ec1"
	yas      = "6a2174c3e6c37b5b5b487ec4"
	expresso = "6a217510e6c37b5b5b487ec7"
)

var place = []string{orange, yas, expresso}

func portage() Demande {
	return Demande{
		ID: "d1", TypeDemande: TypePortage, TypeAbonne: AbonneParticulier,
		StatutDemande: StatutEnCours, EtapeActuelle: EtapeAcceptation,
		StatutEtapeActuel: EtapeEnCours,
		OperateurSourceID: orange, OperateurDestinataireID: yas,
		CreateurOperateurID: yas,
	}
}

func TestSequenceDesEtapes(t *testing.T) {
	suite := []Etape{
		EtapeAcceptation, EtapeDesactivation, EtapeActivation,
		EtapeConfirmation, EtapeCompletion,
	}
	for i := 0; i < len(suite)-1; i++ {
		suivante, ok := EtapeSuivante(suite[i])
		require.True(t, ok, string(suite[i]))
		require.Equal(t, suite[i+1], suivante)
	}
	_, ok := EtapeSuivante(EtapeCompletion)
	require.False(t, ok, "COMPLETION est terminale")
}

func TestResponsableEtape(t *testing.T) {
	require.Equal(t, RoleSource, ResponsableEtape(EtapeAcceptation, TypePortage))
	require.Equal(t, RoleSource, ResponsableEtape(EtapeDesactivation, TypePortage))
	require.Equal(t, RoleDestinataire, ResponsableEtape(EtapeActivation, TypePortage))
	require.Equal(t, RoleTous, ResponsableEtape(EtapeConfirmation, TypePortage))
	require.Equal(t, RoleDestinataire, ResponsableEtape(EtapeCompletion, TypePortage))

	// La COMPLETION d'un REVERSE est réservée à l'ARTP.
	require.Equal(t, RoleARTP, ResponsableEtape(EtapeCompletion, TypeReverse))
	require.Equal(t, RoleDestinataire, ResponsableEtape(EtapeCompletion, TypeRestitution))
}

func TestPeutTraiterRefuseLEtapeQuiNIncombePas(t *testing.T) {
	d := portage()
	d.EtapeActuelle = EtapeDesactivation

	require.Nil(t, PeutTraiter(d, orange), "la source traite la DESACTIVATION")

	e := PeutTraiter(d, yas)
	require.NotNil(t, e, "le destinataire ne peut pas désactiver")
	require.Equal(t, "DEMANDE_ACCES_REFUSE", e.Code)
}

func TestPeutTraiterRefuseAcceptationEtConfirmation(t *testing.T) {
	d := portage()

	e := PeutTraiter(d, orange)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
	require.Equal(t,
		"L'étape ACCEPTATION se traite via POST /api/gateway/v1/demandes/acceptation.",
		e.Message)

	d.EtapeActuelle = EtapeConfirmation
	e = PeutTraiter(d, orange)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
	require.Equal(t,
		"L'étape CONFIRMATION se traite via POST /api/gateway/v1/demandes/a-confirmer.",
		e.Message)
}

func TestPeutTraiterRefuseCompletionReverse(t *testing.T) {
	d := portage()
	d.TypeDemande = TypeReverse
	d.EtapeActuelle = EtapeCompletion

	e := PeutTraiter(d, yas)
	require.NotNil(t, e)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", e.Code)
	require.Equal(t,
		"La complétion (COMPLETION) d'une demande REVERSE est réservée à l'ARTP, une fois que tous les opérateurs ont confirmé.",
		e.Message)
}

func TestPeutTraiterRefuseUneDemandeNonEnCours(t *testing.T) {
	d := portage()
	d.EtapeActuelle = EtapeDesactivation
	d.StatutDemande = StatutAnnule

	e := PeutTraiter(d, orange)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
}

func TestPeutTraiterRefusePendantLaConvergence(t *testing.T) {
	// R-10 : l'étape a été traitée, la transition n'est pas encore appliquée.
	d := portage()
	d.EtapeActuelle = EtapeDesactivation
	d.TransitionEnAttente = true

	e := PeutTraiter(d, orange)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
}

func TestConfirmateursPortageExcluentLeDestinataire(t *testing.T) {
	// D-6, mesuré au SIT : sur un portage ORANGE → YAS, EXPRESSO doit confirmer.
	d := portage()
	d.EtapeActuelle = EtapeConfirmation

	require.ElementsMatch(t, []string{orange, expresso}, ConfirmateursAttendus(d, place))
}

func TestConfirmateursRestitutionEtReverseIncluentTousLeMonde(t *testing.T) {
	for _, td := range []TypeDemande{TypeRestitution, TypeReverse} {
		d := portage()
		d.TypeDemande = td
		d.EtapeActuelle = EtapeConfirmation
		require.ElementsMatchf(t, place, ConfirmateursAttendus(d, place), string(td))
	}
}

func TestPeutAccepter(t *testing.T) {
	d := portage()

	require.Nil(t, PeutAccepter(d, orange))

	// TC-034 : le destinataire ne peut pas accepter sa propre demande.
	e := PeutAccepter(d, yas)
	require.NotNil(t, e)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", e.Code)

	// Un tiers non plus.
	require.NotNil(t, PeutAccepter(d, expresso))

	// Hors de l'étape ACCEPTATION.
	d2 := portage()
	d2.EtapeActuelle = EtapeActivation
	e = PeutAccepter(d2, orange)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
}

func TestPeutAnnuler(t *testing.T) {
	d := portage()

	require.Nil(t, PeutAnnuler(d, yas), "le créateur annule")

	e := PeutAnnuler(d, orange)
	require.NotNil(t, e)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", e.Code)
	require.Equal(t,
		"Seul l'opérateur ayant créé la demande (opérateur destinataire) peut l'annuler.",
		e.Message)

	d.EtapeActuelle = EtapeDesactivation
	e = PeutAnnuler(d, yas)
	require.NotNil(t, e)
	require.Equal(t, "ETAPE_INVALIDE", e.Code)
	require.Equal(t,
		"Cette demande ne peut plus être annulée (étape actuelle : DESACTIVATION).",
		e.Message)
}

func TestErreursSontDesAppErr(t *testing.T) {
	var e *apperr.Error = PeutAnnuler(portage(), orange)
	require.NotNil(t, e)
}
```

- [ ] **Step 2 : Lancer les tests, vérifier qu'ils échouent**

Run: `go test ./internal/domain/...`
Expected: FAIL — `undefined: Demande`.

- [ ] **Step 3 : Implémenter les types**

Create `internal/domain/demande.go` :

```go
// Package domain porte les règles de portabilité, sans I/O ni HTTP. Il ne connaît
// pas le mode de fidélité : une règle métier est la même dans les deux modes, seul
// son rendu change.
package domain

type Etape string

const (
	EtapeAcceptation   Etape = "ACCEPTATION"
	EtapeDesactivation Etape = "DESACTIVATION"
	EtapeActivation    Etape = "ACTIVATION"
	EtapeConfirmation  Etape = "CONFIRMATION"
	EtapeCompletion    Etape = "COMPLETION"
)

type StatutDemande string

const (
	StatutEnCours StatutDemande = "EN_COURS"
	StatutTermine StatutDemande = "TERMINE"
	StatutAnnule  StatutDemande = "ANNULE"
	// StatutRejete — [HYP] ni documenté au guide, ni observé en recette.
	StatutRejete StatutDemande = "REJETE"
)

type StatutEtape string

const (
	EtapeEnCours  StatutEtape = "EN_COURS"
	EtapeTerminee StatutEtape = "TERMINE"
	EtapeExpiree  StatutEtape = "EXPIRE"
	EtapeValidee  StatutEtape = "VALIDE"
)

type TypeDemande string

const (
	TypePortage     TypeDemande = "PORTAGE"
	TypeRestitution TypeDemande = "RESTITUTION"
	TypeReverse     TypeDemande = "REVERSE"
)

type TypeAbonne string

const (
	AbonneParticulier TypeAbonne = "PARTICULIER"
	AbonneEntreprise  TypeAbonne = "ENTREPRISE"
)

type Role int

const (
	RoleSource Role = iota
	RoleDestinataire
	RoleTous
	RoleARTP
)

type Demande struct {
	ID                      string
	Numero                  string
	TypeDemande             TypeDemande
	TypeAbonne              TypeAbonne
	StatutDemande           StatutDemande
	EtapeActuelle           Etape
	StatutEtapeActuel       StatutEtape
	OperateurSourceID       string
	OperateurDestinataireID string
	CreateurOperateurID     string
	// TransitionEnAttente vaut true entre le traitement d'une étape et sa
	// convergence effective (R-10).
	TransitionEnAttente bool
}
```

- [ ] **Step 4 : Implémenter les règles**

Create `internal/domain/etapes.go` :

```go
package domain

import (
	"fmt"

	"github.com/yas/numflex-sandbox/internal/apperr"
)

var sequence = []Etape{
	EtapeAcceptation, EtapeDesactivation, EtapeActivation,
	EtapeConfirmation, EtapeCompletion,
}

func EtapeSuivante(e Etape) (Etape, bool) {
	for i, x := range sequence {
		if x == e && i+1 < len(sequence) {
			return sequence[i+1], true
		}
	}
	return "", false
}

func ResponsableEtape(e Etape, td TypeDemande) Role {
	switch e {
	case EtapeAcceptation, EtapeDesactivation:
		return RoleSource
	case EtapeActivation:
		return RoleDestinataire
	case EtapeConfirmation:
		return RoleTous
	case EtapeCompletion:
		if td == TypeReverse {
			return RoleARTP
		}
		return RoleDestinataire
	}
	return RoleARTP
}

// EndpointEtape nomme l'endpoint qui traite une étape, pour construire le message
// d'ETAPE_INVALIDE tel que le guide le rédige (§7.10).
func EndpointEtape(e Etape) string {
	switch e {
	case EtapeAcceptation:
		return "POST /api/gateway/v1/demandes/acceptation"
	case EtapeConfirmation:
		return "POST /api/gateway/v1/demandes/a-confirmer"
	default:
		return "POST /api/gateway/v1/demandes/traitement"
	}
}

// PeutTraiter décide si un opérateur peut appeler /demandes/traitement maintenant.
func PeutTraiter(d Demande, operateurID string) *apperr.Error {
	if d.StatutDemande != StatutEnCours {
		return apperr.EtapeInvalide(fmt.Sprintf(
			"Cette demande n'est plus en cours (statut : %s).", d.StatutDemande))
	}
	if d.TransitionEnAttente {
		return apperr.EtapeInvalide(fmt.Sprintf(
			"L'étape %s a déjà été traitée pour cette demande.", d.EtapeActuelle))
	}

	switch d.EtapeActuelle {
	case EtapeAcceptation, EtapeConfirmation:
		return apperr.EtapeInvalide(fmt.Sprintf("L'étape %s se traite via %s.",
			d.EtapeActuelle, EndpointEtape(d.EtapeActuelle)))
	}

	switch ResponsableEtape(d.EtapeActuelle, d.TypeDemande) {
	case RoleARTP:
		return apperr.DemandeAccesRefuse(
			"La complétion (COMPLETION) d'une demande REVERSE est réservée à l'ARTP, une fois que tous les opérateurs ont confirmé.")
	case RoleSource:
		if operateurID != d.OperateurSourceID {
			return apperr.DemandeAccesRefuse(fmt.Sprintf(
				"L'étape %s incombe à l'opérateur source.", d.EtapeActuelle))
		}
	case RoleDestinataire:
		if operateurID != d.OperateurDestinataireID {
			return apperr.DemandeAccesRefuse(fmt.Sprintf(
				"L'étape %s incombe à l'opérateur destinataire.", d.EtapeActuelle))
		}
	}
	return nil
}

// ConfirmateursAttendus liste les opérateurs dont la confirmation est requise.
// PORTAGE : tous les opérateurs de la place sauf le destinataire, qui est
// auto-confirmé une fois les autres validés — vérifié par mesure au SIT, un
// opérateur tiers ni source ni destinataire devant confirmer pour solder l'étape.
// RESTITUTION et REVERSE : tout le monde, destinataire compris.
func ConfirmateursAttendus(d Demande, tousOperateurs []string) []string {
	out := make([]string, 0, len(tousOperateurs))
	for _, op := range tousOperateurs {
		if d.TypeDemande == TypePortage && op == d.OperateurDestinataireID {
			continue
		}
		out = append(out, op)
	}
	return out
}

func PeutAccepter(d Demande, operateurID string) *apperr.Error {
	if d.StatutDemande != StatutEnCours {
		return apperr.EtapeInvalide(fmt.Sprintf(
			"Cette demande n'est plus en cours (statut : %s).", d.StatutDemande))
	}
	if d.EtapeActuelle != EtapeAcceptation || d.TransitionEnAttente {
		return apperr.EtapeInvalide(fmt.Sprintf(
			"Cette demande n'est plus à l'étape ACCEPTATION (étape actuelle : %s).",
			d.EtapeActuelle))
	}
	if operateurID != d.OperateurSourceID {
		return apperr.DemandeAccesRefuse(
			"Seul l'opérateur source peut accepter ou rejeter cette demande.")
	}
	return nil
}

func PeutAnnuler(d Demande, operateurID string) *apperr.Error {
	if operateurID != d.CreateurOperateurID {
		return apperr.DemandeAccesRefuse(
			"Seul l'opérateur ayant créé la demande (opérateur destinataire) peut l'annuler.")
	}
	if d.StatutDemande != StatutEnCours || d.EtapeActuelle != EtapeAcceptation {
		return apperr.EtapeInvalide(fmt.Sprintf(
			"Cette demande ne peut plus être annulée (étape actuelle : %s).", d.EtapeActuelle))
	}
	return nil
}
```

- [ ] **Step 5 : Lancer les tests, vérifier qu'ils passent**

Run: `go test ./internal/domain/... -v`
Expected: PASS — les onze tests.

- [ ] **Step 6 : Commit**

```bash
git add -A
git commit -m "feat: machine a etats de portabilite et regles d habilitation"
```

---

## Task 9 : Moteur — expiration, convergence, actes de l'ARTP

**Files:**
- Create: `internal/engine/engine.go`, `internal/engine/transitions.go`
- Create: `internal/engine/engine_test.go`
- Modify: `cmd/server/main.go` (démarrer le moteur)

**Interfaces:**
- Consumes: `config.Config`, `store.DB`, `domain`.
- Produces:
  - `engine.Engine` construit par `engine.New(cfg *config.Config, db *store.DB) *Engine`
  - `(*Engine).Run(ctx context.Context)` — boucle bloquante, s'arrête sur annulation du contexte.
  - `(*Engine).Tick(ctx context.Context) error` — un passage, exporté pour les tests.
  - `(*Engine).PlanifierTransition(ctx context.Context, demandeID string) error` — appelée par les handlers après traitement d'une étape ; écrit `transition_prevue_a = now + délai tiré dans [min, max]`.
  - `(*Engine).AppliquerTransition(ctx context.Context, demandeID string, origine string) error` — avance d'une étape ; `origine` vaut `"ACTION"` ou `"EXPIRATION"`.
  - `(*Engine).PlaceGelee(ctx context.Context) (bool, error)` — vrai si un incident interne est ouvert.

- [ ] **Step 1 : Écrire les tests du moteur**

Create `internal/engine/engine_test.go` :

```go
package engine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/config"
	"github.com/yas/numflex-sandbox/internal/domain"
	"github.com/yas/numflex-sandbox/internal/seed"
	"github.com/yas/numflex-sandbox/internal/store"
)

// insererDemande crée une demande directement en base, à l'étape voulue.
func insererDemande(t *testing.T, db *store.DB, id string, etape domain.Etape, ageEtape time.Duration) {
	t.Helper()
	debut := time.Now().Add(-ageEtape)
	_, err := db.Pool.Exec(context.Background(),
		`INSERT INTO demande
		   (id, numero, type_abonne, type_demande, statut_demande, etape_actuelle,
		    statut_etape_actuel, operateur_source_id, operateur_destinataire_id,
		    createur_operateur_id, processus, routage_info, date_demande, date_debut_etape)
		 VALUES ($1,'771000001','PARTICULIER','PORTAGE','EN_COURS',$2,'EN_COURS',
		         $3,$4,$4,'PREPAID','191',now(),$5)`,
		id, string(etape), seed.OperateurOrange, seed.OperateurYAS, debut)
	require.NoError(t, err)

	_, err = db.Pool.Exec(context.Background(),
		`INSERT INTO demande_numero (demande_id, numero, statut) VALUES ($1,'771000001','EN_COURS')`, id)
	require.NoError(t, err)
}

func etatDemande(t *testing.T, db *store.DB, id string) (etape, statutEtape, statutDemande string) {
	t.Helper()
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT etape_actuelle, statut_etape_actuel, statut_demande FROM demande WHERE id = $1`, id).
		Scan(&etape, &statutEtape, &statutDemande))
	return
}

func moteur(t *testing.T, ajuste ...func(*config.Config)) (*Engine, *store.DB) {
	t.Helper()
	db := store.NewTestDB(t)
	cfg := &config.Config{EngineTick: time.Millisecond, EtapeTimeout: 0}
	for _, f := range ajuste {
		f(cfg)
	}
	return New(cfg, db), db
}

func TestExpirationFaitAvancerSansAucunAppel(t *testing.T) {
	// TC-062 / ANO-006 : les étapes progressent seules.
	e, db := moteur(t, func(c *config.Config) { c.EtapeTimeout = 2 * time.Second })
	insererDemande(t, db, "d1", domain.EtapeAcceptation, 3*time.Second)

	require.NoError(t, e.Tick(context.Background()))

	etape, statutEtape, statutDemande := etatDemande(t, db, "d1")
	require.Equal(t, "DESACTIVATION", etape)
	require.Equal(t, "EN_COURS", statutEtape)
	require.Equal(t, "EN_COURS", statutDemande)

	// L'historique conserve la trace de l'expiration.
	var origine, statut string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT origine, statut FROM etape_historique
		  WHERE demande_id = 'd1' AND etape = 'ACCEPTATION'`).Scan(&origine, &statut))
	require.Equal(t, "EXPIRATION", origine)
	require.Equal(t, "EXPIRE", statut)
}

func TestExpirationNAvancePasAvantLeDelai(t *testing.T) {
	e, db := moteur(t, func(c *config.Config) { c.EtapeTimeout = time.Hour })
	insererDemande(t, db, "d1", domain.EtapeAcceptation, time.Minute)

	require.NoError(t, e.Tick(context.Background()))

	etape, _, _ := etatDemande(t, db, "d1")
	require.Equal(t, "ACCEPTATION", etape)
}

func TestExpirationDesactiveeQuandLeDelaiEstNul(t *testing.T) {
	e, db := moteur(t) // EtapeTimeout = 0
	insererDemande(t, db, "d1", domain.EtapeAcceptation, 48*time.Hour)

	require.NoError(t, e.Tick(context.Background()))

	etape, _, _ := etatDemande(t, db, "d1")
	require.Equal(t, "ACCEPTATION", etape)
}

func TestCycleCompletParExpiration(t *testing.T) {
	// Le portage n°2 du SIT : créé, aucun appel, TERMINE 29 minutes plus tard.
	e, db := moteur(t, func(c *config.Config) { c.EtapeTimeout = time.Nanosecond })
	insererDemande(t, db, "d1", domain.EtapeAcceptation, time.Second)

	for i := 0; i < 5; i++ {
		require.NoError(t, e.Tick(context.Background()))
	}

	etape, statutEtape, statutDemande := etatDemande(t, db, "d1")
	require.Equal(t, "COMPLETION", etape)
	require.Equal(t, "EXPIRE", statutEtape)
	require.Equal(t, "TERMINE", statutDemande)

	var finalisation *time.Time
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT date_finalisation FROM demande WHERE id = 'd1'`).Scan(&finalisation))
	require.NotNil(t, finalisation)

	// Le numéro a réellement changé d'opérateur au registre national.
	var actuel string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT operateur_actuel_id FROM numero WHERE msisdn = '771000001'`).Scan(&actuel))
	require.Equal(t, seed.OperateurYAS, actuel)
}

func TestPlanifierPuisAppliquerLaTransition(t *testing.T) {
	e, db := moteur(t) // convergence nulle : transition immédiatement due
	insererDemande(t, db, "d1", domain.EtapeDesactivation, time.Second)

	require.NoError(t, e.PlanifierTransition(context.Background(), "d1"))

	// Tant que le moteur n'a pas tourné, l'étape reste la précédente (R-10).
	etape, _, _ := etatDemande(t, db, "d1")
	require.Equal(t, "DESACTIVATION", etape)

	require.NoError(t, e.Tick(context.Background()))

	etape, _, _ = etatDemande(t, db, "d1")
	require.Equal(t, "ACTIVATION", etape)

	var origine string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT origine FROM etape_historique
		  WHERE demande_id = 'd1' AND etape = 'DESACTIVATION'`).Scan(&origine))
	require.Equal(t, "ACTION", origine)
}

func TestConvergenceRespecteLeDelai(t *testing.T) {
	e, db := moteur(t, func(c *config.Config) {
		c.ConvergenceMin = time.Hour
		c.ConvergenceMax = time.Hour
	})
	insererDemande(t, db, "d1", domain.EtapeDesactivation, time.Second)

	require.NoError(t, e.PlanifierTransition(context.Background(), "d1"))
	require.NoError(t, e.Tick(context.Background()))

	etape, _, _ := etatDemande(t, db, "d1")
	require.Equal(t, "DESACTIVATION", etape, "la transition n'est pas encore due")
}

func TestRoutageRecalculeAuPassageEnConfirmation(t *testing.T) {
	e, db := moteur(t)
	insererDemande(t, db, "d1", domain.EtapeActivation, time.Second)

	require.NoError(t, e.PlanifierTransition(context.Background(), "d1"))
	require.NoError(t, e.Tick(context.Background()))

	var routage string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT routage_info FROM demande WHERE id = 'd1'`).Scan(&routage))
	require.Equal(t, "192", routage, "routage destinataire (YAS) pour un numéro porté")
}

func TestPlaceGeleeSuspendLeMoteur(t *testing.T) {
	// BR-012 : un incident interne gèle le traitement pour tous.
	e, db := moteur(t, func(c *config.Config) { c.EtapeTimeout = time.Nanosecond })
	insererDemande(t, db, "d1", domain.EtapeAcceptation, time.Second)

	_, err := db.Pool.Exec(context.Background(),
		`INSERT INTO incident (id, operateur_id, type_incident_id, fige_systeme,
		                       description, statut, date_ouverture)
		 VALUES ('i1',$1,$2,true,'panne','EN_COURS',now())`,
		seed.OperateurExpresso, seed.TypeIncidentTechnique)
	require.NoError(t, err)

	gelee, err := e.PlaceGelee(context.Background())
	require.NoError(t, err)
	require.True(t, gelee)

	require.NoError(t, e.Tick(context.Background()))

	etape, _, _ := etatDemande(t, db, "d1")
	require.Equal(t, "ACCEPTATION", etape, "le moteur ne doit rien avancer pendant le gel")
}

func TestIncidentGatewayNeGelePas(t *testing.T) {
	e, db := moteur(t)
	_, err := db.Pool.Exec(context.Background(),
		`INSERT INTO incident (id, operateur_id, type_incident_id, fige_systeme,
		                       description, statut, date_ouverture)
		 VALUES ('i1',$1,$2,false,'timeout','EN_COURS',now())`,
		seed.OperateurYAS, seed.TypeIncidentGateway)
	require.NoError(t, err)

	gelee, err := e.PlaceGelee(context.Background())
	require.NoError(t, err)
	require.False(t, gelee)
}
```

- [ ] **Step 2 : Lancer les tests, vérifier qu'ils échouent**

Run: `make test`
Expected: FAIL — `undefined: New`.

- [ ] **Step 3 : Implémenter la boucle et le gel**

Create `internal/engine/engine.go` :

```go
// Package engine reproduit ce que la plateforme NumFlex fait sans qu'aucun
// opérateur n'agisse : l'expiration des étapes (ANO-006), la convergence différée
// après traitement (R-10), et les actes réservés à l'ARTP.
package engine

import (
	"context"
	"log"
	"math/rand"
	"time"

	"github.com/yas/numflex-sandbox/internal/config"
	"github.com/yas/numflex-sandbox/internal/store"
)

type Engine struct {
	cfg *config.Config
	db  *store.DB
}

func New(cfg *config.Config, db *store.DB) *Engine {
	return &Engine{cfg: cfg, db: db}
}

func (e *Engine) Run(ctx context.Context) {
	t := time.NewTicker(e.cfg.EngineTick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := e.Tick(ctx); err != nil {
				log.Printf("moteur : %v", err)
			}
		}
	}
}

// Tick effectue un passage : convergences dues, expirations, actes de l'ARTP.
func (e *Engine) Tick(ctx context.Context) error {
	gelee, err := e.PlaceGelee(ctx)
	if err != nil {
		return err
	}
	if gelee {
		return nil
	}
	if err := e.appliquerConvergencesDues(ctx); err != nil {
		return err
	}
	if err := e.expirerEtapes(ctx); err != nil {
		return err
	}
	if err := e.validerReversesAutomatiquement(ctx); err != nil {
		return err
	}
	return e.completerReversesConfirmes(ctx)
}

// PlaceGelee : un incident de type figeSysteme ouvert, chez n'importe quel
// opérateur, bloque le traitement pour tout le monde (BR-012).
func (e *Engine) PlaceGelee(ctx context.Context) (bool, error) {
	var n int
	err := e.db.Pool.QueryRow(ctx,
		`SELECT count(*) FROM incident i
		   JOIN type_incident t ON t.id = i.type_incident_id
		  WHERE i.statut = 'EN_COURS' AND t.fige_systeme`).Scan(&n)
	return n > 0, err
}

// PlanifierTransition marque l'étape courante comme traitée et fixe la date à
// laquelle la transition sera réellement appliquée. Entre les deux, la demande
// continue de présenter l'étape précédente — c'est le comportement mesuré (R-10).
func (e *Engine) PlanifierTransition(ctx context.Context, demandeID string) error {
	delai := e.cfg.ConvergenceMin
	if ecart := e.cfg.ConvergenceMax - e.cfg.ConvergenceMin; ecart > 0 {
		delai += time.Duration(rand.Int63n(int64(ecart)))
	}
	_, err := e.db.Pool.Exec(ctx,
		`UPDATE demande SET transition_prevue_a = $2 WHERE id = $1`,
		demandeID, time.Now().Add(delai))
	return err
}

func (e *Engine) appliquerConvergencesDues(ctx context.Context) error {
	rows, err := e.db.Pool.Query(ctx,
		`SELECT id FROM demande
		  WHERE statut_demande = 'EN_COURS'
		    AND transition_prevue_a IS NOT NULL
		    AND transition_prevue_a <= now()`)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()

	for _, id := range ids {
		if err := e.AppliquerTransition(ctx, id, "ACTION"); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) expirerEtapes(ctx context.Context) error {
	if e.cfg.EtapeTimeout <= 0 {
		return nil
	}
	rows, err := e.db.Pool.Query(ctx,
		`SELECT id FROM demande
		  WHERE statut_demande = 'EN_COURS'
		    AND transition_prevue_a IS NULL
		    AND date_debut_etape + make_interval(secs => $1) <= now()`,
		e.cfg.EtapeTimeout.Seconds())
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()

	for _, id := range ids {
		if err := e.AppliquerTransition(ctx, id, "EXPIRATION"); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4 : Implémenter la transition et ses effets**

Create `internal/engine/transitions.go` :

```go
package engine

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/yas/numflex-sandbox/internal/domain"
)

// AppliquerTransition solde l'étape courante et fait passer la demande à la
// suivante. origine vaut "ACTION" (traitement nominal) ou "EXPIRATION".
func (e *Engine) AppliquerTransition(ctx context.Context, demandeID, origine string) error {
	tx, err := e.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var d domain.Demande
	var etape, statutDem, typeDem string
	err = tx.QueryRow(ctx,
		`SELECT etape_actuelle, statut_demande, type_demande,
		        operateur_source_id, operateur_destinataire_id, numero
		   FROM demande WHERE id = $1 FOR UPDATE`, demandeID).
		Scan(&etape, &statutDem, &typeDem, &d.OperateurSourceID,
			&d.OperateurDestinataireID, &d.Numero)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return err
	}
	if domain.StatutDemande(statutDem) != domain.StatutEnCours {
		return nil
	}

	courante := domain.Etape(etape)
	typeDemande := domain.TypeDemande(typeDem)
	maintenant := time.Now()

	statutEtapeSoldee := string(domain.EtapeTerminee)
	if origine == "EXPIRATION" {
		statutEtapeSoldee = string(domain.EtapeExpiree)
	} else if courante == domain.EtapeCompletion {
		statutEtapeSoldee = string(domain.EtapeValidee)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO etape_historique (demande_id, etape, statut, origine, date_debut, date_fin)
		 SELECT id, etape_actuelle, $2, $3, date_debut_etape, $4 FROM demande WHERE id = $1`,
		demandeID, statutEtapeSoldee, origine, maintenant); err != nil {
		return err
	}

	suivante, existe := domain.EtapeSuivante(courante)
	if !existe {
		// COMPLETION soldée : la demande se termine.
		if err := e.effetsFinDeDemande(ctx, tx, demandeID, typeDemande, d); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE demande
			    SET statut_demande = 'TERMINE', statut_etape_actuel = $2,
			        date_finalisation = $3, transition_prevue_a = NULL
			  WHERE id = $1`, demandeID, statutEtapeSoldee, maintenant); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	// Effets de bord attachés à la sortie de l'étape.
	if courante == domain.EtapeActivation && typeDemande == domain.TypePortage {
		if err := e.transfererAuRegistre(ctx, tx, demandeID, d); err != nil {
			return err
		}
		if err := e.recalculerRoutage(ctx, tx, demandeID, d); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE demande
		    SET etape_actuelle = $2, statut_etape_actuel = 'EN_COURS',
		        date_debut_etape = $3, transition_prevue_a = NULL
		  WHERE id = $1`, demandeID, string(suivante), maintenant); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// transfererAuRegistre inscrit le changement d'opérateur au registre national.
// C'est le constat central du SIT : quand une étape expire, ce transfert a lieu
// alors qu'aucun HLR n'a été touché.
func (e *Engine) transfererAuRegistre(ctx context.Context, tx pgx.Tx, demandeID string, d domain.Demande) error {
	_, err := tx.Exec(ctx,
		`UPDATE numero SET operateur_actuel_id = $2, date_dernier_portage = now()
		  WHERE msisdn IN (SELECT numero FROM demande_numero
		                    WHERE demande_id = $1 AND NOT exclu AND statut <> 'REJETE')`,
		demandeID, d.OperateurDestinataireID)
	return err
}

// recalculerRoutage finalise le routage numéro par numéro (§7.10) : préfixe du
// destinataire pour les numéros portés, de la source pour les numéros rejetés.
func (e *Engine) recalculerRoutage(ctx context.Context, tx pgx.Tx, demandeID string, d domain.Demande) error {
	var prefixeDest, prefixeSource string
	if err := tx.QueryRow(ctx, `SELECT prefixe_routage FROM operateur WHERE id = $1`,
		d.OperateurDestinataireID).Scan(&prefixeDest); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `SELECT prefixe_routage FROM operateur WHERE id = $1`,
		d.OperateurSourceID).Scan(&prefixeSource); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE demande_numero
		    SET routage_info = CASE WHEN statut = 'REJETE' OR exclu THEN $2 ELSE $3 END
		  WHERE demande_id = $1`, demandeID, prefixeSource, prefixeDest); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE demande SET routage_info = $2 WHERE id = $1`,
		demandeID, prefixeDest)
	return err
}

// effetsFinDeDemande : pour une RESTITUTION ou un REVERSE, le numéro rejoint son
// opérateur d'origine et routageInfo n'apparaît qu'ici (§7.10).
func (e *Engine) effetsFinDeDemande(ctx context.Context, tx pgx.Tx, demandeID string,
	td domain.TypeDemande, d domain.Demande) error {

	if td == domain.TypePortage {
		return nil
	}
	if _, err := tx.Exec(ctx,
		`UPDATE numero
		    SET operateur_actuel_id = $2, date_dernier_portage = now(), deja_restitue = true
		  WHERE msisdn = $1`, d.Numero, d.OperateurDestinataireID); err != nil {
		return err
	}
	var prefixe string
	if err := tx.QueryRow(ctx, `SELECT prefixe_routage FROM operateur WHERE id = $1`,
		d.OperateurDestinataireID).Scan(&prefixe); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE demande SET routage_info = $2 WHERE id = $1`, demandeID, prefixe)
	return err
}
```

`validerReversesAutomatiquement` et `completerReversesConfirmes` sont implémentées en Task 19 ; à cette étape, les déclarer avec un corps qui retourne `nil`.

- [ ] **Step 5 : Lancer les tests, vérifier qu'ils passent**

Run: `make test`
Expected: PASS — les neuf tests du moteur.

- [ ] **Step 6 : Brancher le moteur sur le serveur et sur le harnais de test**

Modify `cmd/server/main.go` — construire `engine.New(cfg, db)`, le lancer par `go moteur.Run(ctx)` avec un contexte annulé à l'arrêt, et le renseigner dans `Deps.Moteur`.

Modify `internal/api/testutil_test.go` — `*engine.Engine` satisfait l'interface `api.Moteur` déclarée en Task 5. Le harnais le construit et expose de quoi déclencher une convergence dans les tests, sans dépendre du ticker :

```go
	mot := engine.New(cfg, db)
	d := &Deps{
		Cfg: cfg, DB: db,
		R:      httpx.NewRenderer(cfg.Fidelity, cfg.ClockSkew),
		Moteur: mot,
	}
	h := &harnais{t: t, srv: srv, cfg: cfg, db: db, moteur: mot}
```

Ajouter le champ `moteur *engine.Engine` à la structure `harnais`, ainsi que :

```go
// converger déclenche un passage du moteur et vérifie qu'aucune transition ne
// reste due. Les tests pilotent le moteur explicitement plutôt que d'attendre
// son ticker.
func (h *harnais) converger() {
	h.t.Helper()
	require.NoError(h.t, h.moteur.Tick(context.Background()))
}

func (h *harnais) etape(id string) string {
	h.t.Helper()
	var e string
	require.NoError(h.t, h.db.Pool.QueryRow(context.Background(),
		"SELECT etape_actuelle FROM demande WHERE id = $1", id).Scan(&e))
	return e
}

func (h *harnais) statutDemande(id string) string {
	h.t.Helper()
	var s string
	require.NoError(h.t, h.db.Pool.QueryRow(context.Background(),
		"SELECT statut_demande FROM demande WHERE id = $1", id).Scan(&s))
	return s
}
```

Pour les tests du profil déterministe (`ConvergenceMin` et `ConvergenceMax` nuls), un seul `h.converger()` après chaque action suffit à appliquer la transition.

- [ ] **Step 7 : Commit**

```bash
git add -A
git commit -m "feat: moteur d expiration, de convergence differee et de transitions"
```

---

## Task 10 : Éligibilité et création d'une demande particulier

**Files:**
- Create: `internal/domain/eligibilite.go`, `internal/domain/eligibilite_test.go`
- Modify: `internal/api/demandes_creation.go` (remplace le stub créé en Task 5)
- Create: `internal/api/creation_particulier_test.go`, `internal/api/dto.go`

**Interfaces:**
- Consumes: `domain`, `Deps`, `verifierOTP`, `oid`.
- Produces:
  - `domain.EtatNumero{MSISDN, OperateurActuelID, OperateurOrigineID string; DateDernierPortage *time.Time; DejaRestitue bool; DemandeEnCours bool}`
  - `domain.VerifierEligibilitePortage(n EtatNumero, sourceID, destinataireID string, delaiPortage time.Duration) *apperr.Error`
  - `domain.VerifierEligibiliteRestitution(n EtatNumero, delaiRestitution time.Duration) *apperr.Error`
  - Constantes `domain.DelaiEntrePortages = 3 * 30 * 24 * time.Hour` et `domain.DelaiAvantRestitution = 6 * 30 * 24 * time.Hour`.
  - `(*Deps).routesCreation(g *gin.RouterGroup)` — à cette tâche, seule `/demandes/particulier` est câblée.
  - `(*Deps).demandeDTO(ctx context.Context, id string) (map[string]any, error)` — sérialisation commune à tous les endpoints qui renvoient une demande.

- [ ] **Step 1 : Écrire les tests d'éligibilité**

Create `internal/domain/eligibilite_test.go` :

```go
package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func ilYA(jours int) *time.Time {
	t := time.Now().AddDate(0, 0, -jours)
	return &t
}

func TestPortageNominal(t *testing.T) {
	n := EtatNumero{MSISDN: "771000001", OperateurActuelID: orange, OperateurOrigineID: orange}
	require.Nil(t, VerifierEligibilitePortage(n, orange, yas, DelaiEntrePortages))
}

func TestPortageOperateurSourceIncorrect(t *testing.T) {
	n := EtatNumero{MSISDN: "771000001", OperateurActuelID: orange, OperateurOrigineID: orange}
	e := VerifierEligibilitePortage(n, expresso, yas, DelaiEntrePortages)
	require.NotNil(t, e)
	require.Equal(t, "OPERATEUR_SOURCE_INCORRECT", e.Code)
}

func TestPortageNumeroDejaChezDestinataire(t *testing.T) {
	n := EtatNumero{MSISDN: "761000001", OperateurActuelID: yas, OperateurOrigineID: yas}
	e := VerifierEligibilitePortage(n, yas, yas, DelaiEntrePortages)
	require.NotNil(t, e)
	require.Equal(t, "NUMERO_DEJA_CHEZ_DESTINATAIRE", e.Code)
}

func TestPortageDemandeDejaEnCours(t *testing.T) {
	n := EtatNumero{MSISDN: "771000001", OperateurActuelID: orange,
		OperateurOrigineID: orange, DemandeEnCours: true}
	e := VerifierEligibilitePortage(n, orange, yas, DelaiEntrePortages)
	require.NotNil(t, e)
	require.Equal(t, "DEMANDE_EN_COURS_POUR_NUMERO", e.Code)
}

func TestPortageDelaiNonRespecte(t *testing.T) {
	n := EtatNumero{MSISDN: "772000001", OperateurActuelID: orange,
		OperateurOrigineID: yas, DateDernierPortage: ilYA(30)}
	e := VerifierEligibilitePortage(n, orange, yas, DelaiEntrePortages)
	require.NotNil(t, e)
	require.Equal(t, "DELAI_PORTAGE_NON_RESPECTE", e.Code)
	// ANO-002 : ce refus se présente comme une panne serveur.
	require.Equal(t, "Unexpected runtime exception", e.RealDetail)
}

func TestPortageDelaiRespecte(t *testing.T) {
	n := EtatNumero{MSISDN: "773000001", OperateurActuelID: yas,
		OperateurOrigineID: orange, DateDernierPortage: ilYA(240)}
	require.Nil(t, VerifierEligibilitePortage(n, yas, expresso, DelaiEntrePortages))
}

func TestRestitutionNumeroNonPorte(t *testing.T) {
	n := EtatNumero{MSISDN: "771000001", OperateurActuelID: orange, OperateurOrigineID: orange}
	e := VerifierEligibiliteRestitution(n, DelaiAvantRestitution)
	require.NotNil(t, e)
	require.Equal(t, "NUMERO_NON_PORTE", e.Code)
}

func TestRestitutionDejaRestituee(t *testing.T) {
	n := EtatNumero{MSISDN: "775000001", OperateurActuelID: yas, OperateurOrigineID: orange,
		DateDernierPortage: ilYA(240), DejaRestitue: true}
	e := VerifierEligibiliteRestitution(n, DelaiAvantRestitution)
	require.NotNil(t, e)
	require.Equal(t, "NUMERO_DEJA_RESTITUE", e.Code)
}

func TestRestitutionTropTot(t *testing.T) {
	n := EtatNumero{MSISDN: "774000001", OperateurActuelID: yas, OperateurOrigineID: orange,
		DateDernierPortage: ilYA(60)}
	e := VerifierEligibiliteRestitution(n, DelaiAvantRestitution)
	require.NotNil(t, e)
	require.Equal(t, "DELAI_RESTITUTION_NON_RESPECTE", e.Code)
	// ANO-020 : le code exploitable est enterré dans une chaîne.
	require.Contains(t, e.RealDetail, "error.numeroRestitutionTooEarly")
}

func TestRestitutionNominale(t *testing.T) {
	n := EtatNumero{MSISDN: "773000001", OperateurActuelID: yas, OperateurOrigineID: orange,
		DateDernierPortage: ilYA(240)}
	require.Nil(t, VerifierEligibiliteRestitution(n, DelaiAvantRestitution))
}
```

- [ ] **Step 2 : Lancer les tests, vérifier qu'ils échouent**

Run: `go test ./internal/domain/...`
Expected: FAIL — `undefined: EtatNumero`.

- [ ] **Step 3 : Implémenter l'éligibilité**

Create `internal/domain/eligibilite.go` :

```go
package domain

import (
	"time"

	"github.com/yas/numflex-sandbox/internal/apperr"
)

const (
	// Délai entre deux portages — motif de rejet ARTP « Dernier portage inférieur à 3 mois ».
	DelaiEntrePortages = 3 * 30 * 24 * time.Hour
	// Délai avant restitution — §7.5 du guide.
	DelaiAvantRestitution = 6 * 30 * 24 * time.Hour
)

type EtatNumero struct {
	MSISDN             string
	OperateurActuelID  string
	OperateurOrigineID string
	DateDernierPortage *time.Time
	DejaRestitue       bool
	DemandeEnCours     bool
}

func VerifierEligibilitePortage(n EtatNumero, sourceID, destinataireID string,
	delaiPortage time.Duration) *apperr.Error {

	if n.OperateurActuelID == destinataireID {
		return apperr.NumeroDejaChezDestinataire()
	}
	if n.OperateurActuelID != sourceID {
		return apperr.OperateurSourceIncorrect()
	}
	if n.DemandeEnCours {
		return apperr.DemandeEnCoursPourNumero()
	}
	if n.DateDernierPortage != nil && time.Since(*n.DateDernierPortage) < delaiPortage {
		return apperr.DelaiPortageNonRespecte()
	}
	return nil
}

func VerifierEligibiliteRestitution(n EtatNumero, delaiRestitution time.Duration) *apperr.Error {
	if n.DateDernierPortage == nil || n.OperateurActuelID == n.OperateurOrigineID {
		return apperr.NumeroNonPorte()
	}
	if n.DejaRestitue {
		return apperr.NumeroDejaRestitue()
	}
	if time.Since(*n.DateDernierPortage) < delaiRestitution {
		return apperr.DelaiRestitutionNonRespecte()
	}
	if n.DemandeEnCours {
		return apperr.DemandeEnCoursPourNumero()
	}
	return nil
}
```

- [ ] **Step 4 : Écrire les tests de création particulier**

Create `internal/api/creation_particulier_test.go` :

```go
package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/seed"
)

func corpsParticulier(numero string) map[string]any {
	return map[string]any{
		"numero":                  numero,
		"otpCode":                 "123456",
		"operateurSourceId":       seed.OperateurOrange,
		"operateurDestinataireId": seed.OperateurYAS,
		"typePortabilite":         "PREPAID",
		"client": map[string]any{
			"nom": "Diallo", "prenom": "Mamadou",
			"dateNaissance": "1975-03-20", "lieuNaissance": "Dakar",
			"typePiece": "CNI", "numeroPiece": "1234567890123",
		},
	}
}

// creerPortage envoie l'OTP puis crée une demande particulier ORANGE → YAS.
func (h *harnais) creerPortage(numero string) string {
	h.t.Helper()
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton, map[string]any{"numero": numero})

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		jeton, corpsParticulier(numero))
	require.Equal(h.t, http.StatusCreated, rep.StatusCode, corps)

	data := corps["data"].(map[string]any)
	return data["id"].(string)
}

func TestCreationParticulierNominale(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		jeton, corpsParticulier("771000001"))

	require.Equal(t, http.StatusCreated, rep.StatusCode)
	require.Equal(t, "Demande créée avec succès", corps["message"])

	data := corps["data"].(map[string]any)
	require.Regexp(t, `^[0-9a-f]{24}$`, data["id"])
	require.Equal(t, "771000001", data["numero"])
	require.Equal(t, "PARTICULIER", data["typeAbonne"])
	require.Equal(t, "PORTAGE", data["typeDemande"])
	require.Equal(t, "EN_COURS", data["statutDemande"])
	require.Equal(t, "ACCEPTATION", data["etapeActuelle"])
	require.Equal(t, "PREPAID", data["processus"])
	require.Equal(t, "191", data["routageInfo"], "routage initial de la source")

	src := data["operateurSource"].(map[string]any)
	require.Equal(t, seed.OperateurOrange, src["id"])
	require.Equal(t, "ORANGE", src["nom"])
	dst := data["operateurDestinataire"].(map[string]any)
	require.Equal(t, "YAS", dst["nom"])
}

func TestCreationParticulierLieuNaissanceObligatoire(t *testing.T) {
	// TC-050 / ANO-010 : le guide le documente facultatif, la plateforme le rejette.
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000002"})

	c := corpsParticulier("771000002")
	delete(c["client"].(map[string]any), "lieuNaissance")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/particulier", jeton, c)

	require.Equal(t, http.StatusBadRequest, rep.StatusCode)
	champs := corps["fieldErrors"].([]any)
	require.Len(t, champs, 1)
	require.Equal(t, "client.lieuNaissance", champs[0].(map[string]any)["field"])
}

func TestCreationParticulierDoitEtreLeDestinataire(t *testing.T) {
	h := nouveauHarnais(t)
	jetonOrange := h.jeton("orange", "orange2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jetonOrange,
		map[string]any{"numero": "771000003"})

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		jetonOrange, corpsParticulier("771000003"))

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestCreationParticulierDelaiPortageSePresenteCommeUnePanne(t *testing.T) {
	// ANO-002.
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "772000001"})

	c := corpsParticulier("772000001")
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/particulier", jeton, c)

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "Unexpected runtime exception", corps["detail"])
	require.NotContains(t, corps, "code")
}

func TestCreationParticulierSansOTP(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		h.jeton("yas", "yas2026"), corpsParticulier("771000004"))

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "Aucun OTP actif pour ce numéro", corps["detail"])
}

func TestCreationParticulierConsommeLOTP(t *testing.T) {
	h := nouveauHarnais(t)
	h.creerPortage("771000005")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/otp/verify",
		h.jeton("yas", "yas2026"),
		map[string]any{"numero": "771000005", "otpCode": "123456"})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "RuntimeException: Code déjà utilisé", corps["detail"])
}

func TestCreationParticulierDemandeDejaEnCours(t *testing.T) {
	h := nouveauHarnais(t)
	h.creerPortage("771000006")

	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000006"})
	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		jeton, corpsParticulier("771000006"))

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestCreationParticulierEnModeContratRendUnCodeMetier(t *testing.T) {
	h := nouveauHarnais(t, func(c *config.Config) { c.Fidelity = config.FidelityContract })
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "772000002"})

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/particulier",
		jeton, corpsParticulier("772000002"))

	require.Equal(t, http.StatusConflict, rep.StatusCode)
	require.Equal(t, "DELAI_PORTAGE_NON_RESPECTE", corps["code"])
	require.Equal(t, false, corps["success"])
}
```

Ajouter l'import `"github.com/yas/numflex-sandbox/internal/config"` au fichier de test.

- [ ] **Step 5 : Lancer les tests, vérifier qu'ils échouent**

Run: `make test`
Expected: FAIL — 404 sur `/demandes/particulier`.

- [ ] **Step 6 : Implémenter la création particulier**

Create `internal/api/dto.go` avec `(*Deps).demandeDTO`, qui lit une demande et la sérialise au format du guide (§7.3) : `id`, `numero`, `typeAbonne`, `typeDemande`, `statutDemande`, `etapeActuelle`, `statutEtapeActuel`, `operateurSource{id,nom}`, `operateurDestinataire{id,nom}`, `dateDemande`, `processus`, `routageInfo`, et `dateFinalisation` si renseignée. Tous les horodatages passent par `d.R.Skew()`.

Create `internal/api/demandes_creation.go` :

```go
package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yas/numflex-sandbox/internal/apperr"
	"github.com/yas/numflex-sandbox/internal/domain"
	"github.com/yas/numflex-sandbox/internal/oid"
)

// routesCreation est complétée au fil des tâches : /demandes/entreprise en
// Task 11, /demandes/restitution en Task 12. Ne câbler ici que ce qui existe,
// sinon le paquet ne compile pas à la fin de cette tâche.
func (d *Deps) routesCreation(g *gin.RouterGroup) {
	g.POST("/demandes/particulier", d.postDemandeParticulier)
}

type clientDTO struct {
	Nom           string `json:"nom"`
	Prenom        string `json:"prenom"`
	DateNaissance string `json:"dateNaissance"`
	LieuNaissance string `json:"lieuNaissance"`
	TypePiece     string `json:"typePiece"`
	NumeroPiece   string `json:"numeroPiece"`
	RaisonSociale string `json:"raisonSociale"`
	NumRC         string `json:"numRC"`
}

type reqParticulier struct {
	Numero                  string    `json:"numero"`
	OtpCode                 string    `json:"otpCode"`
	OperateurSourceID       string    `json:"operateurSourceId"`
	OperateurDestinataireID string    `json:"operateurDestinataireId"`
	TypePortabilite         string    `json:"typePortabilite"`
	Client                  clientDTO `json:"client"`
}

func (d *Deps) postDemandeParticulier(c *gin.Context) {
	var req reqParticulier
	if err := c.ShouldBindJSON(&req); err != nil {
		d.R.Fail(c, apperr.FormatJSONInvalide())
		return
	}
	if champs := validerParticulier(req); len(champs) > 0 {
		d.R.Fail(c, apperr.Validation(champs...))
		return
	}

	appelant := Appelant(c)
	if req.OperateurDestinataireID != appelant.OperateurID {
		d.R.Fail(c, apperr.DemandeAccesRefuse(
			"L'opérateur connecté doit être l'opérateur destinataire de la demande."))
		return
	}
	if e := d.verifierOTP(c, req.Numero, req.OtpCode); e != nil {
		d.R.Fail(c, e)
		return
	}

	etat, err := d.etatNumero(c, req.Numero)
	if err != nil {
		d.R.Fail(c, err)
		return
	}
	if e := domain.VerifierEligibilitePortage(etat, req.OperateurSourceID,
		req.OperateurDestinataireID, domain.DelaiEntrePortages); e != nil {
		d.R.Fail(c, e)
		return
	}

	id := oid.New()
	maintenant := time.Now()

	tx, err2 := d.DB.Pool.Begin(c)
	if err2 != nil {
		d.R.Fail(c, apperr.ErreurInterne("ouverture de transaction"))
		return
	}
	defer tx.Rollback(c)

	var prefixeSource string
	if err := tx.QueryRow(c, `SELECT prefixe_routage FROM operateur WHERE id = $1`,
		req.OperateurSourceID).Scan(&prefixeSource); err != nil {
		d.R.Fail(c, apperr.ValidationEchouee("Opérateur source inconnu"))
		return
	}

	if _, err := tx.Exec(c,
		`INSERT INTO demande
		   (id, numero, type_abonne, type_demande, statut_demande, etape_actuelle,
		    statut_etape_actuel, operateur_source_id, operateur_destinataire_id,
		    createur_operateur_id, processus, routage_info, date_demande, date_debut_etape)
		 VALUES ($1,$2,'PARTICULIER','PORTAGE','EN_COURS','ACCEPTATION','EN_COURS',
		         $3,$4,$4,$5,$6,$7,$7)`,
		id, req.Numero, req.OperateurSourceID, req.OperateurDestinataireID,
		req.TypePortabilite, prefixeSource, maintenant); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("création de la demande"))
		return
	}
	if _, err := tx.Exec(c,
		`INSERT INTO demande_numero (demande_id, numero, statut, routage_info)
		 VALUES ($1,$2,'EN_COURS',$3)`, id, req.Numero, prefixeSource); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("enregistrement du numéro"))
		return
	}
	if _, err := tx.Exec(c,
		`INSERT INTO demande_client
		   (demande_id, nom, prenom, date_naissance, lieu_naissance, type_piece, numero_piece)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		id, req.Client.Nom, req.Client.Prenom, req.Client.DateNaissance,
		req.Client.LieuNaissance, req.Client.TypePiece, req.Client.NumeroPiece); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("enregistrement du client"))
		return
	}
	if err := tx.Commit(c); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("validation de la transaction"))
		return
	}

	if err := d.consommerOTP(c, req.Numero); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("consommation de l'OTP"))
		return
	}

	dto, err3 := d.demandeDTO(c, id)
	if err3 != nil {
		d.R.Fail(c, apperr.ErreurInterne("relecture de la demande"))
		return
	}
	d.R.OK(c, http.StatusCreated, "Demande créée avec succès", dto)
}

// validerParticulier reproduit la validation de la plateforme, y compris son écart
// au guide : lieuNaissance est documenté facultatif mais rejeté si absent (ANO-010).
func validerParticulier(r reqParticulier) []apperr.FieldError {
	var champs []apperr.FieldError
	obligatoire := func(champ, valeur string) {
		if valeur == "" {
			champs = append(champs, apperr.FieldError{
				ObjectName: "demandeParticulierDTO", Field: champ,
				Message: "ne doit pas être vide",
			})
		}
	}
	if !motifMSISDN.MatchString(r.Numero) {
		champs = append(champs, apperr.FieldError{
			ObjectName: "demandeParticulierDTO", Field: "numero",
			Message: "doit correspondre à \"^[0-9]{9}$\"",
		})
	}
	obligatoire("otpCode", r.OtpCode)
	obligatoire("operateurSourceId", r.OperateurSourceID)
	obligatoire("operateurDestinataireId", r.OperateurDestinataireID)
	obligatoire("client.nom", r.Client.Nom)
	obligatoire("client.prenom", r.Client.Prenom)
	obligatoire("client.dateNaissance", r.Client.DateNaissance)
	obligatoire("client.lieuNaissance", r.Client.LieuNaissance)
	obligatoire("client.typePiece", r.Client.TypePiece)
	obligatoire("client.numeroPiece", r.Client.NumeroPiece)
	if r.TypePortabilite != "PREPAID" && r.TypePortabilite != "POSTPAID" {
		champs = append(champs, apperr.FieldError{
			ObjectName: "demandeParticulierDTO", Field: "typePortabilite",
			Message: "doit valoir PREPAID ou POSTPAID",
		})
	}
	return champs
}
```

Ajouter `(*Deps).etatNumero(ctx, msisdn) (domain.EtatNumero, *apperr.Error)` dans `dto.go` : lit la ligne `numero` et calcule `DemandeEnCours` par `EXISTS (SELECT 1 FROM demande_numero dn JOIN demande dm ON dm.id = dn.demande_id WHERE dn.numero = $1 AND dm.statut_demande = 'EN_COURS')`. Un numéro absent du registre est traité comme `OPERATEUR_SOURCE_INCORRECT`.

- [ ] **Step 7 : Lancer les tests, vérifier qu'ils passent**

Run: `make test`
Expected: PASS — les huit tests de création particulier et les dix d'éligibilité.

- [ ] **Step 8 : Commit**

```bash
git add -A
git commit -m "feat: eligibilite au portage et creation de demande particulier"
```

---

## Task 11 : Création d'une demande entreprise (flotte)

**Files:**
- Modify: `internal/api/demandes_creation.go` (`postDemandeEntreprise`)
- Create: `internal/api/creation_entreprise_test.go`

**Interfaces:**
- Consumes: `domain.VerifierEligibilitePortage`, `verifierOTP`, `etatNumero`.
- Produces: rien de nouveau hors le handler.

- [ ] **Step 1 : Écrire les tests**

Create `internal/api/creation_entreprise_test.go` :

```go
package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/seed"
)

func corpsEntreprise(porteur string, flotte []string) map[string]any {
	return map[string]any{
		"numeroPorteurFlotte":     porteur,
		"otpCode":                 "123456",
		"operateurSourceId":       seed.OperateurOrange,
		"operateurDestinataireId": seed.OperateurYAS,
		"typePortabilite":         "POSTPAID",
		"numerosFlotte":           flotte,
		"client": map[string]any{
			"raisonSociale": "Entreprise SARL", "numRC": "123456789",
			"prenom": "Ousmane", "nom": "Diallo", "dateNaissance": "1975-03-20",
			"typePiece": "CNI", "numeroPiece": "1234567890123",
		},
	}
}

func TestFlotteNominale(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("771000001", []string{"771000001", "771000002", "771000003"}))

	require.Equal(t, http.StatusCreated, rep.StatusCode, corps)
	require.Equal(t, "Demande flotte créée", corps["message"])

	data := corps["data"].(map[string]any)
	demande := data["demande"].(map[string]any)
	require.Equal(t, "ENTREPRISE", demande["typeAbonne"])
	require.Equal(t, "ACCEPTATION", demande["etapeActuelle"])
	require.Equal(t, float64(3), data["numerosPortesCount"])
	require.Equal(t, float64(0), data["numerosExclusCount"])
}

func TestFlotteExclusionPartielle(t *testing.T) {
	// BR-006 / invariant 11 : la flotte réussit avec moins de numéros que demandé.
	h := nouveauHarnais(t)
	h.creerPortage("771000009") // ce numéro a désormais une demande en cours

	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("771000001", []string{"771000001", "771000002", "771000009"}))

	require.Equal(t, http.StatusCreated, rep.StatusCode)
	data := corps["data"].(map[string]any)
	require.Equal(t, float64(2), data["numerosPortesCount"])
	require.Equal(t, float64(1), data["numerosExclusCount"])
	require.Equal(t, "1 numéro(s) exclu(s) de la demande.", data["avertissement"])

	exclus := data["numerosExclus"].([]any)
	require.Len(t, exclus, 1)
	premier := exclus[0].(map[string]any)
	require.Equal(t, "771000009", premier["numero"])
	require.Equal(t, "Demande déjà en cours pour ce numéro", premier["raison"])
	require.Equal(t, "DEMANDE_EN_COURS_POUR_NUMERO", premier["codeErreur"])
}

func TestFlotteVide(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("771000001", []string{}))

	require.Equal(t, http.StatusBadRequest, rep.StatusCode)
	require.Contains(t, corps, "fieldErrors")
}

func TestFlotteOperateursMixtes(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("771000001", []string{"771000001", "701000001"}))

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestFlotteAucunNumeroEligible(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "772000001"})

	// Tranche 772 : portée il y a 30 jours, donc sous le délai de 3 mois.
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("772000001", []string{"772000001", "772000002"}))

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "RuntimeException: Aucun numéro de la flotte n'est éligible au portage",
		corps["detail"])

	var n int
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM demande").Scan(&n))
	require.Equal(t, 0, n, "aucune demande ne doit être créée")
}

func TestFlotteUnSeulOTPCouvreToutLaFlotte(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	// OTP envoyé uniquement sur le porteur.
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("771000001", []string{"771000001", "771000002", "771000003"}))

	require.Equal(t, http.StatusCreated, rep.StatusCode)
}
```

- [ ] **Step 2 : Lancer les tests, vérifier qu'ils échouent**

Run: `make test`
Expected: FAIL.

- [ ] **Step 3 : Implémenter la création flotte**

Modify `internal/api/demandes_creation.go` — ajouter la ligne
`g.POST("/demandes/entreprise", d.postDemandeEntreprise)` à `routesCreation`, puis `reqEntreprise` (`numeroPorteurFlotte`, `otpCode`, `operateurSourceId`, `operateurDestinataireId`, `typePortabilite`, `numerosFlotte []string`, `client`) et `postDemandeEntreprise`, dans cet ordre :

1. Validation de forme ; `numerosFlotte` vide → `apperr.Validation` sur le champ `numerosFlotte`, message `ne doit pas être vide`.
2. L'appelant doit être le destinataire.
3. Un seul OTP, vérifié sur `numeroPorteurFlotte`, puis consommé à la fin.
4. Lire l'état de chaque numéro. Si les `operateurActuelId` diffèrent entre eux → `apperr.FlotteOperateursMixtes()`.
5. Pour chaque numéro, `domain.VerifierEligibilitePortage`. En cas d'erreur, **exclure** le numéro : conserver `numero`, `raison` = `e.Message`, `codeErreur` = `e.Code`.
6. Si aucun numéro ne reste → `apperr.AucunNumeroEligible()`, sans rien créer.
7. Créer la demande (`type_abonne = 'ENTREPRISE'`, `numero` = `numeroPorteurFlotte`), une ligne `demande_numero` par numéro retenu (`statut = 'EN_COURS'`, `exclu = false`) et une par numéro exclu (`exclu = true`, `raison_exclusion`, `code_erreur_exclusion`), la ligne `demande_client` avec `raison_sociale` et `num_rc`.
8. Répondre `201` avec le message `Demande flotte créée` et la structure du §7.4 :

```go
data := gin.H{
	"demande": gin.H{
		"id":             id,
		"typeDemande":    "PORTAGE",
		"typeAbonne":     "ENTREPRISE",
		"statutDemande":  "EN_COURS",
		"etapeActuelle":  "ACCEPTATION",
	},
	"numerosPortesCount": len(retenus),
	"numerosExclusCount": len(exclus),
	"numerosExclus":      exclus,
}
if len(exclus) > 0 {
	data["avertissement"] = fmt.Sprintf("%d numéro(s) exclu(s) de la demande.", len(exclus))
}
```

`numerosExclus` est toujours présent, tableau vide compris ; `avertissement` n'apparaît que s'il y a des exclusions.

- [ ] **Step 4 : Lancer les tests, vérifier qu'ils passent**

Run: `make test`
Expected: PASS — les six tests de flotte.

- [ ] **Step 5 : Commit**

```bash
git add -A
git commit -m "feat: creation de demande flotte avec exclusion partielle"
```

---

## Task 12 : Création d'une demande de restitution

**Files:**
- Modify: `internal/api/demandes_creation.go` (`postDemandeRestitution`)
- Create: `internal/api/creation_restitution_test.go`

**Interfaces:**
- Consumes: `domain.VerifierEligibiliteRestitution`.
- Produces: rien de nouveau.

- [ ] **Step 1 : Écrire les tests**

Create `internal/api/creation_restitution_test.go` :

```go
package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/seed"
)

func TestRestitutionNominale(t *testing.T) {
	// Tranche 773 : YAS, portée depuis ORANGE il y a 240 jours.
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})

	require.Equal(t, http.StatusCreated, rep.StatusCode, corps)
	data := corps["data"].(map[string]any)
	require.Equal(t, "RESTITUTION", data["typeDemande"])
	require.Equal(t, "ACCEPTATION", data["etapeActuelle"])

	// L'opérateur d'origine récupère le numéro : il est destinataire.
	require.Equal(t, seed.OperateurOrange,
		data["operateurDestinataire"].(map[string]any)["id"])
	require.Equal(t, seed.OperateurYAS,
		data["operateurSource"].(map[string]any)["id"])

	// routageInfo n'existe qu'à partir de la COMPLETION (§7.10).
	require.Nil(t, data["routageInfo"])
}

func TestRestitutionNumeroNonPorte(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "771000001"})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t,
		"RuntimeException: Le numéro n'a pas été porté, pas de restitution/reverse possible",
		corps["detail"])
}

func TestRestitutionTropTotEstUne500EncapsulantUne400(t *testing.T) {
	// ANO-020.
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "774000001"})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Contains(t, corps["detail"], "error.numeroRestitutionTooEarly")
	require.NotContains(t, corps, "code")
}

func TestRestitutionDejaRestituee(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "775000001"})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "RuntimeException: Ce numéro a déjà été restitué", corps["detail"])
}

func TestRestitutionReserveeALOperateurDOrigine(t *testing.T) {
	h := nouveauHarnais(t)
	// EXPRESSO n'est ni détenteur ni opérateur d'origine du 773000001.
	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.jeton("expresso", "expresso2026"), map[string]any{"numero": "773000001"})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestRestitutionSansOTP(t *testing.T) {
	// §7.5 : le corps ne porte que le numéro, aucun OTP n'est exigé.
	h := nouveauHarnais(t)
	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "773000002"})
	require.Equal(t, http.StatusCreated, rep.StatusCode)
}
```

- [ ] **Step 2 : Lancer les tests, vérifier qu'ils échouent**

Run: `make test`
Expected: FAIL.

- [ ] **Step 3 : Implémenter la restitution**

Modify `internal/api/demandes_creation.go` — ajouter la ligne
`g.POST("/demandes/restitution", d.postDemandeRestitution)` à `routesCreation`, puis
`postDemandeRestitution` :

1. Corps `{ "numero": "..." }` uniquement ; format 9 chiffres sinon `apperr.Validation`.
2. Lire l'état du numéro.
3. **[HYP]** L'appelant doit être `OperateurOrigineID` du numéro ; sinon `apperr.DemandeAccesRefuse("Seul l'opérateur d'origine du numéro peut demander sa restitution.")`. Le guide ne tranche pas la répartition des rôles ; cette décision est documentée au §9.4 de la spec.
4. `domain.VerifierEligibiliteRestitution`.
5. Créer la demande : `type_demande = 'RESTITUTION'`, `type_abonne = 'PARTICULIER'`, `operateur_source_id` = détenteur actuel, `operateur_destinataire_id` = `createur_operateur_id` = opérateur d'origine, `etape_actuelle = 'ACCEPTATION'`, **`routage_info` NULL**, `processus` NULL.
6. Une ligne `demande_numero`, aucune ligne `demande_client`.
7. Répondre `201`, message `Demande de restitution créée avec succès`.

Aucun OTP n'intervient.

- [ ] **Step 4 : Lancer les tests, vérifier qu'ils passent**

Run: `make test`
Expected: PASS — les six tests.

- [ ] **Step 5 : Commit**

```bash
git add -A
git commit -m "feat: creation de demande de restitution"
```

---

## Task 13 : Consultation — les dix listes

**Files:**
- Create: `internal/api/demandes_lecture.go`, `internal/api/lecture_test.go`

**Interfaces:**
- Consumes: `demandeDTO`, `domain.ConfirmateursAttendus`.
- Produces: `(*Deps).routesLecture(g *gin.RouterGroup)` ; `(*Deps).chargerDemande(ctx context.Context, id string) (domain.Demande, *apperr.Error)` réutilisée par les tâches 14 à 17 ; `(*Deps).tousOperateurs(ctx context.Context) ([]string, error)`.

- [ ] **Step 1 : Écrire les tests**

Create `internal/api/lecture_test.go` :

```go
package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// avancerA fait progresser une demande jusqu'à l'étape voulue en manipulant
// directement la base — les endpoints de traitement sont testés ailleurs.
func (h *harnais) avancerA(id, etape string) {
	h.t.Helper()
	_, err := h.db.Pool.Exec(context.Background(),
		`UPDATE demande SET etape_actuelle = $2, statut_etape_actuel = 'EN_COURS',
		                    date_debut_etape = now(), transition_prevue_a = NULL
		  WHERE id = $1`, id, etape)
	require.NoError(h.t, err)
}

func TestMesDemandesVoitSourceEtDestinataire(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	for _, compte := range [][2]string{{"yas", "yas2026"}, {"orange", "orange2026"}} {
		data := h.liste("/api/gateway/v1/demandes/mes-demandes", h.jeton(compte[0], compte[1]))
		require.Len(t, data, 1, compte[0])
		require.Equal(t, id, data[0].(map[string]any)["id"])
	}

	// EXPRESSO n'est partie à rien.
	require.Empty(t, h.liste("/api/gateway/v1/demandes/mes-demandes",
		h.jeton("expresso", "expresso2026")))
}

func TestMesDemandesNAcceptePasDePagination(t *testing.T) {
	h := nouveauHarnais(t)
	h.creerPortage("771000001")

	_, corps := h.appel(http.MethodGet, "/api/gateway/v1/demandes/mes-demandes",
		h.jeton("yas", "yas2026"), nil)

	require.NotContains(t, corps, "page")
	require.NotContains(t, corps, "size")
	require.NotContains(t, corps, "totalElements")
}

func TestAAccepterEstReserveeALaSource(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	data := h.liste("/api/gateway/v1/demandes/a-accepter", h.jeton("orange", "orange2026"))
	require.Len(t, data, 1)
	require.Equal(t, id, data[0].(map[string]any)["id"])

	require.Empty(t, h.liste("/api/gateway/v1/demandes/a-accepter", h.jeton("yas", "yas2026")))
}

func TestATraiterSuitLeResponsableDeLEtape(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	h.avancerA(id, "DESACTIVATION")
	require.Len(t, h.liste("/api/gateway/v1/demandes/a-traiter", h.jeton("orange", "orange2026")), 1)
	require.Empty(t, h.liste("/api/gateway/v1/demandes/a-traiter", h.jeton("yas", "yas2026")))

	h.avancerA(id, "ACTIVATION")
	require.Len(t, h.liste("/api/gateway/v1/demandes/a-traiter", h.jeton("yas", "yas2026")), 1)
	require.Empty(t, h.liste("/api/gateway/v1/demandes/a-traiter", h.jeton("orange", "orange2026")))
}

func TestAConfirmerContientLeTiers(t *testing.T) {
	// D-6, mesuré au SIT : EXPRESSO, ni source ni destinataire, doit confirmer.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "CONFIRMATION")

	require.Len(t, h.liste("/api/gateway/v1/demandes/a-confirmer",
		h.jeton("orange", "orange2026")), 1)
	require.Len(t, h.liste("/api/gateway/v1/demandes/a-confirmer",
		h.jeton("expresso", "expresso2026")), 1)

	// Le destinataire est auto-confirmé : la demande ne figure pas dans sa file.
	require.Empty(t, h.liste("/api/gateway/v1/demandes/a-confirmer",
		h.jeton("yas", "yas2026")))
}

func TestDetailAConfirmerRefuseAuDestinataire(t *testing.T) {
	// Mesuré : GET /a-confirmer/{id} avec le jeton du destinataire répond 500.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "CONFIRMATION")

	rep, _ := h.appel(http.MethodGet, "/api/gateway/v1/demandes/a-confirmer/"+id,
		h.jeton("yas", "yas2026"), nil)
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)

	rep, _ = h.appel(http.MethodGet, "/api/gateway/v1/demandes/a-confirmer/"+id,
		h.jeton("orange", "orange2026"), nil)
	require.Equal(t, http.StatusOK, rep.StatusCode)
}

func TestDetailDemandeInconnue(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodGet,
		"/api/gateway/v1/demandes/a-traiter/6a0000000000000000000000",
		h.jeton("yas", "yas2026"), nil)

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "RuntimeException: Demande introuvable", corps["detail"])
}

func TestInEtOutNeContiennentQueLesPortagesTermines(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	require.Empty(t, h.liste("/api/gateway/v1/demandes/in", h.jeton("yas", "yas2026")))

	_, err := h.db.Pool.Exec(context.Background(),
		`UPDATE demande SET statut_demande = 'TERMINE', etape_actuelle = 'COMPLETION',
		                    statut_etape_actuel = 'VALIDE', date_finalisation = now()
		  WHERE id = $1`, id)
	require.NoError(t, err)

	data := h.liste("/api/gateway/v1/demandes/in", h.jeton("yas", "yas2026"))
	require.Len(t, data, 1)
	require.Equal(t, "TERMINE", data[0].(map[string]any)["statutDemande"])
	require.NotNil(t, data[0].(map[string]any)["dateFinalisation"])

	require.Len(t, h.liste("/api/gateway/v1/demandes/out", h.jeton("orange", "orange2026")), 1)
	require.Empty(t, h.liste("/api/gateway/v1/demandes/out", h.jeton("yas", "yas2026")))
}

func TestInExclutLesRestitutions(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})
	require.Equal(t, http.StatusCreated, rep.StatusCode)
	id := corps["data"].(map[string]any)["id"].(string)

	_, err := h.db.Pool.Exec(context.Background(),
		`UPDATE demande SET statut_demande = 'TERMINE', date_finalisation = now() WHERE id = $1`, id)
	require.NoError(t, err)

	require.Empty(t, h.liste("/api/gateway/v1/demandes/in", h.jeton("orange", "orange2026")),
		"/in ne porte que sur les portages")
}

func TestMessagesDesListes(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")

	cas := map[string]string{
		"/api/gateway/v1/demandes/mes-demandes":    "Demandes récupérées avec succès",
		"/api/gateway/v1/demandes/a-accepter":      "Demandes à accepter récupérées avec succès",
		"/api/gateway/v1/demandes/a-traiter":       "Demandes à traiter récupérées avec succès",
		"/api/gateway/v1/demandes/a-confirmer":     "Demandes à confirmer récupérées avec succès",
		"/api/gateway/v1/demandes/deja-confirmees": "Demandes déjà confirmées récupérées avec succès",
		"/api/gateway/v1/demandes/in":              "Demandes IN récupérées avec succès",
		"/api/gateway/v1/demandes/out":             "Demandes OUT récupérées avec succès",
	}
	for chemin, message := range cas {
		_, corps := h.appel(http.MethodGet, chemin, jeton, nil)
		require.Equalf(t, message, corps["message"], chemin)
	}
}
```

- [ ] **Step 2 : Lancer les tests, vérifier qu'ils échouent**

Run: `make test`
Expected: FAIL.

- [ ] **Step 3 : Implémenter les dix routes**

Create `internal/api/demandes_lecture.go` :

```go
package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/yas/numflex-sandbox/internal/apperr"
	"github.com/yas/numflex-sandbox/internal/domain"
)

func (d *Deps) routesLecture(g *gin.RouterGroup) {
	g.GET("/demandes/mes-demandes", d.getMesDemandes)
	g.GET("/demandes/a-accepter", d.getAAccepter)
	g.GET("/demandes/a-accepter/:id", d.getAAccepterDetail)
	g.GET("/demandes/a-traiter", d.getATraiter)
	g.GET("/demandes/a-traiter/:id", d.getATraiterDetail)
	g.GET("/demandes/a-confirmer", d.getAConfirmer)
	g.GET("/demandes/a-confirmer/:id", d.getAConfirmerDetail)
	g.GET("/demandes/deja-confirmees", d.getDejaConfirmees)
	g.GET("/demandes/in", d.getIn)
	g.GET("/demandes/out", d.getOut)
}
```

Le contrat impose de faire coexister des segments littéraux (`a-accepter`, `a-confirmer`, `in`, `out`…) et un paramètre (`/demandes/:id/acceptation`, `/demandes/:id/annuler`) au même niveau d'URL. **Gin le supporte nativement** — vérifié en enregistrant les dix routes concernées : aucune panique, le littéral l'emporte sur le paramètre au routage. Aucun contournement n'est nécessaire.

Chaque liste applique son filtre :

| Route | Filtre SQL |
|---|---|
| `mes-demandes` | `operateur_source_id = $1 OR operateur_destinataire_id = $1` |
| `a-accepter` | `statut_demande='EN_COURS' AND etape_actuelle='ACCEPTATION' AND operateur_source_id=$1` |
| `a-traiter` | `statut_demande='EN_COURS'` et l'étape courante incombe à `$1` selon `domain.ResponsableEtape` — exprimé en SQL par `(etape_actuelle='DESACTIVATION' AND operateur_source_id=$1) OR (etape_actuelle IN ('ACTIVATION','COMPLETION') AND operateur_destinataire_id=$1 AND NOT (etape_actuelle='COMPLETION' AND type_demande='REVERSE'))` |
| `a-confirmer` | `statut_demande='EN_COURS' AND etape_actuelle='CONFIRMATION'`, l'appelant fait partie de `ConfirmateursAttendus`, et n'a pas déjà confirmé |
| `deja-confirmees` | jointure sur `confirmation` où `operateur_id=$1` — **voir ANO-019 ci-dessous** |
| `in` | `type_demande='PORTAGE' AND statut_demande='TERMINE' AND operateur_destinataire_id=$1` |
| `out` | `type_demande='PORTAGE' AND statut_demande='TERMINE' AND operateur_source_id=$1` |

**ANO-019** — en mode `real`, `deja-confirmees` **omet** les confirmations émises par l'opérateur *source* de la demande : ajouter au filtre `AND c.operateur_id <> dm.operateur_source_id`. En mode `contract`, ne pas appliquer ce filtre. C'est le seul endroit du projet où une requête SQL dépend de `d.R.Fidelity()` ; le commenter explicitement.

Les détails `/{id}` réutilisent le même filtre que leur liste : une demande qui ne satisfait pas le filtre renvoie `apperr.DemandeNonTrouvee()`. C'est ce qui produit le 500 mesuré quand le destinataire consulte `/a-confirmer/{id}`.

Ajouter dans le même fichier :

```go
func (d *Deps) chargerDemande(ctx context.Context, id string) (domain.Demande, *apperr.Error) {
	var dm domain.Demande
	var etape, statutDem, statutEtape, typeDem, typeAb string
	var transition *string

	err := d.DB.Pool.QueryRow(ctx,
		`SELECT id, numero, type_demande, type_abonne, statut_demande, etape_actuelle,
		        statut_etape_actuel, operateur_source_id, operateur_destinataire_id,
		        createur_operateur_id, transition_prevue_a::text
		   FROM demande WHERE id = $1`, id).
		Scan(&dm.ID, &dm.Numero, &typeDem, &typeAb, &statutDem, &etape, &statutEtape,
			&dm.OperateurSourceID, &dm.OperateurDestinataireID,
			&dm.CreateurOperateurID, &transition)
	if errors.Is(err, pgx.ErrNoRows) {
		return dm, apperr.DemandeNonTrouvee()
	}
	if err != nil {
		return dm, apperr.ErreurInterne("lecture de la demande")
	}

	dm.TypeDemande = domain.TypeDemande(typeDem)
	dm.TypeAbonne = domain.TypeAbonne(typeAb)
	dm.StatutDemande = domain.StatutDemande(statutDem)
	dm.EtapeActuelle = domain.Etape(etape)
	dm.StatutEtapeActuel = domain.StatutEtape(statutEtape)
	dm.TransitionEnAttente = transition != nil
	return dm, nil
}

func (d *Deps) tousOperateurs(ctx context.Context) ([]string, error) {
	rows, err := d.DB.Pool.Query(ctx, `SELECT id FROM operateur ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}
```

Toutes les listes renvoient `[]` et jamais `null` quand elles sont vides, et **aucune** ne porte de champ de pagination.

- [ ] **Step 4 : Lancer les tests, vérifier qu'ils passent**

Run: `make test`
Expected: PASS — les dix tests de lecture.

- [ ] **Step 5 : Commit**

```bash
git add -A
git commit -m "feat: les dix listes de consultation des demandes"
```

---

## Task 14 : Acceptation

**Files:**
- Create: `internal/api/acceptation.go`, `internal/api/acceptation_test.go`

**Interfaces:**
- Consumes: `domain.PeutAccepter`, `chargerDemande`, `Moteur.PlanifierTransition`.
- Produces: `(*Deps).routesAcceptation(g *gin.RouterGroup)`.

- [ ] **Step 1 : Écrire les tests**

Create `internal/api/acceptation_test.go` :

```go
package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/seed"
)

func TestAcceptationNominale(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.jeton("orange", "orange2026"),
		map[string]any{"idDemande": id, "accepte": true, "commentaire": "Demande conforme"})

	require.Equal(t, http.StatusOK, rep.StatusCode, corps)
	require.Equal(t, "Étape traitée avec succès", corps["message"])

	// R-10 : la réponse porte l'étape PRÉCÉDANT la transition.
	data := corps["data"].(map[string]any)
	require.Equal(t, "ACCEPTATION", data["etapeActuelle"])

	// La transition est planifiée, pas encore appliquée.
	var prevue *string
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT transition_prevue_a::text FROM demande WHERE id = $1", id).Scan(&prevue))
	require.NotNil(t, prevue)
}

func TestAcceptationParLeDestinataireRefusee(t *testing.T) {
	// TC-034 : refusé — mais en HTTP 500.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.jeton("yas", "yas2026"), map[string]any{"idDemande": id, "accepte": true})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.NotContains(t, corps, "code")
}

func TestRejetSansMotifRefuse(t *testing.T) {
	// TC-044 : refusé — en HTTP 500.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id, "accepte": false})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t,
		"RuntimeException: Un motif de rejet est obligatoire pour rejeter une demande",
		corps["detail"])
}

func TestRejetAvecMotifTermineLaDemande(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.jeton("orange", "orange2026"), map[string]any{
			"idDemande": id, "accepte": false,
			"motifRejetId": seed.MotifIdentiteNonProuvee,
			"commentaire":  "Contrat non résilié",
		})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	var statut, motif string
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT statut_demande, motif_rejet_id FROM demande WHERE id = $1", id).
		Scan(&statut, &motif))
	require.Equal(t, "REJETE", statut)
	require.Equal(t, seed.MotifIdentiteNonProuvee, motif)
}

func TestAcceptationIdDemandeInconnu(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.jeton("orange", "orange2026"),
		map[string]any{"idDemande": "6a0000000000000000000000", "accepte": true})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t, "RuntimeException: Demande introuvable", corps["detail"])
}

func TestAcceptationNumeroRefuseCarV2IdentifieParIdDemande(t *testing.T) {
	// Rupture v1 → v2 : le champ numero n'est plus reconnu.
	h := nouveauHarnais(t)
	h.creerPortage("771000001")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/acceptation",
		h.jeton("orange", "orange2026"),
		map[string]any{"numero": "771000001", "accepte": true})

	require.Equal(t, http.StatusBadRequest, rep.StatusCode)
	champs := corps["fieldErrors"].([]any)
	require.Equal(t, "idDemande", champs[0].(map[string]any)["field"])
}

func TestAcceptationFlotteAvecRejetPartiel(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})
	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("771000001", []string{"771000001", "771000002", "771000003"}))
	id := corps["data"].(map[string]any)["demande"].(map[string]any)["id"].(string)

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/acceptation",
		h.jeton("orange", "orange2026"), map[string]any{
			"accepte": true,
			"numerosRejetes": []map[string]any{
				{"numero": "771000002", "motifRejetId": seed.MotifNumeroInactif},
			},
			"commentaire": "Numéro 771000002 non conforme",
		})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	var statut string
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT statut FROM demande_numero WHERE demande_id = $1 AND numero = '771000002'", id).
		Scan(&statut))
	require.Equal(t, "REJETE", statut)

	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT statut FROM demande_numero WHERE demande_id = $1 AND numero = '771000001'", id).
		Scan(&statut))
	require.Equal(t, "EN_COURS", statut)
}

func TestAcceptationFlotteRejetTotal(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("yas", "yas2026")
	h.appel(http.MethodPost, "/api/gateway/v1/otp/send", jeton,
		map[string]any{"numero": "771000001"})
	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/entreprise", jeton,
		corpsEntreprise("771000001", []string{"771000001", "771000002"}))
	id := corps["data"].(map[string]any)["demande"].(map[string]any)["id"].(string)

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/acceptation",
		h.jeton("orange", "orange2026"), map[string]any{
			"accepte": false, "motifRejetId": seed.MotifDonneesManquantes,
			"commentaire": "Dossier incomplet",
		})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	var statut string
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT statut_demande FROM demande WHERE id = $1", id).Scan(&statut))
	require.Equal(t, "REJETE", statut)
}
```

- [ ] **Step 2 : Lancer les tests, vérifier qu'ils échouent**

Run: `make test`
Expected: FAIL.

- [ ] **Step 3 : Implémenter l'acceptation**

Create `internal/api/acceptation.go` avec deux handlers :

**Gel de la place** — comme `/traitement`, les deux handlers d'acceptation commencent par
`if gelee, _ := d.Moteur.PlaceGelee(c); gelee { d.R.Fail(c, apperr.ErreurInterne("Le traitement des demandes est gelé par un incident interne en cours.")); return }`
(§6.5 de la spec, BR-012).

`postAcceptation` (particulier / restitution) — corps `{idDemande, accepte, motifRejetId?, commentaire?}` :

1. `idDemande` vide → `apperr.Validation` sur le champ `idDemande`, message `ne doit pas être vide`.
2. `chargerDemande`.
3. `domain.PeutAccepter(dm, appelant.OperateurID)`.
4. Si `accepte == false` et `motifRejetId` vide → `apperr.MotifRejetObligatoire()`.
5. Si `motifRejetId` renseigné, vérifier qu'il existe ; sinon `apperr.ValidationEchouee("Motif de rejet inconnu")`.
6. **Rejet** : `statut_demande = 'REJETE'`, `statut_etape_actuel = 'TERMINE'`, `date_finalisation = now()`, `motif_rejet_id`, ligne `etape_historique` d'origine `ACTION`. Aucune transition planifiée.
7. **Acceptation** : ligne `etape_historique` non écrite ici — c'est le moteur qui la pose au moment de la transition ; appeler `d.Moteur.PlanifierTransition(c, id)` et enregistrer le commentaire sur la demande.
8. Répondre `200`, message `Étape traitée avec succès`, `data` = `demandeDTO` **relu après** la planification. Comme la transition n'est pas appliquée, le DTO porte encore `ACCEPTATION` : c'est le comportement mesuré (R-10), pas un défaut.

`postAcceptationFlotte` (`/demandes/:id/acceptation`) — corps `{accepte, numerosRejetes?, motifRejetId?, commentaire?}` : mêmes contrôles, plus le marquage `statut = 'REJETE'` et `motif_rejet_id` sur les lignes `demande_numero` visées par `numerosRejetes`. Un numéro de `numerosRejetes` absent de la flotte → `apperr.ValidationEchouee("Le numéro 77XXXXXXX ne fait pas partie de cette demande")`. Si tous les numéros sont rejetés, la demande passe `REJETE`.

- [ ] **Step 4 : Lancer les tests, vérifier qu'ils passent**

Run: `make test`
Expected: PASS — les huit tests.

- [ ] **Step 5 : Commit**

```bash
git add -A
git commit -m "feat: acceptation particulier et flotte avec rejet partiel"
```

---

## Task 15 : Confirmation

**Files:**
- Create: `internal/api/confirmation.go`, `internal/api/confirmation_test.go`

**Interfaces:**
- Consumes: `domain.ConfirmateursAttendus`, `tousOperateurs`, `chargerDemande`, `Moteur.PlanifierTransition`.
- Produces: `(*Deps).routesConfirmation(g *gin.RouterGroup)`.

- [ ] **Step 1 : Écrire les tests**

Create `internal/api/confirmation_test.go` :

```go
package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfirmationParTousSaufLeDestinataire(t *testing.T) {
	// Mesuré au SIT : ORANGE confirme, l'étape reste EN_COURS ; EXPRESSO la solde.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "CONFIRMATION")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.jeton("orange", "orange2026"),
		map[string]any{"idDemande": id, "commentaire": "Portage confirmé"})

	require.Equal(t, http.StatusOK, rep.StatusCode)
	data := corps["data"].(map[string]any)
	require.Equal(t, "CONFIRMATION", data["etapeActuelle"])
	require.Equal(t, "EN_COURS", data["statutEtapeActuel"])

	var prevue *string
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT transition_prevue_a::text FROM demande WHERE id = $1", id).Scan(&prevue))
	require.Nil(t, prevue, "il manque la confirmation d'EXPRESSO")

	rep, _ = h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.jeton("expresso", "expresso2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT transition_prevue_a::text FROM demande WHERE id = $1", id).Scan(&prevue))
	require.NotNil(t, prevue, "l'étape est soldée, la transition est planifiée")
}

func TestConfirmationParLeDestinataireRefusee(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "CONFIRMATION")

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.jeton("yas", "yas2026"), map[string]any{"idDemande": id})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestDoubleConfirmationRefusee(t *testing.T) {
	// TC-041 : anti-rejeu — refusé, en HTTP 500.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "CONFIRMATION")

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Contains(t, corps["detail"], "déjà confirmé")
}

func TestConfirmationHorsEtapeConfirmation(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Contains(t, corps["detail"], "ACCEPTATION")
}

func TestRestitutionExigeLaConfirmationDuDestinataire(t *testing.T) {
	h := nouveauHarnais(t)
	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/restitution",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})
	id := corps["data"].(map[string]any)["id"].(string)
	h.avancerA(id, "CONFIRMATION")

	// ORANGE est destinataire de la restitution et doit néanmoins confirmer.
	data := h.liste("/api/gateway/v1/demandes/a-confirmer", h.jeton("orange", "orange2026"))
	require.Len(t, data, 1)

	for _, compte := range [][2]string{
		{"orange", "orange2026"}, {"yas", "yas2026"}, {"expresso", "expresso2026"},
	} {
		rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
			h.jeton(compte[0], compte[1]), map[string]any{"idDemande": id})
		require.Equalf(t, http.StatusOK, rep.StatusCode, compte[0])
	}

	var prevue *string
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		"SELECT transition_prevue_a::text FROM demande WHERE id = $1", id).Scan(&prevue))
	require.NotNil(t, prevue)
}

func TestDejaConfirmeesNeTracePasLaSourceEnModeReel(t *testing.T) {
	// ANO-019 : ORANGE confirme avec succès, sa liste renvoie 0.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "CONFIRMATION")

	h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})
	h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.jeton("expresso", "expresso2026"), map[string]any{"idDemande": id})

	require.Empty(t, h.liste("/api/gateway/v1/demandes/deja-confirmees",
		h.jeton("orange", "orange2026")), "la source n'est pas tracée (ANO-019)")
	require.Len(t, h.liste("/api/gateway/v1/demandes/deja-confirmees",
		h.jeton("expresso", "expresso2026")), 1, "le tiers l'est")
}

func TestDejaConfirmeesTraceLaSourceEnModeContrat(t *testing.T) {
	h := nouveauHarnais(t, func(c *config.Config) { c.Fidelity = config.FidelityContract })
	id := h.creerPortage("771000001")
	h.avancerA(id, "CONFIRMATION")

	h.appel(http.MethodPost, "/api/gateway/v1/demandes/a-confirmer",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})

	require.Len(t, h.liste("/api/gateway/v1/demandes/deja-confirmees",
		h.jeton("orange", "orange2026")), 1)
}
```

Ajouter l'import `"github.com/yas/numflex-sandbox/internal/config"`.

- [ ] **Step 2 : Lancer les tests, vérifier qu'ils échouent**

Run: `make test`
Expected: FAIL.

- [ ] **Step 3 : Implémenter la confirmation**

Create `internal/api/confirmation.go` — `postAConfirmer`, corps `{idDemande, commentaire?}` :

0. Gel de la place : même garde que `/traitement` et `/acceptation` (§6.5, BR-012).
1. `idDemande` obligatoire.
2. `chargerDemande` ; statut `EN_COURS` et étape `CONFIRMATION` sinon `apperr.EtapeInvalide(fmt.Sprintf("Cette demande n'est pas à l'étape CONFIRMATION (étape actuelle : %s).", dm.EtapeActuelle))`.
3. `TransitionEnAttente` → `apperr.EtapeInvalide("L'étape CONFIRMATION a déjà été soldée pour cette demande.")`.
4. `tousOperateurs` puis `domain.ConfirmateursAttendus` ; si l'appelant n'y figure pas → `apperr.DemandeAccesRefuse("Votre opérateur n'a pas à confirmer cette demande.")`.
5. `INSERT INTO confirmation` ; une violation de clé primaire (`23505`) → `apperr.DemandeAccesRefuse("Votre opérateur a déjà confirmé cette demande.")`.
6. Compter les confirmations : si elles couvrent tous les confirmateurs attendus, appeler `d.Moteur.PlanifierTransition`.
7. Répondre `200`, message `Étape traitée avec succès`, `data` = `demandeDTO` — qui porte `etapeActuelle: CONFIRMATION` et `statutEtapeActuel: EN_COURS` tant que la transition n'est pas appliquée.

- [ ] **Step 4 : Lancer les tests, vérifier qu'ils passent**

Run: `make test`
Expected: PASS — les sept tests.

- [ ] **Step 5 : Commit**

```bash
git add -A
git commit -m "feat: confirmation avec regle tous-sauf-destinataire et anti-rejeu"
```

---

## Task 16 : Traitement des étapes

**Files:**
- Create: `internal/api/traitement.go`, `internal/api/traitement_test.go`

**Interfaces:**
- Consumes: `domain.PeutTraiter`, `chargerDemande`, `Moteur.PlanifierTransition`, `config.CompletionLatency`.
- Produces: `(*Deps).routesTraitement(g *gin.RouterGroup)`.

- [ ] **Step 1 : Écrire les tests**

Create `internal/api/traitement_test.go` :

```go
package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/config"
)

func TestTraitementDesactivationParLaSource(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "DESACTIVATION")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("orange", "orange2026"),
		map[string]any{"idDemande": id, "commentaire": "Numéro désactivé"})

	require.Equal(t, http.StatusOK, rep.StatusCode, corps)
	require.Equal(t, "Étape traitée avec succès", corps["message"])
	// R-10 : la réponse porte l'étape précédant la transition.
	require.Equal(t, "DESACTIVATION", corps["data"].(map[string]any)["etapeActuelle"])
}

func TestTraitementParLeMauvaisOperateur(t *testing.T) {
	// TC-036 dans son principe : l'étape n'incombe pas à l'appelant.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "DESACTIVATION")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("yas", "yas2026"), map[string]any{"idDemande": id})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Contains(t, corps["detail"], "source")
	require.NotContains(t, corps, "code")
}

func TestTraitementRefuseAcceptationEtConfirmation(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t,
		"RuntimeException: L'étape ACCEPTATION se traite via POST /api/gateway/v1/demandes/acceptation.",
		corps["detail"])

	h.avancerA(id, "CONFIRMATION")
	rep, corps = h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t,
		"RuntimeException: L'étape CONFIRMATION se traite via POST /api/gateway/v1/demandes/a-confirmer.",
		corps["detail"])
}

func TestTraitementEnModeContratRendEtapeInvalideEn409(t *testing.T) {
	h := nouveauHarnais(t, func(c *config.Config) { c.Fidelity = config.FidelityContract })
	id := h.creerPortage("771000001")
	h.avancerA(id, "CONFIRMATION")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})

	require.Equal(t, http.StatusConflict, rep.StatusCode)
	require.Equal(t, "ETAPE_INVALIDE", corps["code"])
	require.Equal(t,
		"L'étape CONFIRMATION se traite via POST /api/gateway/v1/demandes/a-confirmer.",
		corps["message"])
}

func TestChampEtapeAccepteEtIgnoreEnSilence(t *testing.T) {
	// ANO-018 : une intégration v1 non migrée n'échoue pas — elle exécute
	// silencieusement l'étape courante, quelle qu'elle soit.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "DESACTIVATION")

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("orange", "orange2026"),
		map[string]any{"idDemande": id, "etape": "CONFIRMATION"})

	require.Equal(t, http.StatusOK, rep.StatusCode, "ni rejet, ni avertissement")

	// La ligne d'historique est écrite par le moteur au moment de la transition,
	// pas par le handler — il faut donc converger avant de la lire (R-10).
	h.converger()

	var etape, statut string
	require.NoError(t, h.db.Pool.QueryRow(context.Background(),
		`SELECT etape, statut FROM etape_historique WHERE demande_id = $1`, id).
		Scan(&etape, &statut))
	require.Equal(t, "DESACTIVATION", etape, "c'est l'étape courante qui a été exécutée")
}

func TestSecondTraitementPendantLaConvergenceRefuse(t *testing.T) {
	h := nouveauHarnais(t, func(c *config.Config) {
		c.ConvergenceMin = time.Hour
		c.ConvergenceMax = time.Hour
	})
	id := h.creerPortage("771000001")
	h.avancerA(id, "DESACTIVATION")
	jeton := h.jeton("orange", "orange2026")

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement", jeton,
		map[string]any{"idDemande": id})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	rep, _ = h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement", jeton,
		map[string]any{"idDemande": id})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestCompletionReverseReserveeALARTP(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	_, err := h.db.Pool.Exec(context.Background(),
		`UPDATE demande SET type_demande = 'REVERSE', etape_actuelle = 'COMPLETION',
		                    date_debut_etape = now() WHERE id = $1`, id)
	require.NoError(t, err)

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("yas", "yas2026"), map[string]any{"idDemande": id})

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t,
		"RuntimeException: La complétion (COMPLETION) d'une demande REVERSE est réservée à l'ARTP, une fois que tous les opérateurs ont confirmé.",
		corps["detail"])
}

func TestLatenceDeCompletion(t *testing.T) {
	// ANO-005 : COMPLETION répond en ~30 s. Ici réduit à 300 ms pour le test.
	h := nouveauHarnais(t, func(c *config.Config) {
		c.CompletionLatency = 300 * time.Millisecond
	})
	id := h.creerPortage("771000001")
	h.avancerA(id, "COMPLETION")

	debut := time.Now()
	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("yas", "yas2026"), map[string]any{"idDemande": id})
	ecoule := time.Since(debut)

	require.Equal(t, http.StatusOK, rep.StatusCode)
	require.GreaterOrEqual(t, ecoule, 300*time.Millisecond)
}

func TestAucuneCleDIdempotenceNEstLue(t *testing.T) {
	// ANO-005 : NumFlex n'accepte aucun Idempotency-Key ; le rejeu tombe sur
	// une erreur d'état, indiscernable d'une panne.
	h := nouveauHarnais(t, func(c *config.Config) {
		c.ConvergenceMin = time.Hour
		c.ConvergenceMax = time.Hour
	})
	id := h.creerPortage("771000001")
	h.avancerA(id, "DESACTIVATION")

	req := map[string]any{"idDemande": id}
	jeton := h.jeton("orange", "orange2026")
	h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement", jeton, req)

	rep := h.brut(http.MethodPost, "/api/gateway/v1/demandes/traitement", jeton, req)
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}
```

- [ ] **Step 2 : Lancer les tests, vérifier qu'ils échouent**

Run: `make test`
Expected: FAIL.

- [ ] **Step 3 : Implémenter le traitement**

Create `internal/api/traitement.go` :

```go
package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yas/numflex-sandbox/internal/apperr"
	"github.com/yas/numflex-sandbox/internal/domain"
)

func (d *Deps) routesTraitement(g *gin.RouterGroup) {
	g.POST("/demandes/traitement", d.postTraitement)
}

// reqTraitement ne déclare PAS de champ etape : retiré en v2. Un client v1 qui
// l'envoie encore n'est ni rejeté ni averti — le champ est simplement ignoré
// et l'étape courante est exécutée (ANO-018).
type reqTraitement struct {
	IDDemande   string `json:"idDemande"`
	Commentaire string `json:"commentaire"`
}

func (d *Deps) postTraitement(c *gin.Context) {
	var req reqTraitement
	if err := c.ShouldBindJSON(&req); err != nil {
		d.R.Fail(c, apperr.FormatJSONInvalide())
		return
	}
	if req.IDDemande == "" {
		d.R.Fail(c, apperr.Validation(apperr.FieldError{
			ObjectName: "traitementDTO", Field: "idDemande", Message: "ne doit pas être vide",
		}))
		return
	}

	if gelee, err := d.Moteur.PlaceGelee(c); err == nil && gelee {
		d.R.Fail(c, apperr.ErreurInterne(
			"Le traitement des demandes est gelé par un incident interne en cours."))
		return
	}

	dm, e := d.chargerDemande(c, req.IDDemande)
	if e != nil {
		d.R.Fail(c, e)
		return
	}
	if e := domain.PeutTraiter(dm, Appelant(c).OperateurID); e != nil {
		d.R.Fail(c, e)
		return
	}

	// ANO-005 : la COMPLETION est la seule étape lente — ~30 s mesurés.
	if dm.EtapeActuelle == domain.EtapeCompletion && d.Cfg.CompletionLatency > 0 {
		time.Sleep(d.Cfg.CompletionLatency)
	}

	if req.Commentaire != "" {
		if _, err := d.DB.Pool.Exec(c,
			`UPDATE demande SET commentaire = $2 WHERE id = $1`,
			dm.ID, req.Commentaire); err != nil {
			d.R.Fail(c, apperr.ErreurInterne("enregistrement du commentaire"))
			return
		}
	}
	if err := d.Moteur.PlanifierTransition(c, dm.ID); err != nil {
		d.R.Fail(c, apperr.ErreurInterne("planification de la transition"))
		return
	}

	// La demande est relue AVANT que la transition ne soit appliquée : la réponse
	// porte donc l'étape précédente. C'est le comportement mesuré (R-10) — un
	// client qui enchaîne sur ce corps émet l'étape suivante trop tôt.
	dto, err := d.demandeDTO(c, dm.ID)
	if err != nil {
		d.R.Fail(c, apperr.ErreurInterne("relecture de la demande"))
		return
	}
	d.R.OK(c, http.StatusOK, "Étape traitée avec succès", dto)
}
```

- [ ] **Step 4 : Lancer les tests, vérifier qu'ils passent**

Run: `make test`
Expected: PASS — les neuf tests.

- [ ] **Step 5 : Commit**

```bash
git add -A
git commit -m "feat: traitement des etapes avec etape deduite et convergence differee"
```

---

## Task 17 : Annulation

**Files:**
- Create: `internal/api/annulation.go`, `internal/api/annulation_test.go`

**Interfaces:**
- Consumes: `domain.PeutAnnuler`, `chargerDemande`.
- Produces: `(*Deps).routesAnnulation(g *gin.RouterGroup)`.

- [ ] **Step 1 : Écrire les tests**

Create `internal/api/annulation_test.go` :

```go
package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnnulationParLeCreateur(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.jeton("yas", "yas2026"), nil)

	require.Equal(t, http.StatusOK, rep.StatusCode, corps)
	require.Equal(t, "Demande annulée avec succès", corps["message"])

	data := corps["data"].(map[string]any)
	require.Equal(t, "ANNULE", data["statutDemande"])
	require.Equal(t, "ACCEPTATION", data["etapeActuelle"])
	require.Equal(t, "TERMINE", data["statutEtapeActuel"])
	require.NotNil(t, data["dateFinalisation"])
}

func TestAnnulationParLaSourceRefusee(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.jeton("orange", "orange2026"), nil)

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t,
		"RuntimeException: Seul l'opérateur ayant créé la demande (opérateur destinataire) peut l'annuler.",
		corps["detail"])
}

func TestAnnulationApresAcceptationRefusee(t *testing.T) {
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "DESACTIVATION")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.jeton("yas", "yas2026"), nil)

	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
	require.Equal(t,
		"RuntimeException: Cette demande ne peut plus être annulée (étape actuelle : DESACTIVATION).",
		corps["detail"])
}

func TestAnnulationSansCorpsDeRequete(t *testing.T) {
	// §7.11 : aucun corps n'est requis.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")

	rep := h.brut(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.jeton("yas", "yas2026"), nil)
	require.Equal(t, http.StatusOK, rep.StatusCode)
}

func TestAnnulationEnModeContrat(t *testing.T) {
	h := nouveauHarnais(t, func(c *config.Config) { c.Fidelity = config.FidelityContract })
	id := h.creerPortage("771000001")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/demandes/"+id+"/annuler",
		h.jeton("orange", "orange2026"), nil)

	require.Equal(t, http.StatusForbidden, rep.StatusCode)
	require.Equal(t, "DEMANDE_ACCES_REFUSE", corps["code"])
}
```

Ajouter l'import `"github.com/yas/numflex-sandbox/internal/config"`.

- [ ] **Step 2 : Lancer les tests, vérifier qu'ils échouent**

Run: `make test`
Expected: FAIL.

- [ ] **Step 3 : Implémenter l'annulation**

Create `internal/api/annulation.go` — `postAnnuler` sur `/demandes/:id/annuler`, sans corps :

1. `chargerDemande` par le paramètre d'URL.
2. `domain.PeutAnnuler(dm, appelant.OperateurID)`.
3. `UPDATE demande SET statut_demande='ANNULE', statut_etape_actuel='TERMINE', date_finalisation=now(), transition_prevue_a=NULL`.
4. Ligne `etape_historique` avec `origine = 'ACTION'`, `statut = 'TERMINE'`.
5. Répondre `200`, message `Demande annulée avec succès`, `data` = `demandeDTO`.

- [ ] **Step 4 : Lancer les tests, vérifier qu'ils passent**

Run: `make test`
Expected: PASS — les cinq tests.

- [ ] **Step 5 : Commit**

```bash
git add -A
git commit -m "feat: annulation reservee au createur avant acceptation"
```

---

## Task 18 : Incidents et gel de la place

**Files:**
- Create: `internal/api/incidents.go`, `internal/api/incidents_test.go`

**Interfaces:**
- Consumes: `seed.TypeIncidentGateway`, `seed.TypeIncidentTechnique`, `Moteur.PlaceGelee`.
- Produces: `(*Deps).routesIncidents(g *gin.RouterGroup)`.

- [ ] **Step 1 : Écrire les tests**

Create `internal/api/incidents_test.go` :

```go
package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/seed"
)

func TestDeclarationIncidentGateway(t *testing.T) {
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/incidents/gateway",
		h.jeton("orange", "orange2026"),
		map[string]any{"commentaire": "Timeout de connexion à l'API gateway numFlex"})

	require.Equal(t, http.StatusCreated, rep.StatusCode)
	require.Equal(t, "Incident déclaré avec succès", corps["message"])

	data := corps["data"].(map[string]any)
	require.Equal(t, seed.TypeIncidentGateway, data["typeIncidentId"])
	require.Equal(t, "Gateway", data["type"])
	require.Equal(t, false, data["figeSysteme"])
	require.Equal(t, "Timeout de connexion à l'API gateway numFlex", data["description"])
	require.Equal(t, "EN_COURS", data["statut"])
	require.Equal(t, "ORANGE", data["operateur"].(map[string]any)["nom"])
}

func TestDeclarationIncidentInterneGelLaPlace(t *testing.T) {
	// BR-012 : un incident interne bloque le traitement pour tous les opérateurs.
	h := nouveauHarnais(t)
	id := h.creerPortage("771000001")
	h.avancerA(id, "DESACTIVATION")

	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/incidents/interne",
		h.jeton("expresso", "expresso2026"),
		map[string]any{"commentaire": "Panne du système de routage interne, portages bloqués"})
	require.Equal(t, http.StatusCreated, rep.StatusCode)
	require.Equal(t, true, corps["data"].(map[string]any)["figeSysteme"])
	incidentID := corps["data"].(map[string]any)["id"].(string)

	// ORANGE, étranger à l'incident, ne peut plus traiter.
	rep, _ = h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)

	// Résolution par le déclarant : la place repart.
	rep, _ = h.appel(http.MethodPost,
		"/api/gateway/v1/incidents/interne/"+incidentID+"/resoudre",
		h.jeton("expresso", "expresso2026"), map[string]any{"commentaire": "Service rétabli"})
	require.Equal(t, http.StatusOK, rep.StatusCode)

	rep, _ = h.appel(http.MethodPost, "/api/gateway/v1/demandes/traitement",
		h.jeton("orange", "orange2026"), map[string]any{"idDemande": id})
	require.Equal(t, http.StatusOK, rep.StatusCode)
}

func TestSeulLeDeclarantResout(t *testing.T) {
	h := nouveauHarnais(t)
	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/incidents/gateway",
		h.jeton("orange", "orange2026"), map[string]any{"commentaire": "timeout"})
	id := corps["data"].(map[string]any)["id"].(string)

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/incidents/gateway/"+id+"/resoudre",
		h.jeton("yas", "yas2026"), map[string]any{"commentaire": "rétabli"})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestResolutionParLeMauvaisSegment(t *testing.T) {
	h := nouveauHarnais(t)
	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/incidents/interne",
		h.jeton("orange", "orange2026"), map[string]any{"commentaire": "panne"})
	id := corps["data"].(map[string]any)["id"].(string)

	rep, corps2 := h.appel(http.MethodPost, "/api/gateway/v1/incidents/gateway/"+id+"/resoudre",
		h.jeton("orange", "orange2026"), map[string]any{"commentaire": "rétabli"})

	require.Equal(t, http.StatusBadRequest, rep.StatusCode)
	champs := corps2["fieldErrors"].([]any)
	require.Contains(t, champs[0].(map[string]any)["message"],
		"/api/gateway/v1/incidents/interne")
}

func TestUnSeulIncidentInterneOuvertParOperateur(t *testing.T) {
	h := nouveauHarnais(t)
	jeton := h.jeton("orange", "orange2026")

	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/incidents/interne", jeton,
		map[string]any{"commentaire": "panne 1"})
	require.Equal(t, http.StatusCreated, rep.StatusCode)

	rep, _ = h.appel(http.MethodPost, "/api/gateway/v1/incidents/interne", jeton,
		map[string]any{"commentaire": "panne 2"})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestMesIncidentsSontCloisonnesParSegmentEtParOperateur(t *testing.T) {
	h := nouveauHarnais(t)
	h.appel(http.MethodPost, "/api/gateway/v1/incidents/gateway",
		h.jeton("orange", "orange2026"), map[string]any{"commentaire": "timeout"})
	h.appel(http.MethodPost, "/api/gateway/v1/incidents/interne",
		h.jeton("orange", "orange2026"), map[string]any{"commentaire": "panne"})

	jetonOrange := h.jeton("orange", "orange2026")
	require.Len(t, h.liste("/api/gateway/v1/incidents/gateway/mes-incidents", jetonOrange), 1)
	require.Len(t, h.liste("/api/gateway/v1/incidents/interne/mes-incidents", jetonOrange), 1)

	require.Empty(t, h.liste("/api/gateway/v1/incidents/gateway/mes-incidents",
		h.jeton("yas", "yas2026")))
}

func TestDeclarationSansTypeIncidentId(t *testing.T) {
	// §7.12 : le corps ne prend qu'un commentaire ; le type est résolu côté serveur.
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/incidents/gateway",
		h.jeton("yas", "yas2026"),
		map[string]any{"commentaire": "test", "typeIncidentId": seed.TypeIncidentTechnique})

	require.Equal(t, http.StatusCreated, rep.StatusCode)
	// Le typeIncidentId fourni est ignoré : c'est l'endpoint qui décide.
	require.Equal(t, seed.TypeIncidentGateway, corps["data"].(map[string]any)["typeIncidentId"])
}
```

- [ ] **Step 2 : Lancer les tests, vérifier qu'ils échouent**

Run: `make test`
Expected: FAIL.

- [ ] **Step 3 : Implémenter les incidents**

Create `internal/api/incidents.go` — six routes, deux familles partageant la même logique paramétrée par `figeSysteme` :

1. `POST /incidents/gateway` et `POST /incidents/interne` : corps `{commentaire}` uniquement. Le type est résolu par `SELECT id, libelle FROM type_incident WHERE fige_systeme = $1 LIMIT 1` ; tout `typeIncidentId` présent dans le corps est ignoré.
2. La colonne `incident.fige_systeme` est renseignée depuis le type résolu. Pour le segment interne uniquement, un incident `EN_COURS` déjà ouvert chez l'appelant → `apperr.EtapeInvalide("Un incident interne est déjà ouvert pour votre opérateur.")` — l'index unique partiel de la migration le garantit aussi côté base. Le segment gateway n'est soumis à aucune limite de ce genre.
3. Réponse `201`, message `Incident déclaré avec succès`, `data` = `{id, typeIncidentId, type, figeSysteme, description, statut, dateOuverture, operateur{id,nom}}`.
4. `POST /incidents/{gateway|interne}/:id/resoudre` : incident inexistant → `apperr.DemandeNonTrouvee()` avec le message `Incident introuvable` ; déclarant différent de l'appelant → `apperr.DemandeAccesRefuse("Seul l'opérateur ayant déclaré l'incident peut le résoudre.")` ; mauvais segment → `apperr.Validation(apperr.FieldError{ObjectName:"incidentDTO", Field:"id", Message:"Cet incident se résout via POST /api/gateway/v1/incidents/interne/{id}/resoudre"})` (ou `/gateway/` selon le cas). Sinon `statut='RESOLU'`, `date_resolution=now()`, `commentaire_resolution`. Réponse `200`, message `Incident résolu avec succès`.
5. `GET /incidents/{gateway|interne}/mes-incidents` : incidents de l'appelant dans le segment, tous statuts. Ces deux listes **acceptent** `page` et `size` (défauts `0` et `20`) — contrairement aux listes de demandes.

- [ ] **Step 4 : Lancer les tests, vérifier qu'ils passent**

Run: `make test`
Expected: PASS — les sept tests.

- [ ] **Step 5 : Commit**

```bash
git add -A
git commit -m "feat: incidents gateway et internes, gel de la place"
```

---

## Task 19 : Reverse — endpoints, CLI régulateur et complétion ARTP

**Files:**
- Create: `internal/api/reverse.go`, `internal/api/reverse_test.go`
- Create: `cmd/artp/main.go`
- Modify: `internal/engine/engine.go` (`validerReversesAutomatiquement`, `completerReversesConfirmes`)
- Create: `internal/engine/reverse.go`, `internal/engine/reverse_test.go`

**Interfaces:**
- Consumes: `domain`, `store`, `config.ReverseAutoValidation`.
- Produces:
  - `(*Deps).routesReverse(g *gin.RouterGroup)` — `POST /reverse-requests`, `GET /reverse-requests/mes-demandes`.
  - `engine.ValiderReverse(ctx context.Context, db *store.DB, reverseID string) error` — crée la `Demande` REVERSE directement à `CONFIRMATION`.
  - `engine.RejeterReverse(ctx context.Context, db *store.DB, reverseID string) error`.
  - Binaire `artp` : `artp reverse valider <id>`, `artp reverse rejeter <id>`, `artp reverse lister`, `artp seed`.

- [ ] **Step 1 : Écrire les tests**

Create `internal/api/reverse_test.go` :

```go
package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/seed"
)

func TestSoumissionReverseParLOperateurDOrigine(t *testing.T) {
	// Tranche 773 : YAS actuellement, ORANGE à l'origine.
	h := nouveauHarnais(t)
	rep, corps := h.appel(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})

	require.Equal(t, http.StatusCreated, rep.StatusCode, corps)
	require.Equal(t, "Demande de reverse soumise avec succès", corps["message"])

	data := corps["data"].(map[string]any)
	require.Equal(t, "773000001", data["numero"])
	require.Equal(t, "EN_ATTENTE", data["statut"])
	require.Equal(t, seed.OperateurOrange, data["operateur"].(map[string]any)["id"])
}

func TestSoumissionReverseParUnAutreOperateurRefusee(t *testing.T) {
	h := nouveauHarnais(t)
	rep, _ := h.appel(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.jeton("yas", "yas2026"), map[string]any{"numero": "773000001"})
	require.Equal(t, http.StatusInternalServerError, rep.StatusCode)
}

func TestMesDemandesReverse(t *testing.T) {
	h := nouveauHarnais(t)
	h.appel(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})

	data := h.liste("/api/gateway/v1/reverse-requests/mes-demandes",
		h.jeton("orange", "orange2026"))
	require.Len(t, data, 1)
	require.Equal(t, "EN_ATTENTE", data[0].(map[string]any)["statut"])

	require.Empty(t, h.liste("/api/gateway/v1/reverse-requests/mes-demandes",
		h.jeton("yas", "yas2026")))
}

func TestAucunEndpointDAnnulationDeReverse(t *testing.T) {
	// §7.6 : « Il n'existe pas d'endpoint pour annuler une demande de reverse. »
	h := nouveauHarnais(t)
	_, corps := h.appel(http.MethodPost, "/api/gateway/v1/reverse-requests",
		h.jeton("orange", "orange2026"), map[string]any{"numero": "773000001"})
	id := corps["data"].(map[string]any)["id"].(string)

	rep := h.brut(http.MethodPost, "/api/gateway/v1/reverse-requests/"+id+"/annuler",
		h.jeton("orange", "orange2026"), nil)
	require.Equal(t, http.StatusNotFound, rep.StatusCode)
}
```

Create `internal/engine/reverse_test.go` :

```go
package engine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/config"
	"github.com/yas/numflex-sandbox/internal/seed"
	"github.com/yas/numflex-sandbox/internal/store"
)

func insererReverse(t *testing.T, db *store.DB, id string) {
	t.Helper()
	_, err := db.Pool.Exec(context.Background(),
		`INSERT INTO reverse_request (id, numero, operateur_id, statut, date_demande)
		 VALUES ($1,'773000001',$2,'EN_ATTENTE',now())`, id, seed.OperateurOrange)
	require.NoError(t, err)
}

func TestValidationCreeUneDemandeDirectementEnConfirmation(t *testing.T) {
	e, db := moteur(t)
	insererReverse(t, db, "r1")

	require.NoError(t, ValiderReverse(context.Background(), db, "r1"))

	var statut string
	var demandeID *string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT statut, demande_id FROM reverse_request WHERE id = 'r1'`).
		Scan(&statut, &demandeID))
	require.Equal(t, "VALIDE", statut)
	require.NotNil(t, demandeID)

	var typeDem, etape string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT type_demande, etape_actuelle FROM demande WHERE id = $1`, *demandeID).
		Scan(&typeDem, &etape))
	require.Equal(t, "REVERSE", typeDem)
	require.Equal(t, "CONFIRMATION", etape, "ni ACCEPTATION, ni DESACTIVATION/ACTIVATION")

	_ = e
}

func TestRejetNeCreeAucuneDemande(t *testing.T) {
	_, db := moteur(t)
	insererReverse(t, db, "r1")

	require.NoError(t, RejeterReverse(context.Background(), db, "r1"))

	var statut string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT statut FROM reverse_request WHERE id = 'r1'`).Scan(&statut))
	require.Equal(t, "REJETE", statut)

	var n int
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM demande`).Scan(&n))
	require.Equal(t, 0, n)
}

func TestValidationAutomatiqueApresDelai(t *testing.T) {
	e, db := moteur(t, func(c *config.Config) {
		c.ReverseAutoValidation = time.Nanosecond
	})
	insererReverse(t, db, "r1")

	require.NoError(t, e.Tick(context.Background()))

	var statut string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT statut FROM reverse_request WHERE id = 'r1'`).Scan(&statut))
	require.Equal(t, "VALIDE", statut)
}

func TestPasDeValidationAutomatiqueParDefaut(t *testing.T) {
	e, db := moteur(t) // ReverseAutoValidation = 0
	insererReverse(t, db, "r1")

	require.NoError(t, e.Tick(context.Background()))

	var statut string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT statut FROM reverse_request WHERE id = 'r1'`).Scan(&statut))
	require.Equal(t, "EN_ATTENTE", statut)
}

func TestLARTPCompleteUnReverseUneFoisToutLeMondeConfirme(t *testing.T) {
	e, db := moteur(t)
	insererReverse(t, db, "r1")
	require.NoError(t, ValiderReverse(context.Background(), db, "r1"))

	var demandeID string
	require.NoError(t, db.Pool.QueryRow(context.Background(),
		`SELECT demande_id FROM reverse_request WHERE id = 'r1'`).Scan(&demandeID))

	for _, op := range []string{seed.OperateurOrange, seed.OperateurYAS, seed.OperateurExpresso} {
		_, err := db.Pool.Exec(context.Background(),
			`INSERT INTO confirmation (demande_id, operateur_id, date_conf) VALUES ($1,$2,now())`,
			demandeID, op)
		require.NoError(t, err)
	}

	require.NoError(t, e.Tick(context.Background()))
	require.NoError(t, e.Tick(context.Background()))

	etape, _, statutDemande := etatDemande(t, db, demandeID)
	require.Equal(t, "TERMINE", statutDemande)
	require.Equal(t, "COMPLETION", etape)
}
```

- [ ] **Step 2 : Lancer les tests, vérifier qu'ils échouent**

Run: `make test`
Expected: FAIL.

- [ ] **Step 3 : Implémenter les endpoints reverse**

Create `internal/api/reverse.go` :

- `POST /reverse-requests`, corps `{numero}`. L'appelant doit être `operateur_origine_id` du numéro, sinon `apperr.DemandeAccesRefuse("Seul l'opérateur source (opérateur d'origine du numéro) peut soumettre une demande de reverse pour ce numéro.")`. Le numéro doit être porté, sinon `apperr.NumeroNonPorte()`. Insertion en `EN_ATTENTE`. Réponse `201`, message `Demande de reverse soumise avec succès`, `data` = `{id, numero, statut, dateDemande, operateur{id,nom}}`.
- `GET /reverse-requests/mes-demandes` : les demandes de l'appelant, tous statuts, **avec** `page` et `size`. Message `Demandes de reverse récupérées avec succès`.
- Aucune route d'annulation : le §7.6 l'exclut explicitement.

- [ ] **Step 4 : Implémenter les actes de l'ARTP dans le moteur**

Create `internal/engine/reverse.go` :

```go
package engine

import (
	"context"
	"time"

	"github.com/yas/numflex-sandbox/internal/oid"
	"github.com/yas/numflex-sandbox/internal/store"
)

// ValiderReverse est un acte de l'ARTP, hors périmètre de l'API gateway (§7.6).
// Il crée une Demande de type REVERSE directement à l'étape CONFIRMATION : ni
// ACCEPTATION, ni DESACTIVATION/ACTIVATION.
func ValiderReverse(ctx context.Context, db *store.DB, reverseID string) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var numero, operateurOrigine, statut string
	if err := tx.QueryRow(ctx,
		`SELECT numero, operateur_id, statut FROM reverse_request WHERE id = $1 FOR UPDATE`,
		reverseID).Scan(&numero, &operateurOrigine, &statut); err != nil {
		return err
	}
	if statut != "EN_ATTENTE" {
		return nil
	}

	var detenteurActuel string
	if err := tx.QueryRow(ctx,
		`SELECT operateur_actuel_id FROM numero WHERE msisdn = $1`, numero).
		Scan(&detenteurActuel); err != nil {
		return err
	}

	id := oid.New()
	maintenant := time.Now()
	if _, err := tx.Exec(ctx,
		`INSERT INTO demande
		   (id, numero, type_abonne, type_demande, statut_demande, etape_actuelle,
		    statut_etape_actuel, operateur_source_id, operateur_destinataire_id,
		    createur_operateur_id, date_demande, date_debut_etape)
		 VALUES ($1,$2,'PARTICULIER','REVERSE','EN_COURS','CONFIRMATION','EN_COURS',
		         $3,$4,$4,$5,$5)`,
		id, numero, detenteurActuel, operateurOrigine, maintenant); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO demande_numero (demande_id, numero, statut) VALUES ($1,$2,'EN_COURS')`,
		id, numero); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE reverse_request SET statut='VALIDE', date_decision=$2, demande_id=$3
		  WHERE id = $1`, reverseID, maintenant, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func RejeterReverse(ctx context.Context, db *store.DB, reverseID string) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE reverse_request SET statut='REJETE', date_decision=now()
		  WHERE id = $1 AND statut = 'EN_ATTENTE'`, reverseID)
	return err
}

func (e *Engine) validerReversesAutomatiquement(ctx context.Context) error {
	if e.cfg.ReverseAutoValidation <= 0 {
		return nil
	}
	rows, err := e.db.Pool.Query(ctx,
		`SELECT id FROM reverse_request
		  WHERE statut = 'EN_ATTENTE' AND date_demande + make_interval(secs => $1) <= now()`,
		e.cfg.ReverseAutoValidation.Seconds())
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()

	for _, id := range ids {
		if err := ValiderReverse(ctx, e.db, id); err != nil {
			return err
		}
	}
	return nil
}

// completerReversesConfirmes : la COMPLETION d'un REVERSE est réservée à l'ARTP.
// Aucun endpoint ne l'expose ; c'est le moteur qui la prononce une fois que tous
// les opérateurs ont confirmé.
func (e *Engine) completerReversesConfirmes(ctx context.Context) error {
	rows, err := e.db.Pool.Query(ctx,
		`SELECT d.id FROM demande d
		  WHERE d.type_demande = 'REVERSE'
		    AND d.statut_demande = 'EN_COURS'
		    AND d.etape_actuelle = 'CONFIRMATION'
		    AND d.transition_prevue_a IS NULL
		    AND (SELECT count(*) FROM confirmation c WHERE c.demande_id = d.id)
		        >= (SELECT count(*) FROM operateur)`)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()

	for _, id := range ids {
		// CONFIRMATION → COMPLETION, puis COMPLETION → TERMINE.
		if err := e.AppliquerTransition(ctx, id, "ACTION"); err != nil {
			return err
		}
		if err := e.AppliquerTransition(ctx, id, "ACTION"); err != nil {
			return err
		}
	}
	return nil
}
```

Supprimer de `engine.go` les corps provisoires de ces deux méthodes.

- [ ] **Step 5 : Implémenter le CLI régulateur**

Create `cmd/artp/main.go` — un binaire distinct, **sans serveur HTTP**, qui porte les actes que le contrat ARTP place hors de l'API :

```
artp reverse lister              liste les demandes de reverse et leur statut
artp reverse valider <id>        valide — crée la Demande REVERSE à CONFIRMATION
artp reverse rejeter <id>        rejette
artp seed                        rejoue le seed (idempotent)
```

Il lit `DATABASE_URL`, ouvre le pool, appelle `engine.ValiderReverse` / `engine.RejeterReverse` / `seed.Run`, et sort en code 1 sur erreur avec un message en français.

- [ ] **Step 6 : Lancer les tests, vérifier qu'ils passent**

Run: `make test`
Expected: PASS — les quatre tests d'API et les cinq du moteur.

- [ ] **Step 7 : Commit**

```bash
git add -A
git commit -m "feat: demandes de reverse, actes ARTP et CLI regulateur"
```

---

## Task 20 : Bout en bout, bascule de fidélité, collection Postman, README

**Files:**
- Create: `test/e2e_test.go`
- Create: `postman/numflex-sandbox.postman_collection.json`, `postman/numflex-sandbox.postman_environment.json`
- Create: `README.md`

**Interfaces:**
- Consumes: tout le reste.
- Produces: rien de nouveau.

- [ ] **Step 1 : Écrire le scénario de bout en bout**

Create `test/e2e_test.go`. Le paquet `test` monte le serveur comme le harnais de `internal/api` (dupliquer le harnais y serait du copier-coller : exporter `api.NewRouter` suffit, et les helpers sont réécrits localement en une trentaine de lignes).

```go
package test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yas/numflex-sandbox/internal/config"
	"github.com/yas/numflex-sandbox/internal/seed"
)

// TestPortageCompletJusquATermine rejoue le §10 du guide : ORANGE → YAS, avec la
// confirmation d'EXPRESSO, jusqu'à TERMINE et apparition dans /in et /out.
func TestPortageCompletJusquATermine(t *testing.T) {
	h := nouveauHarnais(t) // profil déterministe : convergence et latences nulles

	yas := h.jeton("yas", "yas2026")
	orange := h.jeton("orange", "orange2026")
	expresso := h.jeton("expresso", "expresso2026")

	// 1-2. OTP puis création par le destinataire.
	h.post("/api/gateway/v1/otp/send", yas, map[string]any{"numero": "771000001"})
	_, corps := h.post("/api/gateway/v1/demandes/particulier", yas, corpsParticulier("771000001"))
	id := corps["data"].(map[string]any)["id"].(string)

	// 3. La source voit la demande dans sa file.
	require.Len(t, h.liste("/api/gateway/v1/demandes/a-accepter", orange), 1)

	// 4. Acceptation par la source, puis convergence.
	h.post("/api/gateway/v1/demandes/acceptation", orange,
		map[string]any{"idDemande": id, "accepte": true})
	h.converger()
	require.Equal(t, "DESACTIVATION", h.etape(id))

	// 5. Désactivation par la source.
	h.post("/api/gateway/v1/demandes/traitement", orange, map[string]any{"idDemande": id})
	h.converger()
	require.Equal(t, "ACTIVATION", h.etape(id))

	// 6. Activation par le destinataire.
	h.post("/api/gateway/v1/demandes/traitement", yas, map[string]any{"idDemande": id})
	h.converger()
	require.Equal(t, "CONFIRMATION", h.etape(id))

	// 7. Confirmation : la source, puis le tiers. Une seule ne suffit pas.
	h.post("/api/gateway/v1/demandes/a-confirmer", orange, map[string]any{"idDemande": id})
	h.converger()
	require.Equal(t, "CONFIRMATION", h.etape(id), "il manque la confirmation d'EXPRESSO")

	h.post("/api/gateway/v1/demandes/a-confirmer", expresso, map[string]any{"idDemande": id})
	h.converger()
	require.Equal(t, "COMPLETION", h.etape(id))

	// 8. Clôture par le destinataire.
	h.post("/api/gateway/v1/demandes/traitement", yas, map[string]any{"idDemande": id})
	h.converger()

	require.Equal(t, "TERMINE", h.statutDemande(id))

	// La demande apparaît des deux côtés.
	in := h.liste("/api/gateway/v1/demandes/in", yas)
	require.Len(t, in, 1)
	require.Equal(t, id, in[0].(map[string]any)["id"])

	out := h.liste("/api/gateway/v1/demandes/out", orange)
	require.Len(t, out, 1)

	// Le numéro a changé d'opérateur au registre national.
	require.Equal(t, seed.OperateurYAS, h.detenteur("771000001"))
}

// TestPortageParExpirationSansAucunAppel rejoue le portage n°2 du SIT : créé,
// laissé sans action, TERMINE après cinq expirations d'étape (ANO-006, TC-062).
func TestPortageParExpirationSansAucunAppel(t *testing.T) {
	h := nouveauHarnais(t, func(c *config.Config) {
		c.EtapeTimeout = 50 * time.Millisecond
	})

	yas := h.jeton("yas", "yas2026")
	h.post("/api/gateway/v1/otp/send", yas, map[string]any{"numero": "771000001"})
	_, corps := h.post("/api/gateway/v1/demandes/particulier", yas, corpsParticulier("771000001"))
	id := corps["data"].(map[string]any)["id"].(string)

	require.Eventually(t, func() bool {
		return h.statutDemande(id) == "TERMINE"
	}, 5*time.Second, 20*time.Millisecond)

	require.Equal(t, "EXPIRE", h.statutEtape(id))
	require.Equal(t, seed.OperateurYAS, h.detenteur("771000001"),
		"le numéro a changé d'opérateur alors qu'aucun HLR n'a été touché")
}

// TestMemeScenarioEnModeContrat vérifie que seule la présentation change.
func TestMemeScenarioEnModeContrat(t *testing.T) {
	h := nouveauHarnais(t, func(c *config.Config) { c.Fidelity = config.FidelityContract })

	yas := h.jeton("yas", "yas2026")
	orange := h.jeton("orange", "orange2026")
	expresso := h.jeton("expresso", "expresso2026")

	h.post("/api/gateway/v1/otp/send", yas, map[string]any{"numero": "771000001"})
	_, corps := h.post("/api/gateway/v1/demandes/particulier", yas, corpsParticulier("771000001"))
	id := corps["data"].(map[string]any)["id"].(string)

	h.post("/api/gateway/v1/demandes/acceptation", orange,
		map[string]any{"idDemande": id, "accepte": true})
	h.converger()
	h.post("/api/gateway/v1/demandes/traitement", orange, map[string]any{"idDemande": id})
	h.converger()
	h.post("/api/gateway/v1/demandes/traitement", yas, map[string]any{"idDemande": id})
	h.converger()
	h.post("/api/gateway/v1/demandes/a-confirmer", orange, map[string]any{"idDemande": id})
	h.post("/api/gateway/v1/demandes/a-confirmer", expresso, map[string]any{"idDemande": id})
	h.converger()
	h.post("/api/gateway/v1/demandes/traitement", yas, map[string]any{"idDemande": id})
	h.converger()

	require.Equal(t, "TERMINE", h.statutDemande(id))
}

// TestAucuneErreurNePorteDeCodeEnModeReel — ANO-001, vérifié en volume.
func TestAucuneErreurNePorteDeCodeEnModeReel(t *testing.T) {
	h := nouveauHarnais(t)
	yas := h.jeton("yas", "yas2026")
	orange := h.jeton("orange", "orange2026")

	inconnu := "6a0000000000000000000000"
	appels := []struct {
		chemin string
		jeton  string
		corps  any
	}{
		{"/api/gateway/v1/demandes/traitement", yas, map[string]any{"idDemande": inconnu}},
		{"/api/gateway/v1/demandes/acceptation", orange, map[string]any{"idDemande": inconnu, "accepte": true}},
		{"/api/gateway/v1/demandes/a-confirmer", orange, map[string]any{"idDemande": inconnu}},
		{"/api/gateway/v1/demandes/" + inconnu + "/annuler", yas, nil},
		{"/api/gateway/v1/demandes/restitution", orange, map[string]any{"numero": "771000001"}},
		{"/api/gateway/v1/demandes/restitution", orange, map[string]any{"numero": "774000001"}},
		{"/api/gateway/v1/reverse-requests", yas, map[string]any{"numero": "773000001"}},
		{"/api/gateway/v1/otp/verify", yas, map[string]any{"numero": "779999999", "otpCode": "123456"}},
	}

	for _, a := range appels {
		rep, corps := h.postBrut(a.chemin, a.jeton, a.corps)
		require.GreaterOrEqualf(t, rep.StatusCode, 400, a.chemin)
		require.NotContainsf(t, corps, "code", "%s ne doit porter aucun champ code", a.chemin)
		require.NotContainsf(t, corps, "success", "%s ne doit pas être enveloppée", a.chemin)
		require.Containsf(t, corps, "type", "%s doit être un problem+json", a.chemin)
	}
}
```

Le paquet `test` est un paquet distinct : `corpsParticulier`, `nouveauHarnais`, `post`, `postBrut`, `liste`, `converger`, `etape`, `statutDemande`, `statutEtape`, `jeton` et `detenteur` y sont **réécrits localement** — les helpers de `internal/api` sont dans des fichiers `_test.go` et ne s'exportent pas. Reprendre le harnais de la Task 9 tel quel, en ajoutant `statutEtape` (lit `statut_etape_actuel`), `detenteur` (lit `numero.operateur_actuel_id`), et `postBrut`, qui rend la réponse et son corps décodé sans exiger un statut de succès.

Le test `TestPortageParExpirationSansAucunAppel` est le seul qui laisse tourner le ticker : il construit le harnais avec `EngineTick` court et lance `go h.moteur.Run(ctx)`.

- [ ] **Step 2 : Lancer les tests, vérifier qu'ils passent**

Run: `make test`
Expected: PASS — les quatre scénarios de bout en bout, et la totalité de la suite.

- [ ] **Step 3 : Écrire la collection Postman**

Create `postman/numflex-sandbox.postman_collection.json` — même arborescence que celle de l'ARTP : un dossier `authentication` (les deux routes) et un dossier `gateway` avec les 33 requêtes, groupées par section du guide. Chaque requête utilise `{{baseUrl}}` et le dossier `gateway` porte un `auth` de type `bearer` sur `{{token}}`. Un script de test sur `POST /api/authenticate` enregistre automatiquement `token` :

```javascript
pm.environment.set("token", pm.response.json().id_token);
```

Create `postman/numflex-sandbox.postman_environment.json` — variables `baseUrl` = `http://localhost:8080` et `token` vide. Basculer entre le sandbox et la recette ARTP ne demande alors que de changer `baseUrl`.

- [ ] **Step 4 : Écrire le README**

Create `README.md` couvrant :

- ce que le sandbox est et ce qu'il n'est pas ;
- `docker compose up` puis les trois comptes et le vivier de numéros par tranche ;
- le tableau des variables d'environnement du §11 de la spec, avec la mention explicite que **`FIDELITY=real` reproduit les anomalies de la recette et que c'est voulu** ;
- la liste des anomalies reproduites, avec leur identifiant SIT — ANO-001, ANO-002, ANO-003, ANO-004, ANO-005, ANO-006, ANO-008, ANO-011, ANO-014, ANO-015, ANO-016, ANO-018, ANO-019, ANO-020, ANO-021, ANO-022 ;
- les trois `[HYP]` : préfixe de routage EXPRESSO, statut `REJETE`, répartition des rôles en restitution ;
- le profil CI et la commande `make test` ;
- l'usage du binaire `artp` pour valider un reverse ;
- le rappel que l'OTP est **statique** (`123456`) et qu'aucun SMS n'est envoyé.

- [ ] **Step 5 : Vérifier la suite complète et le périmètre des routes**

Run: `make test && go vet ./...`
Expected: PASS, aucun avertissement.

Vérifier manuellement que `internal/api/router.go` ne déclare aucune route absente du §4 de la spec — c'est la garantie de la contrainte D-4.

- [ ] **Step 6 : Commit**

```bash
git add -A
git commit -m "feat: scenarios de bout en bout, collection Postman et README"
```

---

## Ordre d'exécution et dépendances

```
T1 socle
 └─ T2 schéma ──┬─ T3 seed
                └─ T4 erreurs ── T5 auth/routeur ──┬─ T6 référentiels
                                                    ├─ T7 OTP
                                                    └─ T8 domaine ── T9 moteur ──┬─ T10 particulier ── T11 flotte
                                                                                  ├─ T12 restitution
                                                                                  ├─ T13 consultation ──┬─ T14 acceptation
                                                                                  │                      ├─ T15 confirmation
                                                                                  │                      ├─ T16 traitement
                                                                                  │                      └─ T17 annulation
                                                                                  ├─ T18 incidents
                                                                                  └─ T19 reverse ── T20 bout en bout
```

T6, T7 et T8 sont indépendantes entre elles et parallélisables. T14 à T17 le sont aussi une fois T13 livrée.
