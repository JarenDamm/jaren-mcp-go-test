// Owner module: language-go-mcp
//
// Package app is the composition seam for this service's MCP Foundation. The
// language module owns this file (it is managed: re-apply overwrites it, and
// the platform evolves its shape over time). Capability modules extend the
// service by dropping a self-registering file into this package whose init()
// calls Register — they never edit the developer's cmd/server entrypoint.
//
// The Foundation serves the Model Context Protocol over streamable HTTP
// (ADR-0015: the mcp archetype's identity is transport-agnostic; v1 ships
// streamable HTTP so K8s probes, the L3 smoke test, and real MCP clients can
// all reach a network endpoint). The registered tool/resource core lives on
// App.Server, separate from the entrypoint, so a future stdio transport can
// arrive as a capability contributing a second thin binary that reuses this
// same core — never by swapping this entrypoint.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/JarenDamm/jaren-mcp-go-test/internal/config"
)

// mcpServerName and mcpServerVersion are advertised to clients during MCP
// initialization. The platform owns these defaults; edit them on your own
// fork if you rename the service.
const (
	mcpServerName    = "github.com/JarenDamm/jaren-mcp-go-test"
	mcpServerVersion = "v1"
)

// mcpMountPath is the URL the streamable-HTTP MCP transport binds to. The
// SDK's StreamableHTTPHandler accepts both POST (client requests) and GET
// (server-initiated SSE stream) on this single path; clients point their base
// URL here.
const mcpMountPath = "/mcp"

// App is the contract a capability registers against. It carries the MCP
// server (where tools and resources are registered) plus the shared HTTP
// router that exposes the MCP transport alongside the Foundation's liveness
// and readiness probes, and the configuration and logger every handler needs.
// The platform owns its shape (this file is managed) and grows it over time
// via required updates, so add-ons compile against a stable target.
type App struct {
	// Server is the MCP server the SDK exposes. Capability registrations add
	// their tools and resources to it via mcp.AddTool / Server.AddResource.
	Server *mcp.Server
	// Mux is the shared HTTP router. It carries the streamable-HTTP MCP
	// transport at /mcp plus the Foundation's own /healthz and /readyz
	// probes. Capabilities that need a plain HTTP route (rare for an MCP
	// service) can mount it here alongside.
	Mux *http.ServeMux
	// Log is the structured logger; handlers and capabilities log through it.
	Log *slog.Logger
	// Cfg is the loaded runtime configuration.
	Cfg *config.Config
	// OnStart hooks acquire resources after the App is built and before the
	// server accepts traffic; Run invokes them in registration order. A
	// capability appends to OnStart during its registration.
	OnStart []func(context.Context) error
	// OnStop hooks release resources during graceful shutdown; Run invokes
	// them in reverse (LIFO) order, best-effort. The OnStart/OnStop slices are
	// independent (not index-paired), so on a failed start every registered
	// OnStop runs — an OnStop must tolerate a partial or skipped start (guard
	// its state, or append the OnStop from inside its OnStart only once the
	// resource is acquired).
	OnStop []func(context.Context) error
	// ready gates /readyz: false until startAll completes, and flipped back to
	// false when graceful shutdown begins so the load balancer drains this
	// replica. Capability hooks in this package may also Store(false) to shed
	// traffic while a dependency is degraded — it is safe for concurrent use.
	ready atomic.Bool
}

// Registration is a capability's hook into the Foundation: it mutates the
// composed App, typically by registering an MCP tool or resource on
// App.Server.
type Registration func(*App)

// registrations is populated by init() hooks in capability-shipped files in
// this package. New applies each entry to the composed App at startup.
var registrations []Registration

// Register adds a capability hook to the registry. Capability modules call it
// from an init() in their own file in this package, so adding a capability
// never requires editing the developer's entrypoint.
func Register(r Registration) {
	registrations = append(registrations, r)
}

// New builds the composed App from the configuration and logger: it creates
// the MCP server and the HTTP router, mounts the Foundation's own
// liveness/readiness endpoints plus the streamable-HTTP MCP transport, then
// applies every registered capability hook in registration order so each can
// add its tools and resources to the MCP server.
func New(cfg *config.Config, logger *slog.Logger) *App {
	a := &App{
		Server: mcp.NewServer(&mcp.Implementation{
			Name:    mcpServerName,
			Version: mcpServerVersion,
		}, nil),
		Mux: http.NewServeMux(),
		Log: logger,
		Cfg: cfg,
	}

	a.Mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	a.Mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		// Readiness is distinct from liveness (/healthz): it reports whether the
		// service should receive traffic, so it stays 503 until startup finishes
		// and again once shutdown begins.
		if !a.ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	for _, register := range registrations {
		register(a)
	}

	// Mount the MCP transport after capabilities have registered their tools
	// and resources, so the single shared *mcp.Server the handler serves
	// already carries the full surface. The getServer callback returns the
	// same server for every request: tool dispatch is stateless here, so one
	// server instance is shared across connections.
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return a.Server
	}, nil)
	a.Mux.Handle(mcpMountPath, mcpHandler)

	return a
}

// Run builds the App and serves it on cfg.Addr until the process receives
// SIGINT or SIGTERM (or the passed context is cancelled), then shuts the
// server down gracefully within a bounded timeout.
func Run(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	a := New(cfg, logger)

	// Acquire resources before serving. On the first failure, startAll has
	// already released what it wired (best-effort, reverse), so just abort.
	if err := a.startAll(ctx); err != nil {
		return err
	}
	// Release resources on the way out, after the server has stopped.
	defer a.shutdown()

	// Resources are wired; report ready so the load balancer routes traffic.
	a.ready.Store(true)

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: a.Mux,
		// Bound slow/idle clients (gosec G112, Slowloris/CWE-400). WriteTimeout
		// is intentionally left unset: the MCP streamable-HTTP transport opens a
		// long-lived SSE stream on GET /mcp, and a write deadline would truncate it.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("mcp server listening", "addr", cfg.Addr, "mcp_path", mcpMountPath)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received")
		// Fail readiness first so the load balancer stops routing new requests
		// while in-flight ones drain during srv.Shutdown below.
		a.ready.Store(false)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// startAll runs every OnStart hook in registration order. Startup is
// fail-fast: on the first error it releases everything wired so far via
// shutdown (reverse, best-effort) and returns the error, so the caller aborts
// and the process exits non-zero. A capability that needs no startup work
// simply registers no OnStart.
func (a *App) startAll(ctx context.Context) error {
	for _, start := range a.OnStart {
		if err := start(ctx); err != nil {
			a.shutdown()
			return fmt.Errorf("startup: %w", err)
		}
	}
	return nil
}

// shutdown runs every registered OnStop hook in reverse (LIFO) order,
// best-effort: a failing hook is logged and the remaining hooks still run, so
// one capability cannot strand another's cleanup. It uses a fresh bounded
// context because the run-loop's context is already cancelled by shutdown time.
func (a *App) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i := len(a.OnStop) - 1; i >= 0; i-- {
		if err := a.OnStop[i](ctx); err != nil {
			a.Log.Error("shutdown hook failed", "index", i, "err", err)
		}
	}
}
