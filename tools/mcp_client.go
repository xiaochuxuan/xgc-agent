package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	defaultMCPClientTimeout = 30 * time.Second
	defaultMCPClientName    = "xgc-agent"
	defaultMCPClientVersion = "dev"
)

// MCPClient communicates with an MCP server via the standard mcp-go client.
type MCPClient struct {
	client     mcpclient.MCPClient
	ClientName string
	Version    string
}

// MCPToolDef describes a tool discovered from an MCP server.
type MCPToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// MCPClientConfig configures MCP client creation.
type MCPClientConfig struct {
	Endpoint   string
	Headers    map[string]string
	Timeout    time.Duration
	ClientName string
	Version    string
}

func NewMCPClient(endpoint string) *MCPClient {
	client, err := NewMCPClientWithConfig(MCPClientConfig{Endpoint: endpoint})
	if err != nil {
		panic(err)
	}
	return client
}

func NewMCPClientWithConfig(cfg MCPClientConfig) (*MCPClient, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("mcp client: empty endpoint")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultMCPClientTimeout
	}
	if strings.TrimSpace(cfg.ClientName) == "" {
		cfg.ClientName = defaultMCPClientName
	}
	if strings.TrimSpace(cfg.Version) == "" {
		cfg.Version = defaultMCPClientVersion
	}

	opts := []transport.StreamableHTTPCOption{
		transport.WithHTTPTimeout(cfg.Timeout),
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, transport.WithHTTPHeaders(cfg.Headers))
	}
	client, err := mcpclient.NewStreamableHttpClient(endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("mcp client: %w", err)
	}
	return &MCPClient{
		client:     client,
		ClientName: cfg.ClientName,
		Version:    cfg.Version,
	}, nil
}

func NewMCPClientFromRaw(client mcpclient.MCPClient, name string, version string) (*MCPClient, error) {
	if client == nil {
		return nil, fmt.Errorf("mcp client: nil client")
	}
	return &MCPClient{
		client:     client,
		ClientName: name,
		Version:    version,
	}, nil
}

func (c *MCPClient) Close() error {
	err := c.client.Close()
	if err != nil {
		return err
	}
	return nil
}

func (c *MCPClient) ensureInitialized(ctx context.Context) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("mcp client: nil client")
	}
	_, err := c.client.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "xgc-agent",
				Version: "dev",
			},
		},
	})
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "already") {
		return fmt.Errorf("mcp initialize: %w", err)
	}
	return nil
}

// DiscoverTools fetches the list of available tools from the MCP server.
func (c *MCPClient) DiscoverTools(ctx context.Context) ([]MCPToolDef, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, err
	}
	result, err := c.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("mcp discover: %w", err)
	}
	defs := make([]MCPToolDef, 0, len(result.Tools))
	for _, tool := range result.Tools {
		schema, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("mcp discover schema %s: %w", tool.Name, err)
		}
		defs = append(defs, MCPToolDef{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: json.RawMessage(schema),
		})
	}
	return defs, nil
}

// CallTool invokes a tool on the MCP server.
func (c *MCPClient) CallTool(ctx context.Context, name string, args json.RawMessage) (*mcp.CallToolResult, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, err
	}
	var arguments any = map[string]any{}
	if len(args) > 0 && string(args) != "null" {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return nil, fmt.Errorf("mcp call decode args: %w", err)
		}
	}
	result, err := c.client.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: arguments,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("mcp call: %w", err)
	}
	return result, nil
}
