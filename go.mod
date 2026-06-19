// BEGIN MANAGED: go-module
module github.com/JarenDamm/jaren-mcp-go-test

go 1.26

// The official Model Context Protocol Go SDK is a Foundation dependency: the
// managed internal/app seam imports it to construct the *mcp.Server and serve
// it over streamable HTTP. Capability modules append their own require entries
// inside their own go.mod regions, never into this one.
require github.com/modelcontextprotocol/go-sdk v1.6.1

require (
	github.com/google/jsonschema-go v0.4.3 // indirect
	github.com/segmentio/asm v1.1.3 // indirect
	github.com/segmentio/encoding v0.5.4 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/oauth2 v0.35.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
)

// END MANAGED
