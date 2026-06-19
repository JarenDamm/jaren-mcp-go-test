# syntax=docker/dockerfile:1
# Owner module: language-go-mcp
#
# Multi-stage build for the generated Go MCP service. Follows the http
# Foundation's container conventions verbatim — the mcp Foundation serves
# streamable HTTP, so it containerizes identically: a network-reachable
# endpoint K8s probes and real MCP clients hit.
#
# Build stage uses golang:<go_version>-alpine, where <go_version>
# matches the toolchain pinned in go.mod (rendered from the same
# .Inputs.go_version input). Runtime is distroless/static:nonroot —
# no shell, no package manager, baked-in nonroot user.
FROM golang:1.26-alpine AS build
WORKDIR /src

# go.mod and go.sum are committed derived artifacts (ADR-0018: primus captures
# them from the verify step at apply time), so the build resolves against a
# pinned, hash-verified graph — the official MCP Go SDK included — rather than
# re-tidying at build time. `go mod download` fetches exactly what go.sum pins;
# the build runs -mod=readonly so a stale lock fails the build loudly instead of
# being silently rewritten.
COPY . .
RUN go mod download

# Static, stripped binary. CGO disabled so the binary runs in
# distroless/static (no libc). The entrypoint lives at ./cmd/server.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -mod=readonly \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/app \
    ./cmd/server

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
# The streamable-HTTP MCP transport listens on 8080 (config default); the
# /mcp path carries the protocol, /healthz and /readyz back the K8s probes.
EXPOSE 8080
ENTRYPOINT ["/app"]
