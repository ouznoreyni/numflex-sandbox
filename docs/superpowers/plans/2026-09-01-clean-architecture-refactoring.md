# Clean Architecture Refactoring — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure the NumFlex sandbox into canonical Clean Architecture — entities, use cases, interface adapters, frameworks — with English identifiers and comments, without changing a single observable byte of the ARTP contract.

**Architecture:** Four layers with an enforced dependency rule. Interactors depend on interfaces declared in the use case layer (`port`), implemented by Postgres gateways in the adapter layer. The two fidelity modes become two presenters. A transaction spanning several aggregates goes through `port.UnitOfWork`.

**Tech Stack:** Go 1.25, Gin, pgx v5, golang-migrate, testify, PostgreSQL 16.

**Spec:** `docs/superpowers/specs/2026-09-01-clean-architecture-refactoring-design.md`

## Global Constraints

- **`make test` is the only verification command that counts.** Plain `go test ./...` skips 119 tests when `DATABASE_URL` is unset and still prints `ok`. Never claim green from `go test ./...` alone.
- **Never modify `conformite_captures_test.go`** during Tasks 4–18. It freezes real captured platform responses and is the proof the contract has not moved. It is renamed only in Task 21.
- **Three vocabularies stay French, verbatim:** route paths (`/api/gateway/v1/demandes/a-accepter`), JSON field names and values (`{"idDemande":…,"etapeActuelle":"ACCEPTATION"}`), SQL tables and columns (`demande.etape_actuelle`).
- **String constant values never change.** `entity.StepAcceptance` must still equal `"ACCEPTATION"`.
- **Comments become English** but keep every `ANO-0xx`, `R-10`, `TC-0xx`, `[HYP]` reference and every guide section number (`§7.3`) exactly as written.
- **Module path:** `github.com/ouznoreyni/numflex-sandbox`.
- **Every task ends with a commit** whose author is `ouznoreyni`, with no `Co-Authored-By` trailer.
- **After every task:** `gofmt -l .` must be empty and `go vet ./...` silent.

---

## File Structure

| Path | Responsibility |
|---|---|
| `internal/entity/` | Enterprise rules. Imports nothing from this module. |
| `internal/usecase/port/` | Gateway interfaces, `UnitOfWork`, service interfaces. |
| `internal/usecase/<capability>/` | One file per use case: input model, output model, boundary, interactor. |
| `internal/adapter/controller/` | Gin decoding → input model → boundary call. |
| `internal/adapter/presenter/` | Output model → view model. `real.go`, `contract.go`. |
| `internal/adapter/gateway/postgres/` | Gateway implementations. Only place that names French columns. |
| `internal/framework/web/` | Gin engine, router, middlewares. |
| `internal/framework/persistence/` | pgx pool, migrations, `UnitOfWork` implementation. |
| `internal/testsupport/` | Fixtures, test database, in-memory port doubles. |
| `test/` | End-to-end scenarios and the dependency-rule guard. |

---

## Phase 1 — Foundation

### Task 1: Dependency-rule guard

The guard comes first so that every later task is checked by it from the moment it lands.

**Files:**
- Create: `test/architecture_test.go`

**Interfaces:**
- Produces: `TestDependencyRule` — fails when a package imports a more outward layer.

- [ ] **Step 1: Write the failing test**

```go
package test

import (
	"os/exec"
	"strings"
	"testing"
)

const module = "github.com/ouznoreyni/numflex-sandbox"

// layerOf maps a package path to its Clean Architecture layer number.
// Lower is more inward. A package may only import packages with a
// number less than or equal to its own.
func layerOf(pkg string) (int, bool) {
	switch {
	case strings.HasPrefix(pkg, module+"/internal/entity"):
		return 0, true
	case strings.HasPrefix(pkg, module+"/internal/usecase"):
		return 1, true
	case strings.HasPrefix(pkg, module+"/internal/adapter"):
		return 2, true
	case strings.HasPrefix(pkg, module+"/internal/framework"),
		strings.HasPrefix(pkg, module+"/cmd"):
		return 3, true
	}
	return 0, false
}

func TestDependencyRule(t *testing.T) {
	out, err := exec.Command("go", "list",
		"-f", "{{.ImportPath}} {{join .Imports \" \"}}", "../...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pkg := fields[0]
		pkgLayer, known := layerOf(pkg)
		if !known {
			continue
		}
		for _, imp := range fields[1:] {
			impLayer, known := layerOf(imp)
			if !known {
				continue
			}
			if impLayer > pkgLayer {
				t.Errorf("dependency rule violated: %s (layer %d) imports %s (layer %d)",
					pkg, pkgLayer, imp, impLayer)
			}
		}
	}
}

// TestEntityIsPure asserts the innermost layer imports nothing from this module.
func TestEntityIsPure(t *testing.T) {
	out, err := exec.Command("go", "list",
		"-f", "{{.ImportPath}} {{join .Imports \" \"}}", "../internal/entity/...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		for _, imp := range fields[1:] {
			if strings.HasPrefix(imp, module+"/") {
				t.Errorf("entity must import nothing from this module, found %s", imp)
			}
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./test/ -run 'TestDependencyRule|TestEntityIsPure' -v`
Expected: FAIL — `go list ../internal/entity/...` errors because the package does not exist yet.

- [ ] **Step 3: Create the entity package so the guard has something to check**

Move `internal/domain/demande.go` to `internal/entity/porting_request.go`, keeping every constant *value* and translating only identifiers:

```go
// Package entity holds the enterprise rules of number portability. It knows
// nothing about HTTP, SQL, or the fidelity mode: a business rule is the same
// in both modes, only its rendering differs.
package entity

type Step string

const (
	StepAcceptance    Step = "ACCEPTATION"
	StepDeactivation  Step = "DESACTIVATION"
	StepActivation    Step = "ACTIVATION"
	StepConfirmation  Step = "CONFIRMATION"
	StepCompletion    Step = "COMPLETION"
)

type RequestStatus string

const (
	RequestInProgress RequestStatus = "EN_COURS"
	RequestCompleted  RequestStatus = "TERMINE"
	RequestCancelled  RequestStatus = "ANNULE"
	// RequestRejected — [HYP] neither documented in the guide nor observed in SIT.
	RequestRejected RequestStatus = "REJETE"
)

type StepStatus string

const (
	StepInProgress StepStatus = "EN_COURS"
	StepCompleted  StepStatus = "TERMINE"
	StepExpired    StepStatus = "EXPIRE"
)

type RequestType string

const (
	RequestTypePorting     RequestType = "PORTAGE"
	RequestTypeRestitution RequestType = "RESTITUTION"
	RequestTypeReverse     RequestType = "REVERSE"
)

type SubscriberType string

const (
	SubscriberIndividual SubscriberType = "PARTICULIER"
	SubscriberEnterprise SubscriberType = "ENTREPRISE"
)

type Role int

const (
	RoleSource Role = iota
	RoleRecipient
	RoleAll
	RoleARTP
)

type PortingRequest struct {
	ID                string
	MSISDN            string
	RequestType       RequestType
	SubscriberType    SubscriberType
	Status            RequestStatus
	CurrentStep       Step
	CurrentStepStatus StepStatus
	SourceOperatorID    string
	RecipientOperatorID string
	CreatorOperatorID   string
	// PendingTransition is true between the processing of a step and its
	// effective convergence (R-10).
	PendingTransition bool
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./test/ -run 'TestDependencyRule|TestEntityIsPure' -v`
Expected: PASS — `internal/entity` exists and imports nothing from the module.

- [ ] **Step 5: Commit**

```bash
git add test/architecture_test.go internal/entity/porting_request.go
git commit -m "test: le test qui rend la regle de dependance verifiable"
```

---

### Task 2: Move the rest of the domain into `entity`

`internal/domain` currently returns `*apperr.Error`, so `apperr` moves in the same task — otherwise `entity` would import an outside package and Task 1's guard would fail.

**Files:**
- Create: `internal/entity/step.go`, `internal/entity/eligibility.go`, `internal/entity/authorization.go`, `internal/entity/fault.go`, `internal/entity/fault_catalog.go`
- Delete: `internal/domain/`, `internal/apperr/`
- Test: `internal/entity/step_test.go`, `internal/entity/eligibility_test.go`

**Interfaces:**
- Consumes: `entity.PortingRequest`, `entity.Step` from Task 1.
- Produces:
  - `func NextStep(s Step) (Step, bool)`
  - `func StepOwner(s Step, rt RequestType) Role`
  - `func StepEndpoint(s Step) string`
  - `func CanProcess(r PortingRequest, operatorID string) *Fault`
  - `func CanAccept(r PortingRequest, operatorID string) *Fault`
  - `func CanCancel(r PortingRequest, operatorID string) *Fault`
  - `func ExpectedConfirmers(r PortingRequest, allOperators []string) []string`
  - `type Fault struct { Code, Message string; Kind FaultKind; Fields []FieldFault; RealDetail string }`
  - `type FaultKind int` with `FaultValidation`, `FaultState`, `FaultAccess`, `FaultNotFound`, `FaultInternal`
  - `type Caller struct { UserID, Username, OperatorID, OperatorName string }` — the operator behind a token, translated from `api.Identite`

- [ ] **Step 1: Rename the existing tests first, and run them**

Move `internal/domain/etapes_test.go` → `internal/entity/step_test.go` and `internal/domain/eligibilite_test.go` → `internal/entity/eligibility_test.go`, translating identifiers only. Keep all 22 test functions and every assertion value.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/entity/ -v`
Expected: FAIL — `undefined: NextStep`, `undefined: CanProcess`, etc.

- [ ] **Step 3: Move the implementations**

Translate `internal/domain/etapes.go` and `eligibilite.go` into `internal/entity/step.go`, `authorization.go`, `eligibility.go`. Translate `internal/apperr/apperr.go` and `catalogue.go` into `internal/entity/fault.go` and `fault_catalog.go`, renaming the type `Error` → `Fault`, `Kind` → `FaultKind`, `FieldError` → `FieldFault`. Constructor names in the catalog (`OTPInvalid`, `OTPExpired`, …) are already English — keep them.

- [ ] **Step 4: Run the full suite**

Run: `make test`
Expected: PASS — all 200 test functions. Everything still compiles because `internal/api` now imports `entity` instead of `domain` and `apperr`.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: le domaine devient la couche entity, en anglais"
```

---

### Task 3: Declare the ports

**Files:**
- Create: `internal/usecase/port/gateway.go`, `internal/usecase/port/unit_of_work.go`, `internal/usecase/port/service.go`
- Create: `internal/testsupport/inmemory/otp_gateway.go`, `internal/testsupport/database.go`
- Modify: `Makefile`
- Test: `internal/usecase/port/port_test.go`

**Interfaces:**
- Produces: the interfaces every later task implements or consumes.

- [ ] **Step 1: Write the compile-time conformance test**

```go
package port_test

import (
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport/inmemory"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// TestInMemoryDoublesSatisfyPorts fails to compile if a double drifts from
// its interface. It is the cheapest guard against a half-updated port.
func TestInMemoryDoublesSatisfyPorts(t *testing.T) {
	var _ port.OTPGateway = (*inmemory.OTPGateway)(nil)
	var _ port.Clock = inmemory.FixedClock{}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/usecase/port/`
Expected: FAIL — package `port` does not exist.

- [ ] **Step 3: Write the ports and the first double**

```go
// Package port declares what the use case layer needs from the outside world.
// Every interface here is implemented in the adapter layer; nothing in this
// package may name pgx, Gin, or any concrete technology.
package port

import (
	"context"
	"time"

	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
)

// OneTimePassword is the persisted state of an OTP challenge.
type OneTimePassword struct {
	MSISDN    string
	Code      string
	ExpiresAt time.Time
	Attempts  int
	Consumed  bool
}

type OTPGateway interface {
	Upsert(ctx context.Context, otp OneTimePassword) error
	Find(ctx context.Context, msisdn string) (OneTimePassword, bool, error)
	IncrementAttempts(ctx context.Context, msisdn string) error
	Consume(ctx context.Context, msisdn string) error
}

// UserGateway resolves the operator behind a set of credentials, and behind a
// username carried by a token. Both the login use case and the authentication
// middleware go through it.
type UserGateway interface {
	ByCredentials(ctx context.Context, username, password string) (entity.Caller, bool, error)
	ByUsername(ctx context.Context, username string) (entity.Caller, bool, error)
}

// Clock is injected so that CLOCK_SKEW_SECONDS and test determinism have a
// single seam. Now is the real instant; Rendered applies the configured skew
// and is used only when a timestamp leaves through a presenter.
type Clock interface {
	Now() time.Time
	Rendered(t time.Time) time.Time
}

type IDGenerator interface {
	NewID() string
}

// Engine is the part of platform behaviour that calls do not drive.
type Engine interface {
	MarketFrozen(ctx context.Context) (bool, error)
	ScheduleTransition(ctx context.Context, requestID string) error
}
```

```go
package port

import "context"

// Repositories is the set of gateways available inside one transaction.
// Gateways obtained here are bound to that transaction; gateways injected
// directly into an interactor are not.
type Repositories struct {
	OTP OTPGateway
	// Requests, Numbers, Reverse, Incidents… are added by later tasks.
}

// UnitOfWork owns the transaction boundary. The interactor decides that there
// is a transaction; the adapter decides what a transaction is. No pgx.Tx ever
// reaches this layer.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(Repositories) error) error
}
```

Write `internal/testsupport/inmemory/otp_gateway.go` with a map-backed `OTPGateway` and a `FixedClock`.

- [ ] **Step 4: Move the test database helper and teach `make test` about the build tag**

Move `internal/store/testdb.go` to `internal/testsupport/database.go`, renaming `NewTestDB` unchanged in behaviour (migrate, truncate, seed).

**This step is mandatory and easy to forget:** the moment a test carries
`//go:build integration`, `go test` stops compiling it unless the tag is passed.
Without this change the 119 database tests would silently vanish from `make test`
while it kept printing `ok` — the exact failure mode the Global Constraints warn
about. Change the `test` target:

```makefile
test: up
	DATABASE_URL="$(DB_TEST)" \
	ETAPE_TIMEOUT_SECONDS=0 CONVERGENCE_MIN_SECONDS=0 CONVERGENCE_MAX_SECONDS=0 \
	COMPLETION_LATENCY_MS=0 CLOCK_SKEW_SECONDS=0 \
	go test -tags=integration ./... -p 1 -count=1

# Unit tests only — no database, no Docker, runs in seconds.
test-unit:
	go test ./... -count=1
```

- [ ] **Step 5: Verify both targets**

Run: `make test` — PASS, and the count of run tests is unchanged from before this task.
Run: `make test-unit` — PASS, and it completes in under five seconds.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat: les ports, leurs doubles en memoire, et make test qui compile les tests d integration"
```

---

### Task 4: Presenters

`httpx.Renderer` already branches on fidelity in `failReel` and `failContrat`. This task turns that branch into two types behind one interface.

**Files:**
- Create: `internal/adapter/presenter/presenter.go`, `real.go`, `contract.go`, `view_model.go`
- Test: `internal/adapter/presenter/real_test.go`, `contract_test.go` (move the 15 assertions of `internal/httpx/renderer_test.go`)
- Delete at the end of Task 18: `internal/httpx/`

**Interfaces:**
- Produces:
  - `type ViewModel struct { Status int; Body any }`
  - `type Presenter interface { Success(status int, message string, data any) ViewModel; SuccessWithoutData(status int, message string) ViewModel; Failure(f *entity.Fault) ViewModel }`
  - `func NewReal(clock port.Clock) *Real`
  - `func NewContract(clock port.Clock) *Contract`

- [ ] **Step 1: Move the renderer tests, converted to view models**

The current tests assert on a `httptest.ResponseRecorder`. Convert each to assert on the returned `ViewModel` instead — same expected status, same expected body. Keep all 15 test functions.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/adapter/presenter/`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement both presenters**

Port `Renderer.OK`, `OKSansData`, `failReel`, `failContrat` verbatim in behaviour: same JSON shapes, same `RuntimeException: ` prefix, same `EnvelopeSansData` omission for ANO-011.

- [ ] **Step 4: Run to verify they pass**

Run: `make test`
Expected: PASS — `internal/httpx` still exists and is still used by `internal/api`; the presenters are additive at this point.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: deux presentateurs pour les deux modes de fidelite"
```

---

### Task 5: Persistence framework and the unit of work

**Files:**
- Create: `internal/framework/persistence/pool.go`, `migrate.go`, `unit_of_work.go`
- Test: `internal/framework/persistence/unit_of_work_test.go`

**Interfaces:**
- Consumes: `port.UnitOfWork`, `port.Repositories` from Task 3.
- Produces:
  - `func Open(ctx context.Context, url string) (*Pool, error)`
  - `func (p *Pool) Close()`
  - `func Migrate(url string) error`
  - `func MigrationsDir() (string, error)`
  - `func NewUnitOfWork(p *Pool) port.UnitOfWork`

- [ ] **Step 1: Write the failing test**

```go
//go:build integration

package persistence_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ouznoreyni/numflex-sandbox/internal/testsupport"
	"github.com/ouznoreyni/numflex-sandbox/internal/usecase/port"
)

// TestUnitOfWorkRollsBack proves the transaction boundary is real: a failure
// inside Do leaves nothing behind. Without this, "one transaction" is a claim
// rather than a guarantee.
func TestUnitOfWorkRollsBack(t *testing.T) {
	db := testsupport.NewTestDB(t)
	uow := persistence.NewUnitOfWork(db)

	boom := errors.New("boom")
	err := uow.Do(context.Background(), func(r port.Repositories) error {
		if err := r.OTP.Upsert(context.Background(), port.OneTimePassword{
			MSISDN: "771000001", Code: "123456",
		}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom, got %v", err)
	}

	// Read back outside any transaction, through a gateway bound to the pool.
	_, found, err := postgres.NewOTPGateway(db.Pool).Find(context.Background(), "771000001")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("the OTP survived a rolled-back transaction")
	}
}
```

Note the ordering constraint: this test reads back through
`postgres.NewOTPGateway`, produced by Task 7. Implement Task 7 first, or write
this test's read-back against `db.Pool.QueryRow` directly and convert it in Task 7.

- [ ] **Step 2: Run to verify it fails**

Run: `make test`
Expected: FAIL — package `persistence` does not exist.

- [ ] **Step 3: Implement**

Move `internal/store/db.go` into `pool.go` and `migrate.go`, translating `RepertoireMigrations` → `MigrationsDir`. Implement `unitOfWork.Do` with `pool.BeginTx`, constructing a `port.Repositories` whose gateways hold the `pgx.Tx`, committing on nil error and rolling back otherwise.

- [ ] **Step 4: Run to verify it passes**

Run: `make test`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: pool, migrations et unite de travail dans la couche framework"
```

---

### Task 6: Web framework and middlewares

**Files:**
- Create: `internal/framework/web/engine.go`, `router.go`, `middleware/authentication.go`, `middleware/cors.go`
- Move: `internal/config/` → `internal/framework/config/`, `internal/auth/` → `internal/framework/token/`, `internal/oid/` → `internal/framework/identifier/`, `internal/seed/` → `internal/framework/seed/`
- Test: `internal/framework/web/middleware/authentication_test.go`, `cors_test.go`

**Interfaces:**
- Produces:
  - `func NewEngine(cfg *config.Config, mw ...gin.HandlerFunc) *gin.Engine`
  - `func Authenticate(secret string, users port.UserGateway) gin.HandlerFunc`
  - `func CallerFrom(c *gin.Context) entity.Caller`
  - `func AllowCORS(origins []string) gin.HandlerFunc`

- [ ] **Step 1: Move the CORS tests**

Move the assertions of `internal/api/cors_test.go` verbatim, renaming identifiers only. They already cover the empty-origins silence and the `*` default.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/framework/web/...`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement, and move the four leaf packages**

Port `internal/api/cors.go` and the authentication middleware. **Keep the prefix guard exactly as `router.go` documents it**: the whole-segment comparison against `/api/gateway/v1` that makes a non-contract path under the prefix return 401 rather than 404, mirroring a Spring Security filter.

Move the four remaining leaf packages into the framework layer, translating identifiers:

| From | To | Renames |
|---|---|---|
| `internal/config` | `internal/framework/config` | `ChargerFichierEnv` → `LoadEnvFile`, `AppliquerArguments` → `ApplyArguments`, `CheminFichierEnv` → `EnvFilePath` |
| `internal/auth` | `internal/framework/token` | `Emettre` → `Issue`, `Verifier` → `Verify` |
| `internal/oid` | `internal/framework/identifier` | implements `port.IDGenerator` |
| `internal/seed` | `internal/framework/seed` | `Run` unchanged, `seedNumeros` → `seedNumbers` |

`config.Config` field names are already English and do not change; only the two
env-loading functions and their comments do. The 14 config tests and 5 seed tests
move with their packages.

- [ ] **Step 4: Run to verify they pass**

Run: `make test`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: moteur Gin, routeur et middlewares dans la couche framework"
```

---

## Phase 2 — The pattern-setting capability

### Task 7: OTP gateway

**Files:**
- Create: `internal/adapter/gateway/postgres/otp_gateway.go`, `mapping.go`
- Test: `internal/adapter/gateway/postgres/otp_gateway_test.go` (build tag `integration`)

**Interfaces:**
- Consumes: `port.OTPGateway`, `port.OneTimePassword` from Task 3.
- Produces: `func NewOTPGateway(q Querier) port.OTPGateway`, where `type Querier interface { Exec(...); Query(...); QueryRow(...) }` is satisfied by both `*pgxpool.Pool` and `pgx.Tx`.

- [ ] **Step 1: Write the failing test**

```go
//go:build integration

package postgres_test

func TestOTPGatewayUpsertResetsAttempts(t *testing.T) {
	db := testsupport.NewTestDB(t)
	g := postgres.NewOTPGateway(db.Pool)
	ctx := context.Background()

	must(t, g.Upsert(ctx, port.OneTimePassword{
		MSISDN: "771000001", Code: "123456",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}))
	must(t, g.IncrementAttempts(ctx, "771000001"))

	// A second send must reset attempts and the consumed flag: the ON CONFLICT
	// clause is what makes a number re-challengeable.
	must(t, g.Upsert(ctx, port.OneTimePassword{
		MSISDN: "771000001", Code: "123456",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}))

	got, found, err := g.Find(ctx, "771000001")
	must(t, err)
	if !found {
		t.Fatal("otp not found")
	}
	if got.Attempts != 0 || got.Consumed {
		t.Fatalf("upsert did not reset: attempts=%d consumed=%v", got.Attempts, got.Consumed)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `make test`
Expected: FAIL — `undefined: postgres.NewOTPGateway`

- [ ] **Step 3: Implement**

Move the four SQL statements out of `internal/api/otp.go` — the `INSERT … ON CONFLICT (numero) DO UPDATE`, the `SELECT code, expire_a, tentatives, consomme`, the `UPDATE otp SET tentatives = tentatives + 1`, and the `UPDATE otp SET consomme = true` — verbatim, into the four gateway methods. Add the column mapping to `mapping.go` as a documented comment block.

- [ ] **Step 4: Run to verify it passes**

Run: `make test`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: le SQL de l'OTP quitte le gestionnaire pour un gateway"
```

---

### Task 8: OTP interactors

These are the project's first true unit tests — no Postgres.

**Files:**
- Create: `internal/usecase/otp/send_otp.go`, `verify_otp.go`
- Test: `internal/usecase/otp/send_otp_test.go`, `verify_otp_test.go`

**Interfaces:**
- Consumes: `port.OTPGateway`, `port.Clock`, `inmemory.OTPGateway`, `inmemory.FixedClock`.
- Produces:
  - `type SendOTPInput struct { MSISDN string }`
  - `type SendOTPBoundary interface { Execute(context.Context, SendOTPInput) error }`
  - `func NewSendOTP(g port.OTPGateway, c port.Clock, code string, ttl time.Duration) *SendOTPInteractor`
  - `type VerifyOTPInput struct { MSISDN, Code string }`
  - `func NewVerifyOTP(g port.OTPGateway, c port.Clock, maxAttempts int) *VerifyOTPInteractor`
  - `func (i *VerifyOTPInteractor) Execute(context.Context, VerifyOTPInput) *entity.Fault`

- [ ] **Step 1: Write the failing tests**

```go
package otp_test

// TestVerifyDoesNotConsume pins TC-021: a successful verification leaves the
// code usable, so that creating the request afterwards still works. Only
// failed attempts are counted.
func TestVerifyDoesNotConsume(t *testing.T) {
	g := inmemory.NewOTPGateway()
	clock := inmemory.FixedClock{At: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
	g.Seed(port.OneTimePassword{
		MSISDN: "771000001", Code: "123456",
		ExpiresAt: clock.Now().Add(5 * time.Minute),
	})

	i := otp.NewVerifyOTP(g, clock, 3)
	if f := i.Execute(context.Background(), otp.VerifyOTPInput{
		MSISDN: "771000001", Code: "123456"}); f != nil {
		t.Fatalf("expected success, got %v", f)
	}

	stored, _, _ := g.Find(context.Background(), "771000001")
	if stored.Consumed {
		t.Fatal("verification consumed the code; TC-021 says it must not")
	}
}

// TestVerifyCountsOnlyFailures pins the three-attempt limit.
func TestVerifyCountsOnlyFailures(t *testing.T) {
	g := inmemory.NewOTPGateway()
	clock := inmemory.FixedClock{At: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)}
	g.Seed(port.OneTimePassword{
		MSISDN: "771000001", Code: "123456",
		ExpiresAt: clock.Now().Add(5 * time.Minute),
	})
	i := otp.NewVerifyOTP(g, clock, 3)

	for n := 1; n <= 3; n++ {
		if f := i.Execute(context.Background(), otp.VerifyOTPInput{
			MSISDN: "771000001", Code: "000000"}); f.Code != "OTP_INVALID" {
			t.Fatalf("attempt %d: expected OTP_INVALID, got %v", n, f)
		}
	}
	if f := i.Execute(context.Background(), otp.VerifyOTPInput{
		MSISDN: "771000001", Code: "123456"}); f == nil || f.Code != "OTP_MAX_ATTEMPTS" {
		t.Fatalf("expected OTP_MAX_ATTEMPTS after three failures, got %v", f)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/usecase/otp/`
Expected: FAIL — `undefined: otp.NewVerifyOTP`

- [ ] **Step 3: Implement**

Move the body of `Deps.verifierOTP` into `VerifyOTPInteractor.Execute`, replacing `d.DB.Pool` calls with gateway calls and `time.Now()` with `clock.Now()`. Order of checks stays exactly as today — consumed, then max attempts, then expiry, then code mismatch — because the tests pin it.

- [ ] **Step 4: Run to verify they pass**

Run: `go test ./internal/usecase/otp/ -v` then `make test`
Expected: PASS, and the OTP unit tests run in milliseconds without a database.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: les interactors OTP, testes sans base"
```

---

### Task 9: OTP controller, and the old handler deleted

**Files:**
- Create: `internal/adapter/controller/otp_controller.go`
- Modify: `internal/framework/web/router.go`
- Delete: `internal/api/otp.go`, `internal/api/otp_test.go`
- Test: the existing HTTP-level assertions move to `internal/adapter/controller/otp_controller_test.go`

**Interfaces:**
- Consumes: `otp.SendOTPBoundary`, `otp.VerifyOTPBoundary`, `presenter.Presenter`.
- Produces: `func NewOTPController(send otp.SendOTPBoundary, verify otp.VerifyOTPBoundary, p presenter.Presenter) *OTPController` with methods `Send(*gin.Context)` and `Verify(*gin.Context)`.

- [ ] **Step 1: Move the HTTP tests**

Move the 8 test functions of `internal/api/otp_test.go`, keeping every asserted status code and body. They still exercise the real router, so they keep the `integration` tag.

- [ ] **Step 2: Run to verify they fail**

Run: `make test`
Expected: FAIL — the controller does not exist.

- [ ] **Step 3: Implement**

The controller does three things and nothing else: bind JSON, validate the `^[0-9]{9}$` MSISDN pattern producing the same `otpSendDTO` field error, call the boundary, hand the result to the presenter. The `motifMSISDN` regexp moves here unchanged.

- [ ] **Step 4: Run to verify it passes**

Run: `make test`
Expected: PASS — including `conformite_captures_test.go`, untouched.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: la capacite OTP passe en clean architecture de bout en bout"
```

---

## Phase 3 — The remaining capabilities

Each task below follows the shape proven by Tasks 7–9: gateway with its integration test, interactors with unit tests against in-memory doubles, controller with the moved HTTP tests, old file deleted. Every task ends with `make test` green and a commit.

### Task 10: Authentication

The two `/api/authenticate` routes are use cases, not middleware: the middleware
of Task 6 *verifies* a token, this task *issues* one.

**Files:**
- Create: `internal/adapter/gateway/postgres/user_gateway.go`, `internal/usecase/auth/authenticate.go`, `describe_caller.go`, `internal/adapter/controller/auth_controller.go`
- Modify: `internal/framework/web/router.go`
- Delete: `internal/api/auth.go`, `internal/api/auth_test.go`
- Test: `internal/usecase/auth/authenticate_test.go`, `internal/adapter/controller/auth_controller_test.go`

**Interfaces:**
- Consumes: `port.UserGateway` (Task 3), `token.Issue` (Task 6), `entity.Caller` (Task 2).
- Produces:
  - `type AuthenticateInput struct { Username, Password string }`
  - `type AuthenticateOutput struct { Token string }`
  - `type AuthenticateBoundary interface { Execute(context.Context, AuthenticateInput) (AuthenticateOutput, *entity.Fault) }`
  - `func NewAuthenticate(users port.UserGateway, issue TokenIssuer, ttl time.Duration) *AuthenticateInteractor`
  - `type TokenIssuer func(username string, roles []string) (string, error)`
  - `func NewDescribeCaller() *DescribeCallerInteractor`

- [ ] **Step 1: Write the failing unit test**

```go
package auth_test

// TestAuthenticateRejectsUnknownUser pins that a bad login yields the platform's
// own 401 shape rather than a Go error leaking through.
func TestAuthenticateRejectsUnknownUser(t *testing.T) {
	users := inmemory.NewUserGateway()
	i := auth.NewAuthenticate(users, func(string, []string) (string, error) {
		t.Fatal("token must not be issued for an unknown user")
		return "", nil
	}, 24*time.Hour)

	_, fault := i.Execute(context.Background(), auth.AuthenticateInput{
		Username: "ghost", Password: "whatever"})
	if fault == nil {
		t.Fatal("expected a fault for an unknown user")
	}
	if fault.Kind != entity.FaultAccess {
		t.Fatalf("expected an access fault, got kind %v", fault.Kind)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/usecase/auth/`
Expected: FAIL — `undefined: auth.NewAuthenticate`

- [ ] **Step 3: Implement**

Move the two SQL statements of `internal/api/auth.go` into `user_gateway.go`.
`AuthenticateInteractor` resolves the caller, then calls the injected `TokenIssuer`;
it never imports the JWT library, which is what keeps the use case layer clean.
`DescribeCallerInteractor` is a pure projection of `entity.Caller` — no gateway.

- [ ] **Step 4: Run to verify it passes**

Run: `make test`
Expected: PASS — the moved HTTP tests still assert the same token shape and the
same 401 body.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: l'authentification en clean architecture"
```

---

### Task 11: Reference data

**Files:**
- Create: `internal/adapter/gateway/postgres/reference_gateway.go`, `internal/usecase/reference/{list_operators,list_rejection_reasons,list_request_types,list_processes,list_incident_types}.go`, `internal/adapter/controller/reference_controller.go`
- Delete: `internal/api/referentiels.go`, `internal/api/referentiels_test.go`

**Interfaces:**
- Produces: `port.ReferenceGateway` with `Operators`, `RejectionReasons`, `RequestTypes`, `Processes`, `IncidentTypes`, each `func(ctx) ([]entity.X, error)`.

- [ ] **Step 1:** Move the 6 test functions of `referentiels_test.go` to the controller test file.
- [ ] **Step 2:** Run `make test` — FAIL, controller undefined.
- [ ] **Step 3:** Move the 5 SELECT statements of `referentiels.go` into the gateway; each interactor is a pass-through with no rule, which is correct and must not be padded.
- [ ] **Step 4:** Run `make test` — PASS.
- [ ] **Step 5:** `git commit -m "refactor: les referentiels en clean architecture"`

### Task 12: Read models

**Files:**
- Create: `internal/adapter/gateway/postgres/query_gateway.go`, `internal/usecase/query/*.go` (7 files), `internal/adapter/controller/query_controller.go`
- Delete: `internal/api/demandes_lecture.go` (408 lines), `internal/api/lecture_test.go`

**Interfaces:**
- Produces: `port.QueryGateway` with one method per queue — `ToAccept`, `ToProcess`, `ToConfirm`, `AlreadyConfirmed`, `Incoming`, `Outgoing`, `Own` — plus `ByID(ctx, id, queue) (entity.PortingRequest, bool, error)`.

- [ ] **Step 1:** Move the 12 test functions of `lecture_test.go`.
- [ ] **Step 2:** Run `make test` — FAIL.
- [ ] **Step 3:** Move the 6 SELECT statements verbatim. The DTO assembly of `dto.go` moves to the presenter, not the gateway: it is view formatting, and it already calls `Skew()`.
- [ ] **Step 4:** Run `make test` — PASS.
- [ ] **Step 5:** `git commit -m "refactor: les sept files de lecture en clean architecture"`

### Task 13: Request creation

**Files:**
- Create: `internal/adapter/gateway/postgres/{request_gateway,number_registry_gateway}.go`, `internal/usecase/porting/{create_individual_request,create_enterprise_request,create_restitution_request}.go`, `internal/adapter/controller/request_controller.go`
- Delete: `internal/api/demandes_creation.go` (468 lines) and its three test files

**Interfaces:**
- Produces: `port.RequestGateway` (`Create`, `ByID`, `UpdateStep`, `SetStatus`, `AttachNumbers`, `AttachClient`), `port.NumberRegistryGateway` (`ByMSISDN`, `UpdateOwner`, `ClearLastPortingDate`).

- [ ] **Step 1:** Move the 13 test functions of `creation_particulier_test.go`, `creation_entreprise_test.go`, `creation_restitution_test.go`.
- [ ] **Step 2:** Run `make test` — FAIL.
- [ ] **Step 3:** The eligibility rules already live in `entity`. The interactor orchestrates: check eligibility, then `UnitOfWork.Do` covering OTP consumption *and* request creation — the guarantee `643415f` established must survive, and Task 5's rollback test is what proves the boundary works.
- [ ] **Step 4:** Run `make test` — PASS.
- [ ] **Step 5:** `git commit -m "refactor: les trois creations de demande en clean architecture"`

### Task 14: Acceptance

**Files:**
- Create: `internal/usecase/porting/{accept_request,accept_fleet_request}.go`, extend `request_controller.go`
- Delete: `internal/api/acceptation.go` (335 lines), `internal/api/acceptation_test.go`

- [ ] **Step 1:** Move the 9 test functions, including the partial-fleet-rejection cases.
- [ ] **Step 2:** Run `make test` — FAIL.
- [ ] **Step 3:** Move the 8 SQL statements into `RequestGateway`. `entity.CanAccept` already holds the rule; the interactor must not duplicate it.
- [ ] **Step 4:** Run `make test` — PASS.
- [ ] **Step 5:** `git commit -m "refactor: l'acceptation particulier et flotte en clean architecture"`

### Task 15: Confirmation, processing, cancellation

**Files:**
- Create: `internal/usecase/porting/{confirm_request,process_step,cancel_request}.go`
- Delete: `internal/api/confirmation.go`, `traitement.go`, `annulation.go` and their three test files

- [ ] **Step 1:** Move the test functions of the three files, keeping the ANO-003 `500` assertions exactly.
- [ ] **Step 2:** Run `make test` — FAIL.
- [ ] **Step 3:** `ProcessStep` calls `entity.CanProcess` then `port.Engine.ScheduleTransition`; confirmation uses `entity.ExpectedConfirmers`. The `ConfirmationGateway` is added to `port.Repositories`.
- [ ] **Step 4:** Run `make test` — PASS.
- [ ] **Step 5:** `git commit -m "refactor: confirmation, traitement et annulation en clean architecture"`

### Task 16: Reverse, incidents, sandbox purge

**Files:**
- Create: `internal/usecase/reverse/*.go`, `internal/usecase/incident/*.go`, `internal/usecase/sandbox/purge_test_data.go`, their gateways and controllers
- Delete: `internal/api/reverse.go`, `incidents.go`, `sandbox.go` and their test files

- [ ] **Step 1:** Move the 20 test functions of the three test files.
- [ ] **Step 2:** Run `make test` — FAIL.
- [ ] **Step 3:** `PurgeTestData` is the strongest case for `UnitOfWork`: it touches five tables and must stay atomic. Its scope stays `createur_operateur_id`, never the `/mes-demandes` filter.
- [ ] **Step 4:** Run `make test` — PASS.
- [ ] **Step 5:** `git commit -m "refactor: reverse, incidents et purge en clean architecture"`

### Task 17: The platform engine

**Files:**
- Create: `internal/usecase/platform/{expire_overdue_steps,converge_pending_transitions,auto_validate_reverses}.go`, `internal/framework/engine/engine.go`
- Delete: `internal/engine/` after moving its 16 tests

- [ ] **Step 1:** Move the 16 test functions of `engine_test.go` and `reverse_test.go`.
- [ ] **Step 2:** Run `make test` — FAIL.
- [ ] **Step 3:** The loop and its ticker stay in `framework/engine`; the three behaviours become interactors. `Tick` calls them in the current order. Keep the database-side deadline computation of `94af3f2` — it exists to remove flakiness and must not move into Go.
- [ ] **Step 4:** Run `make test` — PASS.
- [ ] **Step 5:** `git commit -m "refactor: le moteur de plateforme en clean architecture"`

### Task 18: Composition root, and `internal/api` deleted

**Files:**
- Modify: `cmd/server/main.go`, `cmd/artp/main.go`, `internal/framework/web/router.go`
- Delete: `internal/api/`, `internal/httpx/`, `internal/store/`, `internal/domain/`, `internal/apperr/`

**Interfaces:**
- Consumes: every constructor produced by Tasks 3–16.

- [ ] **Step 1:** Run `make test` and record the count of passing tests as the baseline.
- [ ] **Step 2:** Wire all 36 routes in `router.go`, in the same order and with the same middleware placement as today's `NewRouter`, including the `SANDBOX_ADMIN` conditional group.
- [ ] **Step 3:** Delete the five obsolete packages.
- [ ] **Step 4:** Run `make test` — PASS with the same count as Step 1. Then `go test ./test/ -run TestDependencyRule -v` — PASS.
- [ ] **Step 5:** `git commit -m "refactor: racine de composition, et suppression de l'ancienne couche api"`

---

## Phase 4 — Documentation

### Task 19: Package documentation

**Files:**
- Create: `doc.go` in `entity`, `usecase/port`, each `usecase/<capability>`, `adapter/controller`, `adapter/presenter`, `adapter/gateway/postgres`, `framework/web`, `framework/persistence`, `testsupport`

- [ ] **Step 1:** Write each `doc.go` stating three things: what the package is responsible for, what it is allowed to import, and what it must never know.
- [ ] **Step 2:** Run `go vet ./...` — silent.
- [ ] **Step 3:** Run `make test` — PASS.
- [ ] **Step 4:** `git commit -m "docs: un doc.go par paquet, role et dependances autorisees"`

### Task 20: Architecture document and ADRs

**Files:**
- Create: `docs/architecture.md`, `docs/adr/0001-french-sql-columns.md`, `0002-unit-of-work.md`, `0003-presenters-carry-fidelity.md`, `0004-integration-build-tag.md`
- Modify: `README.md`

- [ ] **Step 1:** Write `docs/architecture.md`: the layer diagram, the dependency rule and the test that enforces it, a full request walkthrough from Gin to pgx and back, and the contract boundary table.
- [ ] **Step 2:** Write the four ADRs in the standard form — context, decision, consequences.
- [ ] **Step 3:** Update the README structure section to the new tree.
- [ ] **Step 4:** Run `make test` — PASS.
- [ ] **Step 5:** `git commit -m "docs: architecture, ADR et README a jour"`

### Task 21: Final sweep

- [ ] **Step 1:** Run `grep -rniE '\b(demande|etape|numero|operateur|motif|traitement|acceptation)\b' --include='*.go' internal/ cmd/ test/ | grep -v '"' | grep -v '`'` — every remaining hit must be inside a SQL string, a JSON tag, or a route path. Fix anything else.
- [ ] **Step 2:** Run `grep -rl 'jackc/pgx\|gin-gonic' --include='*.go' internal/ | grep -v '^internal/adapter\|^internal/framework\|^internal/testsupport'` — expected: no output.
- [ ] **Step 3:** Rename `conformite_captures_test.go` to `captured_responses_test.go`, translating identifiers and comments only — never an asserted value.
- [ ] **Step 4:** Run `make test`, `gofmt -l .`, `go vet ./...` — PASS, empty, silent.
- [ ] **Step 5:** `git commit -m "chore: derniere passe, plus aucun identifiant francais hors contrat"`

---

## Acceptance

- `make test` green, with at least the 200 test functions of the baseline.
- `test/architecture_test.go` green: the dependency rule holds.
- No `pgx` or `gin` import outside `internal/adapter/` and `internal/framework/`.
- `captured_responses_test.go` asserts the same values as `conformite_captures_test.go` did.
- The 36 routes answer exactly as before: same status codes, same bodies.
- `gofmt -l .` empty, `go vet ./...` silent.
