.PHONY: build test
build:
	CGO_ENABLED=1 go build -o bin/pocketcontext ./cmd/pocketcontext
test:
	CGO_ENABLED=1 go test -race ./...
