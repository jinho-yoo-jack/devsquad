.PHONY: test test-integration build run generate resetdb

test:
	go test -race ./...
	go vet ./...

build:
	go build -o bin/devsquad ./cmd/devsquad

run:
	go run ./cmd/devsquad

generate:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate

test-integration:
	docker compose -p devsquad-go-test -f infra/docker-compose.test.yml up -d --wait
	DEVSQUAD_TEST_DB_URL=postgres://devsquad:devsquad@localhost:15432/devsquad go test -race -count=1 -timeout=180s ./internal/integration

# Explicit development reset: deletes this compose project's database and workspaces.
resetdb:
	docker compose -f infra/docker-compose.yml --profile full down -v
	docker compose -f infra/docker-compose.yml up -d --wait postgres
