// Owner module: language-go-mcp
//
// These tests assert the Foundation's contract: the seam applies registered
// capability hooks, the liveness/readiness endpoints respond, and the
// streamable-HTTP MCP transport is mounted. They ship managed alongside app.go
// so the contract travels with the file the platform owns.
package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/JarenDamm/jaren-mcp-go-test/internal/config"
)

// newTestApp builds an App with a throwaway config and a discard logger.
// Capability modules use init() hooks to register lifecycle callbacks that
// touch real I/O. The lifecycle tests below assert ordering against hooks they
// install themselves, so we reset OnStart/OnStop after construction to keep
// those hooks out of the way. Server/tool registrations stay — they're inert
// for these contract assertions.
func newTestApp() *App {
	a := New(
		&config.Config{Addr: ":0", LogLevel: "info"},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	a.OnStart = nil
	a.OnStop = nil
	return a
}

func TestNew_MountsHealthAndReadyEndpoints(t *testing.T) {
	a := newTestApp()
	for _, path := range []string{"/healthz", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		a.Mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want %d", path, rec.Code, http.StatusOK)
		}
	}
}

// TestNew_MountsMCPTransport asserts the streamable-HTTP MCP handler is bound
// at /mcp. A bare GET (no MCP session) won't complete a handshake, but the
// handler must be reachable — a 404 here means the transport never mounted,
// which the full client round-trip in hello_test.go would also catch but more
// slowly. Any non-404 status proves the route exists.
func TestNew_MountsMCPTransport(t *testing.T) {
	a := newTestApp()
	req := httptest.NewRequest(http.MethodGet, mcpMountPath, nil)
	rec := httptest.NewRecorder()
	a.Mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusNotFound {
		t.Errorf("GET %s = 404, want the MCP transport to be mounted", mcpMountPath)
	}
}

func TestNew_BuildsMCPServer(t *testing.T) {
	a := newTestApp()
	if a.Server == nil {
		t.Fatal("App.Server is nil; New did not construct the MCP server")
	}
}

func TestNew_AppliesRegisteredCapabilities(t *testing.T) {
	var applied bool
	Register(func(*App) {
		applied = true
	})

	_ = newTestApp()
	if !applied {
		t.Error("registered capability hook was not applied by New (registry not wired)")
	}
}

func TestApp_Lifecycle_StartInOrder_StopInReverse(t *testing.T) {
	a := newTestApp()
	var events []string
	a.OnStart = append(a.OnStart, func(context.Context) error { events = append(events, "start-A"); return nil })
	a.OnStop = append(a.OnStop, func(context.Context) error { events = append(events, "stop-A"); return nil })
	a.OnStart = append(a.OnStart, func(context.Context) error { events = append(events, "start-B"); return nil })
	a.OnStop = append(a.OnStop, func(context.Context) error { events = append(events, "stop-B"); return errors.New("close failed") })

	if err := a.startAll(context.Background()); err != nil {
		t.Fatalf("startAll: %v", err)
	}
	a.shutdown() // best-effort: stop-B errors, but stop-A must still run

	want := []string{"start-A", "start-B", "stop-B", "stop-A"}
	if !slices.Equal(events, want) {
		t.Errorf("lifecycle order = %v, want %v", events, want)
	}
}

func TestApp_StartAll_FailFast_RunsRegisteredStops(t *testing.T) {
	a := newTestApp()
	var events []string
	a.OnStart = append(a.OnStart, func(context.Context) error { events = append(events, "start-A"); return nil })
	a.OnStop = append(a.OnStop, func(context.Context) error { events = append(events, "stop-A"); return nil })
	a.OnStart = append(a.OnStart, func(context.Context) error { events = append(events, "start-B"); return errors.New("resource unreachable") })
	a.OnStop = append(a.OnStop, func(context.Context) error { events = append(events, "stop-B"); return nil })
	// A later OnStart must never run once B fails (fail-fast).
	a.OnStart = append(a.OnStart, func(context.Context) error { events = append(events, "start-C"); return nil })

	if err := a.startAll(context.Background()); err == nil {
		t.Fatal("expected startAll to return an error when an OnStart fails")
	}

	// B failed → C never ran; startAll unwound by running the registered
	// OnStops in reverse.
	want := []string{"start-A", "start-B", "stop-B", "stop-A"}
	if !slices.Equal(events, want) {
		t.Errorf("events = %v, want %v", events, want)
	}
}
