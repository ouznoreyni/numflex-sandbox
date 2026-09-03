<h1 align="center">NumFlex Sandbox</h1>

<p align="center">
  <strong>A local double of the ARTP Senegal mobile number portability API Gateway.</strong><br>
  One command, no dependencies, the complete state machine — and the acceptance platform's
  anomalies reproduced exactly.
</p>

<p align="center">
  <img alt="All-in-one image size" src="https://img.shields.io/docker/image-size/ouzdiop268/numflex-sandbox/latest?label=all-in-one%20image&color=0db7ed">
  <img alt="Docker Hub pulls" src="https://img.shields.io/docker/pulls/ouzdiop268/numflex-sandbox?color=0db7ed">
  <img alt="Go 1.25" src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white">
  <img alt="ARTP guide v2 contract" src="https://img.shields.io/badge/contract-ARTP%20guide%20v2-informational">
</p>

<p align="center"><a href="README.md">Version française</a></p>

---

Integrating with the ARTP portability platform needs an environment you do not always have: a shared
acceptance instance, time slots, numbers a colleague has already burned. This sandbox makes that
environment local and disposable. It exposes **exactly** the contract's 33 routes, runs the same
`ACCEPTATION → DESACTIVATION → ACTIVATION → CONFIRMATION → COMPLETION` cycle, applies the same
permissions — and fails in the same places the real platform does, because an integration that works
around an anomaly in acceptance must work around it here too.

> **A note on language.** The API speaks French, because the contract does: route paths, JSON field
> names, error messages and reference data are French and must stay so for the sandbox to be
> indistinguishable from the platform. This page is the English companion to
> [`README.md`](README.md).

## Contents

| | |
|---|---|
| **[Getting started](#getting-started)** · [first porting](#your-first-porting-in-four-calls) · [pitfalls](#five-first-run-pitfalls) | Thirty seconds, one command |
| **[Images](#published-images)** | All-in-one `:latest`, slim `:slim` |
| **[Configuration](#configuration)** · [fidelity](#fidelity-real-reproduces-what-was-measured) | One variable, three sources |
| **[Number pool](#number-pool)** | Which MSISDNs exist, and how far |
| **[API surface](#api-surface)** · [accounts](#accounts) · [off-contract](#the-two-off-contract-routes) | 33 + 2 + 2 routes |
| **[Reproduced anomalies](#reproduced-anomalies)** | What is wrong on purpose |
| **[Documentation](#api-documentation)** | Swagger, OpenAPI, Postman |
| **[Deploying](#deploying)** · [publishing](#publishing-the-images) | Slim image, separate database |
| **[Developing](#developing)** · [tests](#tests) · [`artp` CLI](#the-artp-regulator-cli) | Clean Architecture, integration suite |

---

## Getting started

```bash
docker run --rm -p 8080:8080 ouzdiop268/numflex-sandbox:latest
```

That is all: no database to install, no network to create, no configuration file. The image carries
its own PostgreSQL, initialises the cluster, runs the migrations, seeds the number pool, then serves
the API **and** its documentation on a single port.

| | |
|---|---|
| API | <http://localhost:8080/api/gateway/v1> |
| Documentation | <http://localhost:8080/swagger.html> |
| First `200` | **~25 s** after launch — `--rm` pays it again every time |

The default pool gives **a hundred thousand numbers per range**, eight hundred thousand per
operator. To make *every* well-formed number of a range exist — `771000000` through `771999999` —
add `FULL_NUMBERS=true`; that costs 4 min 20 s of startup, paid once if you keep a volume:

```bash
docker run -d -p 8080:8080 -v "$PWD/data:/data" \
  ouzdiop268/numflex-sandbox:latest PGDATA=/data FULL_NUMBERS=true
```

### Published images

| Image | Size | Contents |
|---|---|---|
| `ouzdiop268/numflex-sandbox:latest` | **~44 MB** | The server, PostgreSQL **and** the Swagger page. Nothing to orchestrate. |
| `ouzdiop268/numflex-sandbox:slim` | **~11 MB** | The server alone, on `scratch`. Bring your own database. |

`linux/amd64` and `linux/arm64`, Apple Silicon included. Every publication also lays down a frozen
version tag — `:defcon-1` beside `:latest`, `:slim-defcon-1` beside `:slim`. Moving tags follow the
latest build; version tags match a commit and never move again.

The all-in-one image is a **demonstration convenience, not a hardening**: it ships a shell and a
package manager, and starts as root for the duration of `initdb`. For a deployment, use `:slim` and
a separate database. In both cases PostgreSQL listens on `127.0.0.1` only inside the container and
5432 is never published: the API is the single door.

### Your first porting, in four calls

Paste into another terminal while the container runs. `jq` is only there to read the responses.

```bash
# 1. authenticate — three accounts exist: yas, orange, expresso
TOKEN=$(curl -s localhost:8080/api/authenticate \
  -H 'Content-Type: application/json' \
  -d '{"username":"yas","password":"yas2026"}' | jq -r .id_token)

# 2. read the operator reference list and keep two identifiers
ORANGE=$(curl -s localhost:8080/api/gateway/v1/operateurs -H "Authorization: Bearer $TOKEN" \
  | jq -r '.data[] | select(.nom=="ORANGE") | .id')
YAS=$(curl -s localhost:8080/api/gateway/v1/operateurs -H "Authorization: Bearer $TOKEN" \
  | jq -r '.data[] | select(.nom=="YAS") | .id')

# 3. trigger the OTP — 771000001 belongs to ORANGE and was never ported
curl -s localhost:8080/api/gateway/v1/otp/send -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"numero":"771000001"}'

# 4. create the ORANGE → YAS porting request
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

The rest of the cycle belongs to ORANGE: authenticate again as `orange` for `/demandes/a-accepter`,
then the acceptance. **The token is not neutral — each step is reserved to one specific operator**;
see [Accounts](#accounts).

### Five first-run pitfalls

<details>
<summary>What surprises everyone on first contact — and why it is deliberate</summary>

<br>

**1. The OTP field is called `otpCode`, not `code`.** The code is always `123456`
(`OTP_STATIC_CODE`): no SMS is sent, and the `otp/send` response attests to nothing (ANO-021).

**2. A business rejection comes back as `500`, not `4xx`.** Request not found, operator not
entitled, step not reached: all `500` with `RuntimeException: …` in `detail`. That is not a
failure, it is ANO-003, reproduced on purpose.

**3. A made-up number does not exist, and the sandbox says so badly.** The registry is closed:
outside the seeded numbers, no MSISDN is known, and creation answers
`RuntimeException: Le numéro n'appartient pas à l'opérateur source indiqué`
(`OPERATEUR_SOURCE_INCORRECT`). The same message comes out when the number exists but
`operateurSourceId` is not its current holder — nothing tells the two cases apart. Nothing warns you
at the previous step either: `otp/send` accepts any number without consulting the registry. To
settle it: `GET /api/sandbox/v1/numeros/tranches?operateur=ORANGE` says exactly which numbers exist.

**4. Only the all-in-one image serves the documentation.** `:slim` starts from `scratch` and ships
no page: `/swagger.html` answers `404` there. In both images `/api/gateway/v1` keeps exactly its 33
routes — the page lives at the root, beside the contract, never inside it.

**5. The sandbox is slow by default, and that is deliberate**: `COMPLETION` answers in ~30 s
(ANO-005), a step expires on its own after ~349 s (ANO-006), timestamps drift by 9 minutes
(ANO-015). To explore without any of that:

```bash
docker run --rm -p 8080:8080 ouzdiop268/numflex-sandbox:latest \
  STEP_TIMEOUT_SECONDS=0 COMPLETION_LATENCY_MS=0 CLOCK_SKEW_SECONDS=0
```

</details>

---

## Configuration

Everything is set through environment variables, and nothing else.

| Variable | Default | Effect |
|---|---|---|
| `PORT` | `8080` | HTTP listening port |
| `DATABASE_URL` | — | **Required**, except in the all-in-one image which decides it itself |
| `JWT_SECRET` | `numflex-sandbox-dev-secret` | HS512 signing secret |
| `JWT_TTL_HOURS` | `24` | Token validity |
| `FIDELITY` | `real` | `real` \| `contract` — [see below](#fidelity-real-reproduces-what-was-measured) |
| `STEP_TIMEOUT_SECONDS` | `349` | Step expiry; `0` = no expiry |
| `ENGINE_TICK_SECONDS` | `10` | Engine tick |
| `CONVERGENCE_MIN_SECONDS` | `0` | Convergence window — [see below](#convergence-two-measured-behaviours) |
| `CONVERGENCE_MAX_SECONDS` | `0` | `0` = transition applied within the request |
| `COMPLETION_LATENCY_MS` | `30500` | Simulated `COMPLETION` latency (ANO-005) |
| `CLOCK_SKEW_SECONDS` | `540` | Clock drift injected into timestamps (ANO-015) |
| `OTP_STATIC_CODE` | `123456` | Accepted OTP code |
| `OTP_TTL_SECONDS` | `300` | OTP validity |
| `OTP_MAX_ATTEMPTS` | `3` | Entry attempts |
| `REVERSE_AUTO_VALIDATION_SECONDS` | `0` | `0` = validation through the `artp` CLI only |
| `FULL_NUMBERS` | `false` | Fills every portable range whole, `000000` to `999999` |
| `POOL_NUMBERS_PER_OPERATOR` | `800000` | Never-ported numbers per operator, between `8` and `8000000`. Absent, it follows `FULL_NUMBERS`; set, it wins over it |
| `DOCS_ENABLED` | `true` | Serves `/swagger.html`, `/openapi.yaml`, `/openapi.json` at the root |
| `ENV_FILE` | `.env` | Path of the environment file to load |

`.env.example` lists them all, commented.

### Where values come from

Three sources, strongest first:

| Source | Example |
|---|---|
| Server **arguments** | `docker run … numflex-sandbox:latest PORT=9090 FIDELITY=contract` |
| Process **environment** | `docker run -e PORT=9090`, compose's `environment:`, `export PORT=9090` |
| **`.env` file** | `-v $PWD/.env:/app/.env:ro`, `--env-file /cfg.env`, or `.env` in the working directory |

A stronger source wins: a mounted `.env` provides the base, a `-e` corrects one value for one run,
an argument settles it. The file is looked up in the working directory — `/app` in the container —
unless `ENV_FILE` or `--env-file <path>` names another one; an `ENV_FILE` that is asked for and
missing is a startup error, an implicit `.env` that is absent is not.

**A comment takes a line of its own**: outside quotes, everything after the `=` belongs to the
value, `#` included, so a secret is never truncated. The `.env` is never copied into the image
(`.dockerignore`).

In the all-in-one image, a lone absolute path as the first argument is a shorthand:
`… numflex-sandbox:latest /data` means `PGDATA=/data`, and if it names an existing file it is the
`.env`. It is the one argument the server could not read itself — it only accepts `--env-file` and
`KEY=value`.

### Fidelity: `real` reproduces what was measured

<details>
<summary>Two modes, one state machine</summary>

<br>

In **`real`** mode — the default — the sandbox behaves like the *measured* platform, not the
*documented* one. Business errors come out as `500`, no error response carries a `code` field, the
`otp/send` response omits `data`, timestamps drift by nine minutes. That is the whole point: an
integration that passes here will pass in acceptance.

In **`contract`** mode, the same state machine is served as the contract is written — correct HTTP
codes, systematic envelope, populated error-code catalogue. Useful to check a client has not made
itself dependent on an anomaly.

**The state machine is identical in both modes; only the presentation changes** — which is what
`TestSameScenarioInContractMode` locks down.

</details>

### Convergence: two measured behaviours

<details>
<summary>Why the default applies the transition within the request</summary>

<br>

Responses captured against the platform on **2026-08-27** show **synchronous** transitions:
`POST /demandes/acceptation` answers `DESACTIVATION`, `/demandes/traitement` on `DESACTIVATION`
answers `ACTIVATION`, the last confirmation answers `COMPLETION`. That is the sandbox's default
(`CONVERGENCE_MAX_SECONDS=0`).

The older SIT v0.3 report measured the opposite: the response carried the step *preceding* the
transition, which happened 1 to 6 minutes later (R-10). A non-zero `CONVERGENCE_MIN/MAX_SECONDS`
restores that behaviour — useful to test an integration against that version.

Both sources are measurements; the most recent one sets the default.

</details>

### Deterministic profile (CI)

```
STEP_TIMEOUT_SECONDS=0 CONVERGENCE_MIN_SECONDS=0 CONVERGENCE_MAX_SECONDS=0
COMPLETION_LATENCY_MS=0 CLOCK_SKEW_SECONDS=0
```

No expiry, immediate convergence, no latency: a full cycle in under a second. This is the profile
`make test` uses.

---

## Number pool

The registry is **closed**: only seeded numbers exist, and any other MSISDN is rejected at creation.
A range's prefix is three digits and its tail six — `771000001` reads as `771` + `000001`. A range
always starts at `000000`; what changes is where it stops.

| Operator | Never-ported ranges | Per range | Portable total |
|---|---|---|---|
| ORANGE | `771000000`–`771099999` … `778000000`–`778099999` | 100,000 | **800,000** |
| YAS | `781000000`–`781099999` … `788000000`–`788099999` | 100,000 | **800,000** |
| EXPRESSO | `711` … `718`, tails `000000`–`000999` | 1,000 | 8,000 |
| Historical | `761000000`–`761000999`, `701000000`–`701000999` | 1,000 | 2,000 |

With `FULL_NUMBERS=true` the first two rows become `771000000`–`771999999`: a million per range,
eight million per operator, and every well-formed number of a range exists. EXPRESSO keeps its
thousand per range either way — it is there to exercise porting between two third parties (UC-08),
not to be consumed in volume.

<details>
<summary>The <code>900</code> group: rejection material, not portable stock</summary>

<br>

One range per operator where **every** number has already been ported, the four scenarios stacked in
blocks of a thousand:

| Block | Situation | Makes testable |
|---|---|---|
| `…000000` → `…000999` | ported 30 days ago | `DELAI_PORTAGE_NON_RESPECTE` / ANO-002 |
| `…001000` → `…001999` | ported 8 months ago | Nominal restitution |
| `…002000` → `…002999` | ported 2 months ago | `DELAI_RESTITUTION_NON_RESPECTE` / ANO-020 |
| `…003000` → `…003999` | ported, then already restituted | `NUMERO_DEJA_RESTITUE` |

| Range | Current holder | Origin operator |
|---|---|---|
| `779…` | ORANGE | YAS |
| `789…` | YAS | ORANGE |
| `719…` | EXPRESSO | ORANGE |

`789001001` is held by YAS, came from ORANGE, ported eight months ago: ORANGE may ask for its
restitution. `779000001` has been at ORANGE for thirty days: it hits the three-month delay. Two of
those blocks (8 months, already restituted) are nevertheless past 3 months and port normally.

</details>

**What the volume costs**, measured in the all-in-one image on Apple Silicon, `initdb` and
migrations included:

| | Rows | `numero` table | Cold start |
|---|---|---|---|
| Default | 1,622,000 | 193 MB | **~25 s** |
| `FULL_NUMBERS=true` | 16,022,000 | 1,905 MB | **4 min 20 s** |

The seed inserts one range per statement (`INSERT … SELECT generate_series`) and **skips a range
already installed** — a range is laid down whole or not at all, and the presence of its last number
is enough to know. Restart on the same database: **2 s**. A persistent volume therefore never pays
for the seed twice.

---

## API surface

**37 routes in total, and not one more**: the contract's 33, the 2 authentication ones, the 2
sandbox ones. No health, metrics or debug route — the surface exposed under `/api/gateway/v1` must
be exactly the real platform's, and a route-table test checks it on every build.

| Group | Routes |
|---|---|
| Authentication | `POST`/`GET /api/authenticate` |
| Reference data (§7.1) | `/operateurs`, `/motifs-rejet`, `/types-demande`, `/processus`, `/types-incident` |
| OTP (§7.2) | `/otp/send`, `/otp/verify` |
| Creation (§7.3–7.6) | `/demandes/particulier`, `/demandes/entreprise`, `/demandes/restitution`, `/reverse-requests` |
| Queries (§7.7) | `/demandes/mes-demandes`, `/a-accepter`, `/a-traiter`, `/a-confirmer`, `/deja-confirmees`, `/in`, `/out`, plus the three `:id` details |
| Workflow (§7.8–7.11) | `/demandes/acceptation`, `/demandes/:id/acceptation`, `/demandes/a-confirmer`, `/demandes/traitement`, `/demandes/:id/annuler` |
| Incidents (§7.12) | `/incidents/gateway`, `/incidents/interne`, their `/:id/resoudre` and `/mes-incidents` |
| **Off contract** | `/api/sandbox/v1/numeros/tranches`, `/api/sandbox/v1/demandes` |

### Accounts

| username | password | operator | roles |
|---|---|---|---|
| `orange` | `orange2026` | ORANGE | `ROLE_OPERATEUR_ADMIN`, `ROLE_USER` |
| `yas` | `yas2026` | YAS | `ROLE_OPERATEUR_ADMIN`, `ROLE_USER` |
| `expresso` | `expresso2026` | EXPRESSO | `ROLE_OPERATEUR_ADMIN`, `ROLE_USER` |

The returned `id_token` is an HS512 JWT valid for 24 h, to be presented as `Authorization: Bearer`.

**Each step is reserved to one specific operator.** `yas` creates a request, cancels it, processes
`ACTIVATION` and `COMPLETION`, reads `/demandes/in`. `orange` — the source — accepts, processes
`DESACTIVATION`, reads `/demandes/a-accepter` and `/demandes/out`, asks for a restitution or a
reverse. Confirmation is expected from everyone **except** the recipient. A call made with the wrong
account does not return a readable rejection, but a `500` (ANO-003).

### The two off-contract routes

They do not exist at the ARTP and live under a different prefix, so that `/api/gateway/v1` stays
exactly the platform's surface: a client switching its `baseUrl` to the ARTP acceptance environment
only loses what it knows is not there. Both require a token and appear in the Swagger page under the
**Sandbox** tag.

```bash
GET    /api/sandbox/v1/numeros/tranches?operateur=ORANGE
DELETE /api/sandbox/v1/demandes
```

<details>
<summary><code>GET /numeros/tranches</code> — which numbers exist, and how far</summary>

<br>

`operateur` is `ORANGE`, `YAS` or `EXPRESSO`, case-insensitive; missing or unknown, the response is
a `400` in `problem+json` naming the accepted values — off contract, hence outside ANO-003.

```json
{ "success": true, "code": "SUCCESS", "message": "Tranches de l'opérateur ORANGE",
  "data": { "operateur": "ORANGE", "operateurId": "6a21745ce6c37b5b5b487ec1",
            "nombreTranches": 9, "totalNumeros": 804000,
            "tranches": [ { "prefixe": "771", "premier": "771000000", "dernier": "771099999",
                            "total": 100000, "nature": "JAMAIS_PORTE" } ] } }
```

This answers the question the registry answers badly: an MSISDN outside the pool is rejected with
`Le numéro n'appartient pas à l'opérateur source indiqué`, exactly like an existing number declared
under the wrong source operator.

The count is **read from the database**, not derived from configuration: a range installed at
another volume reports its real size, and a number that moved to its recipient after a completed
porting is counted there. `nature` comes from the rows themselves — a range whose numbers carry a
porting date is rejection material.

Counting costs what counting costs: **2.7 s** on the full pool (4.6 s on the very first call, cold
cache), milliseconds on a smaller one. No index redeems it — a composite
`(operateur_actuel_id, msisdn)` was measured at 902 MB for no gain at all.

</details>

<details>
<summary><code>DELETE /demandes</code> — purge your own test data</summary>

<br>

Deletes every request **created by the token's operator** and puts the national registry back the
way it was.

```json
{ "success": true, "code": "SUCCESS", "message": "Demandes purgées avec succès",
  "data": { "demandesSupprimees": 12, "numerosRestaures": 9,
            "otpSupprimes": 4, "reverseSupprimees": 1 } }
```

**The scope is `createur_operateur_id`**, never the `/mes-demandes` filter. A request belongs to two
operators at once; only its creator made it. Consequence: a Port-IN created by a partner to exercise
your Port-OUT cannot be purged with your token.

**What goes, in one transaction**: the caller's requests and everything depending on them
(`demande_numero`, `demande_client`, `etape_historique`, `confirmation`, by cascade); the OTPs of
the numbers involved — without which the same number could not be requested again without going
through `otp/send`; the caller's reverse requests, whose foreign key would otherwise block the
delete. Incidents and reference data are untouched.

**The registry is restored** for every number involved: back to its origin operator, last porting
date and restitution flag cleared. That is what makes a scenario replayable — without it, an
already-ported number would stay blocked for three months by `DELAI_PORTAGE_NON_RESPECTE`.

</details>

---

## Reproduced anomalies

The sandbox does not fix the platform: it reproduces it. Every anomaly below carries its SIT report
identifier.

<details>
<summary>The twenty anomalies, and what they do</summary>

<br>

| Identifier | Reproduced behaviour |
|---|---|
| ANO-001 | No error response carries a `code` field — the ARTP catalogue is not implemented |
| ANO-002 | Re-porting under 3 months → `500 Unexpected runtime exception`, not a clean business rejection |
| ANO-003 | Business errors (request not found, step not reached, operator not entitled) come out as `500` |
| ANO-004 | The Java exception class name leaks into the error body |
| ANO-005 | `COMPLETION` answers in ~30.5 s, and no `Idempotency-Key` header is read |
| ANO-006 | Steps expire on their own after ~349 s: a full cycle completes without any operator acting |
| ANO-008 | **Invalid or expired** token → `401` with an empty body and no `Content-Type` |
| ANO-009 | The rejection-reason reference exposes the field `motif`, not `libelle` |
| ANO-010 | `client.lieuNaissance` is documented as optional but **rejected when absent** |
| ANO-011 | `POST /otp/send` omits the envelope's `data` field instead of setting it to `null` |
| ANO-013 | A step crossed by an action carries `statutEtapeActuel: TERMINE`, not `VALIDE` |
| ANO-014 | OTP states come out as `500` with free-form messages, outside the catalogue |
| ANO-015 | ~9 min clock drift: a created request is timestamped in the future |
| ANO-016 | Authentication failure served outside the envelope, in `problem+json` |
| ANO-018 | The `etape` field of `/demandes/traitement`, removed in v2, is accepted and **silently ignored** |
| ANO-019 | `deja-confirmees` does not record the **source** operator's confirmation; the third party sees its own |
| ANO-020 | Restitution under 6 months → `500` whose `detail` holds a 400 error serialised into a string |
| ANO-021 | The `otp/send` response does not attest SMS delivery — no SMS is sent at all |
| ANO-022 | No subscriber notification at the end of a porting |

</details>

<details>
<summary>Three assumed hypotheses</summary>

<br>

Neither documented in guide v2 nor measured in acceptance. They are marked `[HYP]` in the code, at
the exact place the decision is made.

1. **EXPRESSO routing prefix** (`internal/framework/seed/seed.go`) — `191` (ORANGE) and `192` (YAS)
   are documented; `193` for EXPRESSO is inferred from the series.
2. **`REJETE` status** (`internal/entity/porting_request.go`) — neither documented in the lifecycle
   nor observed. A refused request must carry a terminal state distinct from `TERMINE`.
3. **Role split in restitution** (`internal/usecase/creation/create_restitution_request.go`) — the
   guide does not settle who is source and who is recipient; the sandbox makes the origin operator
   the recipient.

Other, more local `[HYP]` markers exist: `grep -rn '\[HYP\]' internal/`.

</details>

### What this is not

- It is not the ARTP platform, and it is not a specification of what that platform *should* do. When
  acceptance is inconsistent, the sandbox is too.
- No SMS is sent. The OTP code is **static**.
- No subscriber notification (ANO-022) — that gap is **reproduced**, not fixed.
- No back-office, no UI, no downstream network API.

---

## API documentation

The all-in-one image serves the page itself, **on the API's own port**:

```
http://localhost:8080/swagger.html      the Swagger page, spec inlined
http://localhost:8080/openapi.yaml      the specification alone
http://localhost:8080/openapi.json
```

![The sandbox's Swagger page: the test-account banner, operations grouped by section, and the server selector pointed at http://localhost:8080](docs/images/swagger.png)

"Try it out" works with nothing to configure. Those three paths are **at the root, never under
`/api/gateway/v1`**: they exist only because the image ships the `docs/` folder, and
`DOCS_ENABLED=false` removes them. No reverse proxy inside the container, and that is a measured
choice — an nginx in front would stamp `Server` and `Connection` onto all 33 contract responses,
which carry exactly three headers today.

`docs/openapi.yaml` is the **only source**; `make swagger-build` regenerates `openapi.json` and
`swagger.html` from it. `make swagger` serves the page alone on `8081`, without starting the API.

**Postman**: `postman/` holds a collection modelled on the ARTP's own, grouped by guide section,
plus a `sandbox` folder. The test script of `POST /api/authenticate` records `{{token}}`
automatically; switching to the ARTP acceptance environment only takes changing `{{baseUrl}}`.

```bash
curl -O https://raw.githubusercontent.com/ouznoreyni/numflex-sandbox/main/docs/openapi.yaml
curl -O https://raw.githubusercontent.com/ouznoreyni/numflex-sandbox/main/postman/numflex-sandbox.postman_collection.json
```

---

## Deploying

The `:slim` image starts from `scratch`: two static binaries, the migrations, the TLS trust roots,
nothing else — no shell, no package manager, no base CVE to track. It runs as UID 10001, expects
`DATABASE_URL` and runs the migrations itself at startup.

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

| Detail | Why |
|---|---|
| `-v "$PWD/.env:/app/.env:ro"` | `/app` is the working directory: that is where the server looks for `.env`. Read-only, it has nothing to write there |
| `-e DATABASE_URL=…` | Depends on the Docker network, not on the repository — the database host is `numflex-db`. Wins over the `.env` |
| `PORT=8095 FIDELITY=contract` | Arguments, **after** the image name: they win over everything else |
| `--read-only` | No writable filesystem. The server only opens the database and reads its migrations |
| `--cap-drop ALL` | The image already runs without a shell or a suid binary; this closes what was left |
| `--restart unless-stopped` | Migrations and seed are idempotent: a restart overwrites nothing |

**No volume is required on the API side**, not even for the migrations, which ship inside the image.
A `-v` path must be **absolute**; on macOS it must belong to the folders shared with the Docker VM
(`/Users`, `/private`, `/tmp`).

The image carries **no `HEALTHCHECK`** — the sandbox exposes no health route, the real platform
having none. Probe it from the outside, through `POST /api/authenticate`.

### Publishing the images

```bash
docker login
make push                        # …/numflex-sandbox:latest        (all-in-one)
make push VERSION=defcon-1       # :latest **and** :defcon-1
make push-slim VERSION=defcon-1  # :slim **and** :slim-defcon-1
make push-all VERSION=defcon-1   # both images, in one call
make push REGISTRY=harbor.example.com/numflex
```

Build and publish in one pass, for `linux/amd64` and `linux/arm64` — a multi-arch manifest cannot be
loaded into the local daemon, so there is no prior `make image`. The dedicated `buildx` builder is
created on first call.

`latest` and `slim` are always built and always published; `VERSION=…` adds a frozen tag. **The
clean-tree guard applies to that version tag only**: it must stay reproducible, hence match a
commit, whereas a moving pointer is published from a modified tree. `ALLOW_DIRTY=1` lifts the guard.

---

## Developing

```bash
docker compose up          # Postgres + API, migrations and seed included
make run                   # the server locally, against compose's Postgres
make test                  # the full suite, test database dropped afterwards
make test-unit             # no database, no Docker, a few seconds
make image                 # numflex-sandbox:slim
make image-standalone      # numflex-sandbox:latest
make run-standalone        # builds the all-in-one and runs it
make run-standalone FULL=1 DATA=$PWD/data PORT=9000
make swagger               # the page alone on 8081, without the API
make swagger-build         # regenerates openapi.json and swagger.html
```

Several configuration profiles rather than a single `.env`: mount the folder holding them read-only,
then name the one you want with `--env-file /config/recette.env` — or
`-e ENV_FILE=/config/recette.env`, which does the same.

### Structure

Canonical Clean Architecture: four layers, one dependency rule — a package only imports packages of
a lower or equal layer — and `test/architecture_test.go`, which checks it against the real import
graph on every `make test`.

```
cmd/
  server/            the HTTP server: composition root
  artp/              the regulator CLI
internal/
  entity/            layer 0 — pure business rules, imports nothing from this module
  usecase/
    port/            layer 1 — the interfaces interactors need
    <capability>/    layer 1 — one interactor per use case (otp, creation, acceptance…)
  adapter/
    controller/      layer 2 — HTTP → input model, result → view model
    presenter/       layer 2 — view model, in one of the two fidelity modes
    gateway/postgres/ layer 2 — the gateways. The only place naming a French column
  framework/
    web/             layer 3 — Gin engine, middlewares, wiring of the 37 routes
    persistence/     layer 3 — pgx pool, migrations, unit of work
    engine/          layer 3 — the ticker: expiry, convergence, reverse cycle
    clock/ config/ identifier/ seed/ token/
  testsupport/       test database, in-memory doubles, router harness
test/                end-to-end scenarios, conformance captures, architecture guard
migrations/          the schema, in French — see ADR 0001
scripts/             all-in-one image entrypoint, Swagger page generation
```

The layer diagram and the full path of a request are in
[`docs/architecture.md`](docs/architecture.md). The four structural decisions are in
[`docs/adr/`](docs/adr/): French SQL columns, unit of work, fidelity carried by the presenters,
integration-test build tag.

<details>
<summary>French and English: the border is where the text is going</summary>

<br>

The repository is in English — identifiers, comments, assertion messages, CLI and log output. That
is not a style preference: it is what makes `grep` usable as a test. **A French word outside the
cases below is a defect.**

Everything an API client can observe stays French, because the sandbox must be indistinguishable
from the platform:

| What stays French | Example |
|---|---|
| Route paths | `/api/gateway/v1/demandes/a-accepter` |
| JSON names and values | `{"idDemande":…,"etapeActuelle":"ACCEPTATION"}` |
| Response and fault messages | `"Demande particulier créée avec succès"`, `VALIDATION_ECHOUEE` |
| SQL tables and columns | `demande.etape_actuelle` — see [ADR 0001](docs/adr/0001-french-sql-columns.md) |
| Seeded reference data | `Identité non prouvée`, `Numéro Inactif` |

To which are added **quotations** inside otherwise-English comments: names of captured Postman
requests, excerpts from the ARTP guide. Translating one of those strings breaks fidelity to the
contract.

</details>

### Tests

```bash
make test
```

Raises the test database on `5433`, runs the whole suite under the deterministic profile, then
**removes the container** — pass or fail: every test truncates and reseeds that database, it holds
nothing worth keeping, and the exit status that comes out is `go test`'s. The application database
on `5432` is left alone: `make up` and `make run` own it.

Integration tests mount an `httptest` server against a real database and assert on **the exact HTTP
code and body**, never on an abstraction. Named SIT cases are replayed as they are: TC-021, TC-034,
TC-036, TC-041, TC-044, TC-050, TC-062, plus ANO-001 checked in volume and ANO-018 checked on its
real effect.

`test/e2e_test.go` carries four end-to-end scenarios: the guide's §10 porting (ORANGE → YAS with
EXPRESSO's confirmation) through to `TERMINE`, the same porting completed **by pure expiry with no
call at all**, the same scenario in `FIDELITY=contract`, and the check that no error carries a
`code` field.

### The `artp` regulator CLI

The contract places validation and rejection of a reverse request **outside the API gateway**: those
acts are reserved to the ARTP, after administrative validation. The `artp` binary carries them.

```bash
go build -o artp ./cmd/artp
export DATABASE_URL='postgres://numflex:numflex@localhost:5432/numflex?sslmode=disable'

./artp reverse list             # reverse requests and their status
./artp reverse validate <id>    # creates the REVERSE request, at CONFIRMATION
./artp reverse reject <id>
./artp seed                     # replays the seed (idempotent)
```

It also ships in both images, beside the server. As it is not their entrypoint, ask for it
explicitly — passing `DATABASE_URL` along, since `docker exec` does not inherit the variables the
entrypoint exported:

```bash
docker exec -e DATABASE_URL='postgres://numflex:numflex@127.0.0.1:5432/numflex?sslmode=disable' \
  numflex artp reverse list
```

`artp` **does not run the migrations**: the server is the sole owner of the schema's lifecycle.

A contract reminder: on a REVERSE request, `CONFIRMATION` is expected from **every** operator, the
recipient included, and `COMPLETION` is **reserved to the ARTP** — no operator can trigger it
through `/demandes/traitement`.
