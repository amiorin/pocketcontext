.PHONY: build test
build:
	CGO_ENABLED=1 go build -tags sqlite_math_functions -o bin/pocketcontext ./cmd/pocketcontext
test:
	CGO_ENABLED=1 go test -tags sqlite_math_functions -race ./...
