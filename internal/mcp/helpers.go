// internal/mcp/helpers.go
package mcp

import (
	"encoding/json"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TextResult wraps a plain string as a successful MCP text result.
func TextResult(text string) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: text}},
	}
}

// ErrResult wraps an error as a failed MCP text result.
func ErrResult(err error) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: err.Error()}},
		IsError: true,
	}
}

// JSONResult marshals v to indented JSON and wraps it as a successful MCP text result.
func JSONResult(v any) *sdkmcp.CallToolResult {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ErrResult(err)
	}
	return TextResult(string(b))
}
