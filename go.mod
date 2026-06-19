// BEGIN MANAGED: go-module
module github.com/JarenDamm/jaren-mcp-go-test

go 1.26

// The official Model Context Protocol Go SDK is a Foundation dependency: the
// managed internal/app seam imports it to construct the *mcp.Server and serve
// it over streamable HTTP. Capability modules append their own require entries
// inside their own go.mod regions, never into this one.
require github.com/modelcontextprotocol/go-sdk v1.6.1
// END MANAGED
