DB_TEST := postgres://numflex:numflex@localhost:5433/numflex_test?sslmode=disable

up:
	docker compose up -d postgres postgres-test

test: up
	DATABASE_URL="$(DB_TEST)" \
	ETAPE_TIMEOUT_SECONDS=0 CONVERGENCE_MIN_SECONDS=0 CONVERGENCE_MAX_SECONDS=0 \
	COMPLETION_LATENCY_MS=0 CLOCK_SKEW_SECONDS=0 \
	go test ./... -p 1 -count=1

# Le CORS est ouvert a toute origine par defaut, pour que la page Swagger
# (port 8081) puisse appeler l'API depuis un navigateur. La plateforme reelle
# n'en envoie pas : CORS_ALLOWED_ORIGINS="" retrouve son comportement exact.
run: up
	DATABASE_URL="postgres://numflex:numflex@localhost:5432/numflex?sslmode=disable" \
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
# `latest` est toujours produit et toujours publié. VERSION=... ajoute un second
# tag, figé sur cette version :
#   make image                                   → numflex-sandbox:latest
#   make push                                    → …/numflex-sandbox:latest
#   make push VERSION=v0.4.0                     → :latest + :v0.4.0
#   make push REGISTRY=harbor.noorexe.com/numflex
IMAGE     ?= numflex-sandbox
VERSION   ?=
REGISTRY  ?= docker.io/ouzdiop268
PLATFORMS ?= linux/amd64,linux/arm64
BUILDER   ?= numflex-builder

# Construit pour l'architecture locale et charge l'image dans le démon.
image:
	docker build -t $(IMAGE):latest $(if $(VERSION),-t $(IMAGE):$(VERSION),) .
	@docker images $(IMAGE):latest --format '  {{.Repository}}:{{.Tag}}  {{.Size}}'

# Construit et publie en une passe. buildx est obligatoire ici : un manifeste
# multi-architecture ne peut pas être chargé dans le démon local, donc il n'y a
# pas de `make image` préalable — seul le cache de build est partagé. Le pilote
# `docker` par défaut ne sait pas produire de multi-arch, d'où le constructeur
# dédié, créé au premier appel. `docker login $(REGISTRY)` d'abord.
#
# Le garde d'arbre propre ne vaut que pour un tag de version : celui-ci doit
# rester reproductible, donc correspondre à un commit. `latest` est par nature
# un pointeur mouvant, et se publie depuis un arbre modifié. ALLOW_DIRTY=1 lève
# le garde — la version porte alors le suffixe `-dirty`.
push:
	@test -z "$(VERSION)" || git diff --quiet HEAD 2>/dev/null || test -n "$(ALLOW_DIRTY)" || \
	  { echo "arbre modifié : un tag de version doit correspondre à un commit (ALLOW_DIRTY=1 pour passer outre)"; exit 1; }
	@docker buildx inspect $(BUILDER) >/dev/null 2>&1 || \
	  docker buildx create --name $(BUILDER) --driver docker-container >/dev/null
	docker buildx build --builder $(BUILDER) --platform $(PLATFORMS) \
	  --tag $(REGISTRY)/$(IMAGE):latest \
	  $(if $(VERSION),--tag $(REGISTRY)/$(IMAGE):$(VERSION),) \
	  --push .
	@echo "publié : $(REGISTRY)/$(IMAGE):latest$(if $(VERSION), et :$(VERSION),)"

.PHONY: up test run swagger swagger-build image push
