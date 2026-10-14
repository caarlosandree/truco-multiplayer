.PHONY: test test-short gen-types

test:
	cd backend && go test ./...

test-short:
	cd backend && go test -short ./...

gen-types:
	cd backend && go run github.com/gzuidhof/tygo@latest generate
