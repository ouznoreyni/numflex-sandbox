DB_TEST := postgres://numflex:numflex@localhost:5433/numflex_test?sslmode=disable

# --wait blocks until the healthchecks go green. Without it,
# `docker compose up -d` returns as soon as the containers are *started*,
# not as soon as they accept a connection: on a cold start, go test hits
# the port before Postgres and the suite fails with "connection refused".
#
# `up` brings up the development database alone — the one `make run` uses.
# The test database belongs to `test`, which raises it and takes it down.
up:
	docker compose up -d --wait postgres

# The test database is created for the run and removed after it, whether the
# suite passed or not: it holds nothing worth keeping — every test truncates
# and reseeds it — and a container idling until the next reboot is a
# container nobody remembers starting. The exit status of `go test` is the
# one that comes out, the teardown never masking a failure.
test:
	docker compose up -d --wait postgres-test
	@status=0; \
	DATABASE_URL="$(DB_TEST)" \
	STEP_TIMEOUT_SECONDS=0 CONVERGENCE_MIN_SECONDS=0 CONVERGENCE_MAX_SECONDS=0 \
	COMPLETION_LATENCY_MS=0 CLOCK_SKEW_SECONDS=0 \
	go test -tags=integration ./... -p 1 -count=1 || status=$$?; \
	docker compose rm --stop --force --volumes postgres-test >/dev/null 2>&1 || true; \
	exit $$status

# Unit tests only — no database, no Docker, a few seconds.
test-unit:
	go test ./... -count=1

# The server serves the Swagger page itself, at the root, so "Try it out" is
# same-origin and needs no CORS at all. CORS is open to every origin by default
# for the page served from somewhere else — `make swagger` on 8081, or the file
# opened from disk. The real platform sends none:
# CORS_ALLOWED_ORIGINS="" restores its exact behaviour.
run: up
	DATABASE_URL="postgres://numflex:numflex@localhost:5432/numflex?sslmode=disable" \
	go run ./cmd/server

# Serves docs/ alone, on its own port — for reading the page without starting
# the API. The server also serves it at the root, on the API's port, which is
# the usual way in: http://localhost:8080/swagger.html
# http://localhost:8081/swagger.html
swagger:
	@echo "Swagger UI → http://localhost:8081/swagger.html"
	cd docs && python3 -m http.server 8081 --bind 127.0.0.1

# Regenerates openapi.json and swagger.html from openapi.yaml, the only source.
swagger-build:
	python3 scripts/build_swagger.py

# ─── Images ─────────────────────────────────────────────────────────────────
# Two images, one headline. The all-in-one — server AND database, nothing to
# orchestrate — is what `:latest` points at, because it is what someone
# discovering the sandbox should get from a bare `docker run`. The scratch
# one, which needs a database of its own, is published as `:slim`.
#
#   make image                       → numflex-sandbox:slim, locally
#   make image-standalone            → numflex-sandbox:latest (and :standalone)
#   make push                        → …/numflex-sandbox:latest   (all-in-one)
#   make push VERSION=defcon-1       → :latest + :defcon-1
#   make push-slim VERSION=defcon-1  → :slim + :slim-defcon-1
#   make push-all VERSION=defcon-1   → the two above, in one call
#   make push REGISTRY=harbor.noorexe.com/numflex
IMAGE     ?= numflex-sandbox
VERSION   ?=
REGISTRY  ?= docker.io/ouzdiop268
PLATFORMS ?= linux/amd64,linux/arm64
BUILDER   ?= numflex-builder

# Builds for the local architecture and loads the image into the daemon.
# --target runtime is explicit: the Dockerfile carries a second `standalone`
# target, and the slim image must stay what `make image` produces even if the
# order of the Dockerfile's stages ever changes.
image:
	docker build --target runtime -t $(IMAGE):slim $(if $(VERSION),-t $(IMAGE):slim-$(VERSION),) .
	@$(call size,$(IMAGE):slim)

# Builds and publishes in one pass. buildx is mandatory here: a multi-arch
# manifest cannot be loaded into the local daemon, so there is no prior
# `make image` — only the build cache is shared. The default `docker` driver
# cannot produce multi-arch, hence the dedicated builder, created on first
# call. Run `docker login $(REGISTRY)` first.
#
# The clean-tree guard only applies to a version tag: that one must stay
# reproducible, hence match a commit. `latest` is by nature a moving pointer,
# and is published from a modified tree. ALLOW_DIRTY=1 lifts the guard — the
# tag is then published as is, from a tree that matches no commit.
push:
	@$(clean-tree)
	@$(builder)
	docker buildx build --builder $(BUILDER) --platform $(PLATFORMS) \
	  --target standalone \
	  --tag $(REGISTRY)/$(IMAGE):latest \
	  $(if $(VERSION),--tag $(REGISTRY)/$(IMAGE):$(VERSION),) \
	  --push .
	@echo "published: $(REGISTRY)/$(IMAGE):latest$(if $(VERSION), and :$(VERSION),)"

# The scratch image, published beside it under its own name.
push-slim:
	@$(clean-tree)
	@$(builder)
	docker buildx build --builder $(BUILDER) --platform $(PLATFORMS) \
	  --target runtime \
	  --tag $(REGISTRY)/$(IMAGE):slim \
	  $(if $(VERSION),--tag $(REGISTRY)/$(IMAGE):slim-$(VERSION),) \
	  --push .
	@echo "published: $(REGISTRY)/$(IMAGE):slim$(if $(VERSION), and :slim-$(VERSION),)"

push-all: push push-slim

# The clean-tree guard only applies to a version tag: that one must stay
# reproducible, hence match a commit. A moving pointer — `latest`, `slim` —
# is published from a modified tree without complaint. ALLOW_DIRTY=1 lifts
# the guard, and the version tag then names a tree that matches no commit.
clean-tree = test -z "$(VERSION)" || git diff --quiet HEAD 2>/dev/null || test -n "$(ALLOW_DIRTY)" || \
	  { echo "modified tree: a version tag must match a commit (ALLOW_DIRTY=1 to override)"; exit 1; }

# The default `docker` driver cannot produce a multi-arch manifest, hence a
# dedicated builder, created on first call.
builder = docker buildx inspect $(BUILDER) >/dev/null 2>&1 || \
	  docker buildx create --name $(BUILDER) --driver docker-container >/dev/null

# ─── All-in-one image ───────────────────────────────────────────────────────
# The repository's docker-compose reduced to a single image: PostgreSQL and the
# server in the same container. Nothing to orchestrate, nothing to network.
#
#   make image-standalone                    → numflex-sandbox:standalone, locally
#   make run-standalone                      → runs it, nothing to mount
#   make run-standalone FULL=1               → full ranges (a four-minute seed)
#   make run-standalone DATA=/srv/pg PORT=9000
#   make run-standalone ENV_FILE=./prod.env  → mounts that file on /app/.env
#   make push                                → builds AND publishes, multi-arch
#   make push VERSION=defcon-1               → adds the :defcon-1 tag
#
# Configuration follows the server's precedence — arguments > environment >
# .env > defaults — and the database's data directory is no exception:
#
#   docker run IMAGE PGDATA=/data                        (argument)
#   docker run -e PGDATA=/data IMAGE                     (environment)
#   docker run -v ./.env:/app/.env IMAGE                 (file, PGDATA= inside)
#   docker run -v ./x.env:/c.env IMAGE --env-file /c.env (file, elsewhere)
#
# This image is not a hardening: it ships a shell and a package manager, and
# starts as root for the duration of initdb. For a deployment, use `make push`
# and a separate database.
DATA     ?=
PORT     ?= 8080
ENV_FILE ?=
FULL     ?=

# `docker images` reports a DISK USAGE that counts the build attestations
# and the cache: it announced 457 MB for an image of 120. This reads the
# image's own size, the one that travels.
size = docker image inspect $(1) --format '{{.Size}}' | \
       awk '{ printf "  $(1)  %.0f MB\n", $$1/1024/1024 }'

image-standalone:
	docker build --target standalone -t $(IMAGE):latest -t $(IMAGE):standalone .
	@$(call size,$(IMAGE):latest)

run-standalone: image-standalone
	$(if $(DATA),@mkdir -p "$(DATA)")
	@echo "sandbox → http://localhost:$(PORT)   docs → http://localhost:$(PORT)/swagger.html$(if $(DATA),   data → $(DATA),)$(if $(ENV_FILE),   env → $(ENV_FILE),)"
	docker run --rm -p $(PORT):8080 \
	  $(if $(DATA),-v "$(DATA):/data",) \
	  $(if $(ENV_FILE),-v "$(abspath $(ENV_FILE)):/app/.env:ro",) \
	  $(IMAGE):standalone $(if $(DATA),PGDATA=/data,) $(if $(FULL),FULL_NUMBERS=true,)

.PHONY: up test test-unit run swagger swagger-build image push push-slim push-all image-standalone run-standalone
