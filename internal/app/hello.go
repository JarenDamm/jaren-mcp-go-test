// Owner module: language-go-mcp
//
// hello.go is a worked example of the extension seam — and it's yours to edit
// or delete (scaffolded). It self-registers an MCP tool and an example MCP
// resource via Register, reads the shared logger off the App, and serves a
// greeting. Add your own tools and resources the same way: a file in package
// app with an init() that calls Register.
package app

import (
	"context"
	"fmt"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// HelloInput is the typed input for the hello tool. The SDK infers the tool's
// JSON input schema from this struct via reflection; the jsonschema tag
// becomes the field's description in the advertised schema.
type HelloInput struct {
	Name string `json:"name" jsonschema:"the name to greet"`
}

// HelloOutput is the typed output for the hello tool. The SDK marshals it into
// the call result's structured content alongside the text content below.
type HelloOutput struct {
	Greeting string `json:"greeting"`
}

func init() {
	Register(func(a *App) {
		// A typed tool: the SDK reflects HelloInput/HelloOutput into the
		// advertised JSON schemas, decodes the client's arguments into
		// HelloInput, and marshals the returned HelloOutput into the result's
		// structured content. The text content mirrors the greeting so plain
		// clients that only render text still see it.
		mcp.AddTool(a.Server, &mcp.Tool{
			Name:        "hello",
			Description: "Greet the named person.",
		}, func(_ context.Context, _ *mcp.CallToolRequest, in HelloInput) (*mcp.CallToolResult, HelloOutput, error) {
			name := in.Name
			if name == "" {
				name = "world"
			}
			a.Log.Info("greeting", "name", name)
			greeting := fmt.Sprintf("hello, %s", name)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: greeting}},
			}, HelloOutput{Greeting: greeting}, nil
		})

		// An example resource: a static greeting document the client can read
		// by URI. Resources expose read-only context to the model; this one is
		// the pattern to copy for your own.
		a.Server.AddResource(&mcp.Resource{
			URI:         "greeting://hello",
			Name:        "hello-greeting",
			Description: "A static example greeting resource.",
			MIMEType:    "text/plain",
		}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			content := &mcp.ResourceContents{
				URI:      "greeting://hello",
				MIMEType: "text/plain",
				Text:     "hello, world",
			}
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{content},
			}, nil
		})
	})
}
