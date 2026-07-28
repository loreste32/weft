package weft

import (
	"context"
	"fmt"
)

// MCPServe runs a Weft file as an MCP server on stdio.
// The file should call mcp.serve_stdio([tools]) in its main function.
func MCPServe(path string) error {
	ctx := New(Options{})
	if err := ctx.RunFile(context.Background(), path); err != nil {
		return fmt.Errorf("mcp serve %s: %w", path, err)
	}
	return nil
}
