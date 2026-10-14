.PHONY: test test-short gen-types gen-sql dev-db

test:
	cd backend && go test ./...

test-short:
	cd backend && go test -short ./...

gen-types:
	cd backend && go run github.com/gzuidhof/tygo@v0.2.21 generate

SQLC_VERSION := v1.31.1

gen-sql:
	cd backend && go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

dev-db:
	docker compose up -d --wait postgres
