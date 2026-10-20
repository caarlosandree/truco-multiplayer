.PHONY: test test-short gen-types gen-sql dev-db run migrate web-install web-dev web-build web-test e2e

test:
	cd backend && go test ./...

test-short:
	cd backend && go test -short ./...

gen-types:
	cd backend && go run github.com/gzuidhof/tygo@v0.2.21 generate && go run ./cmd/gen-protocol

SQLC_VERSION := v1.31.1

gen-sql:
	cd backend && go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

dev-db:
	docker compose up -d --wait postgres

run:
	cd backend && set -a && . ../.env && set +a && go run ./cmd/server serve

migrate:
	cd backend && set -a && . ../.env && set +a && go run ./cmd/server migrate

web-install:
	cd frontend && pnpm install --frozen-lockfile

web-dev:
	cd frontend && pnpm dev

web-build:
	cd frontend && pnpm build

web-test:
	cd frontend && pnpm typecheck && pnpm lint && pnpm test

e2e:
	./scripts/e2e.sh
