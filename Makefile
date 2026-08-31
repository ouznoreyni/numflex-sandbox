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
