// Owner module: language-go-mcp
//
// An integration test for the sample tool — the pattern to copy when you add
// your own. It builds the App through the seam (so the hello tool and resource
// registered by hello.go's init() are mounted), serves it on an in-process
// httptest server, and drives the official MCP SDK's streamable-HTTP CLIENT
// against it: initialize handshake -> tools/call -> assert the hello response.
//
// The test is named TestMCP* so the module's smoke.sh can run exactly this
// machine-checked runtime acceptance at L3 verify time (`go test
// ./internal/app/ -run TestMCP`) — a deliberate module-local departure from
// the structural-only smoke convention (ADR-0015 mcp grill, 2026-06-06).
package app

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/JarenDamm/jaren-mcp-go-test/internal/config"
)

func TestMCP_HelloToolOverStreamableHTTP(t *testing.T) {
	a := New(
		&config.Config{Addr: ":0", LogLevel: "info"},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	srv := httptest.NewServer(a.Mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Boot a real MCP client over the streamable-HTTP transport. client.Connect
	// performs the initialize handshake; a non-nil session means it succeeded.
	client := mcp.NewClient(&mcp.Implementation{Name: "hello-test-client", Version: "v0"}, nil)
	transport := &mcp.StreamableClientTransport{Endpoint: srv.URL + "/mcp"}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("client.Connect (initialize handshake): %v", err)
	}
	defer func() { _ = session.Close() }()

	// The hello tool must show up in the advertised surface.
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var found bool
	for _, tool := range tools.Tools {
		if tool.Name == "hello" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("hello tool not advertised by the server; got %d tools", len(tools.Tools))
	}

	// tools/call: greet a named person and assert the echoed greeting comes
	// back in the result's text content.
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "hello",
		Arguments: map[string]any{"name": "primus"},
	})
	if err != nil {
		t.Fatalf("CallTool hello: %v", err)
	}
	if res.IsError {
		t.Fatalf("hello tool returned IsError=true: %+v", res.Content)
	}
	if len(res.Content) == 0 {
		t.Fatal("hello tool returned no content")
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("hello tool content[0] = %T, want *mcp.TextContent", res.Content[0])
	}
	if text.Text != "hello, primus" {
		t.Errorf("hello greeting = %q, want %q", text.Text, "hello, primus")
	}
}
