# Owner module: language-go-mcp
#
# Convenience targets for the generated service. This file is managed by the
# language module — re-apply refreshes it.
.PHONY: build test lint run tidy

build:
	# -mod=readonly builds against the committed go.mod + go.sum (derived
	# artifacts, ADR-0018); run `make tidy` after changing imports to refresh
	# them, then commit.
	go build -mod=readonly ./...

test:
	go test ./...

lint:
	golangci-lint run ./...

run:
	go run ./cmd/server

tidy:
	go mod tidy
