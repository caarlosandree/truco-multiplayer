.PHONY: test test-short gen-types dev-db

test:
	cd backend && go test ./...

test-short:
	cd backend && go test -short ./...

gen-types:
	cd backend && go run github.com/gzuidhof/tygo@v0.2.21 generate

dev-db:
	docker compose up -d --wait postgres
