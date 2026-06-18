// Command server is the entrypoint for github.com/JarenDamm/jaren-mcp-go-test.
//
// This file is yours (scaffolded): the scaffolder renders it once and never
// overwrites it. It loads configuration and constructs the logger, then hands
// both to package app — which owns the MCP server lifecycle and serves it over
// streamable HTTP (managed). Capability modules extend the service by
// registering tools and resources against package app, so you do not edit this
// file to wire them in.
//
// A future stdio transport, if the platform ships one, arrives as a capability
// contributing a SECOND scaffolded entrypoint that reuses package app's
// registered tool/resource core (ADR-0015) — not as a change to this file.
package main

import (
	"context"
	"os"

	"github.com/JarenDamm/jaren-mcp-go-test/internal/app"
	"github.com/JarenDamm/jaren-mcp-go-test/internal/config"
	"github.com/JarenDamm/jaren-mcp-go-test/internal/logging"
)

func main() {
	cfg := config.Load()
	logger := logging.New(cfg)

	if err := app.Run(context.Background(), cfg, logger); err != nil {
		logger.Error("server exited with error", "err", err)
		os.Exit(1)
	}
}
