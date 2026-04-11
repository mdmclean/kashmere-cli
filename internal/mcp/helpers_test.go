// internal/mcp/helpers_test.go
package mcp_test

import (
	"errors"
	"testing"

	internalmcp "github.com/mdmclean/kashmere-cli/internal/mcp"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestTextResult(t *testing.T) {
	r := internalmcp.TextResult("hello")
	if r == nil {
		t.Fatal("got nil result")
	}
	if r.IsError {
		t.Error("IsError should be false")
	}
	if len(r.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(r.Content))
	}
	tc, ok := r.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("expected *sdkmcp.TextContent, got %T", r.Content[0])
	}
	if tc.Text != "hello" {
		t.Errorf("got %q, want %q", tc.Text, "hello")
	}
}

func TestErrResult(t *testing.T) {
	r := internalmcp.ErrResult(errors.New("boom"))
	if r == nil {
		t.Fatal("got nil result")
	}
	if !r.IsError {
		t.Error("IsError should be true")
	}
	if len(r.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(r.Content))
	}
	tc, ok := r.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("expected *sdkmcp.TextContent, got %T", r.Content[0])
	}
	if tc.Text != "boom" {
		t.Errorf("got %q, want %q", tc.Text, "boom")
	}
}

func TestJSONResult(t *testing.T) {
	r := internalmcp.JSONResult(map[string]string{"key": "value"})
	if r == nil {
		t.Fatal("got nil result")
	}
	if r.IsError {
		t.Error("IsError should be false")
	}
	tc, ok := r.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("expected *sdkmcp.TextContent, got %T", r.Content[0])
	}
	if tc.Text == "" {
		t.Error("expected non-empty JSON text")
	}
}
