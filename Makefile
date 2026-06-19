# Owner module: language-go-mcp
#
# Convenience targets for the generated service. This file is managed by the
# language module — re-apply refreshes it.
.PHONY: build test lint run tidy

build:
	# -mod=readonly builds against the committed go.sum (ADR-0017); run
	# `make tidy` after changing deps to refresh go.sum, then commit it.
	go build -mod=readonly ./...

test:
	go test ./...

lint:
	golangci-lint run ./...

run:
	go run ./cmd/server

tidy:
	go mod tidy
