GO ?= go
DB_URL ?= postgres://eddndev@/purpura_test?host=/var/run/postgresql

.PHONY: fmt vet test test-integration build run sqlc tidy ci

fmt:
	gofmt -w .

vet:
	$(GO) vet ./...

test:
	$(GO) test -race ./...

test-integration:
	TEST_DATABASE_URL="$(DB_URL)" $(GO) test -tags=integration -p 1 ./...

build:
	$(GO) build -o bin/purpura-api ./cmd/api

run: build
	./bin/purpura-api

sqlc:
	sqlc generate

tidy:
	$(GO) mod tidy

ci: vet test test-integration build
