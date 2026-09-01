DB_TEST := postgres://numflex:numflex@localhost:5433/numflex_test?sslmode=disable

up:
	docker compose up -d postgres postgres-test

test: up
	DATABASE_URL="$(DB_TEST)" \
	ETAPE_TIMEOUT_SECONDS=0 CONVERGENCE_MIN_SECONDS=0 CONVERGENCE_MAX_SECONDS=0 \
	COMPLETION_LATENCY_MS=0 CLOCK_SKEW_SECONDS=0 \
	go test ./... -p 1 -count=1

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

.PHONY: up test run swagger swagger-build
