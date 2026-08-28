// Package mcp provides a Model Context Protocol server that auto-generates
// tools from Kora's doctype registry. Supports stdio (for Claude Desktop) and
// HTTP (embedded in kora serve) transports.
package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/asenawritescode/kora/api/ai"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/asenawritescode/kora/doctype"
)

type Mode string

const (
	ModeValidationOnly Mode = "validation_only"
	ModeExecutable     Mode = "executable"
)

// Server wraps the MCP server with Kora registry awareness.
type Server struct {
	srv      *mcp.Server
	registry *doctype.Registry
	mode     Mode
}

// New creates a new MCP server populated with tools for all doctypes in the registry.
func New(reg *doctype.Registry, siteName string, mode Mode) *Server {
	if mode == "" {
		mode = ModeExecutable
	}
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "kora-" + siteName,
		Version: "1.0.0",
	}, nil)

	ks := &Server{srv: srv, registry: reg, mode: mode}
	ks.registerTools()
	return ks
}

// Run starts the MCP server on stdio transport (for Claude Desktop).
func (s *Server) Run(ctx context.Context) error {
	return s.srv.Run(ctx, &mcp.StdioTransport{})
}


func (s *Server) registerTools() {
	// Config generation tools.
	s.addConfigTools()

	if s.mode != ModeExecutable {
		return
	}

	catalog := ai.BuildToolCatalog(s.registry)
	for _, tool := range catalog.Tools {
		if tool.Source != "tenant" {
			continue
		}
		s.addProjectedTool(tool)
	}
}

func (s *Server) addConfigTools() {
	mcp.AddTool(s.srv, &mcp.Tool{
		Name:        "validate_yaml",
		Description: "Validate a Kora YAML configuration. Returns syntax errors with line numbers and 'did you mean?' suggestions.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"yaml": map[string]any{"type": "string", "description": "YAML content to validate"},
			},
			"required": []string{"yaml"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		Yaml string `json:"yaml"`
	}) (*mcp.CallToolResult, any, error) {
		errs, _, err := doctype.ValidateYAML([]byte(args.Yaml))
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "Validation error: " + err.Error()}},
			}, nil, nil
		}
		if len(errs) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "✓ YAML is valid"}},
			}, nil, nil
		}
		var lines []string
		for _, e := range errs {
			line := fmt.Sprintf("Line %d: %s", e.Line, e.Message)
			if e.Detail != "" {
				line += " (" + e.Detail + ")"
			}
			lines = append(lines, line)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: strings.Join(lines, "\n")}},
		}, nil, nil
	})
}

func (s *Server) addProjectedTool(tool ai.ToolDescriptor) {
	mcpTool := &mcp.Tool{
		Name:        tool.Name,
		Description: tool.Description,
		InputSchema: tool.InputSchema,
	}
	switch tool.Operation {
	case "find", "list", "get":
		mcp.AddTool(s.srv, mcpTool, func(ctx context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "Validation-only MCP deployment: execute this tool through the chat or API path."}},
			}, nil, nil
		})
	default:
		mcp.AddTool(s.srv, mcpTool, func(ctx context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Would call the %s path for %s.", tool.Operation, tool.Name)}},
			}, nil, nil
		})
	}
}

func sanitizeName(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}
