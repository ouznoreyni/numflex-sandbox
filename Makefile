DB_TEST := postgres://numflex:numflex@localhost:5433/numflex_test?sslmode=disable

up:
	docker compose up -d postgres postgres-test

test: up
	DATABASE_URL="$(DB_TEST)" \
	ETAPE_TIMEOUT_SECONDS=0 CONVERGENCE_MIN_SECONDS=0 CONVERGENCE_MAX_SECONDS=0 \
	COMPLETION_LATENCY_MS=0 CLOCK_SKEW_SECONDS=0 \
	go test ./... -p 1 -count=1

# CORS_ALLOWED_ORIGINS autorise la page Swagger (port 8081) a appeler l'API
# depuis un navigateur. La plateforme reelle n'envoie pas de CORS : retirer
# cette variable pour retrouver son comportement exact.
run: up
	DATABASE_URL="postgres://numflex:numflex@localhost:5432/numflex?sslmode=disable" \
	CORS_ALLOWED_ORIGINS="http://localhost:8081" \
	go run ./cmd/server

# Documentation servie hors de la gateway, sur un port distinct : le sandbox ne
# doit exposer que les 33 routes du contrat (aucune route de doc, de santé ni de
# metrics). http://localhost:8081/swagger.html
swagger:
	@echo "Swagger UI → http://localhost:8081/swagger.html"
	cd docs && python3 -m http.server 8081 --bind 127.0.0.1

# Régénère openapi.json et swagger.html depuis openapi.yaml, seule source.
swagger-build:
	python3 scripts/build_swagger.py

# ─── Image ──────────────────────────────────────────────────────────────────
# Le nom local est IMAGE:VERSION, le nom publié REGISTRY/IMAGE:VERSION. Chaque
# variable se surcharge :
#   make image VERSION=v0.4.0
#   make push  REGISTRY=ghcr.io/yas VERSION=v0.4.0
IMAGE     ?= numflex-sandbox
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
REGISTRY  ?= ghcr.io/yas
PLATFORMS ?= linux/amd64,linux/arm64
BUILDER   ?= numflex-builder

# Construit pour l'architecture locale et charge l'image dans le démon.
image:
	docker build -t $(IMAGE):$(VERSION) -t $(IMAGE):latest .
	@docker images $(IMAGE):$(VERSION) --format '  {{.Repository}}:{{.Tag}}  {{.Size}}'

# Construit et publie en une passe. buildx est obligatoire ici : un manifeste
# multi-architecture ne peut pas être chargé dans le démon local, donc il n'y a
# pas de `make image` préalable — seul le cache de build est partagé. Le pilote
# `docker` par défaut ne sait pas produire de multi-arch, d'où le constructeur
# dédié, créé au premier appel.
#
# `docker login $(REGISTRY)` d'abord. ALLOW_DIRTY=1 pour publier depuis un
# arbre modifié — la version porterait alors le suffixe `-dirty`.
push:
	@git diff --quiet HEAD 2>/dev/null || test -n "$(ALLOW_DIRTY)" || \
	  { echo "arbre de travail modifié : commitez, ou make push ALLOW_DIRTY=1"; exit 1; }
	@docker buildx inspect $(BUILDER) >/dev/null 2>&1 || \
	  docker buildx create --name $(BUILDER) --driver docker-container >/dev/null
	docker buildx build --builder $(BUILDER) --platform $(PLATFORMS) \
	  --tag $(REGISTRY)/$(IMAGE):$(VERSION) \
	  --tag $(REGISTRY)/$(IMAGE):latest \
	  --push .
	@echo "publié : $(REGISTRY)/$(IMAGE):$(VERSION)"

.PHONY: up test run swagger swagger-build image push
