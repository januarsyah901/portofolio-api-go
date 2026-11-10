GO ?= go
include $(wildcard .env)
export

.PHONY: run build test test-integration cover lint vet db-up db-down db-reset fmt

run:
	$(GO) run ./cmd/api

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o bin/api ./cmd/api
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o bin/admin ./cmd/admin

test:
	$(GO) test ./...

test-integration:
	$(GO) test -p 1 -tags=integration -count=1 ./...

cover:
	$(GO) test -coverprofile=coverage.out ./... && $(GO) tool cover -func=coverage.out | tail -1

vet:
	$(GO) vet ./...

lint: vet
	@command -v golangci-lint >/dev/null && golangci-lint run ./... || echo "golangci-lint v2 not installed: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0"

fmt:
	gofmt -s -w .

db-up:
	docker compose -f docker-compose.dev.yml up -d --wait

db-down:
	docker compose -f docker-compose.dev.yml down

db-reset:
	docker compose -f docker-compose.dev.yml down -v && $(MAKE) db-up
