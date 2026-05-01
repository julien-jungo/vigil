package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

const (
	protocolVersion = "2024-11-05"
	jsonRPCVersion  = "2.0"
)

type Client struct {
	transport Transport
	tools     []Tool
	version   string
	nextID    int64
	mu        sync.Mutex
}

// version is the caller's build version and is reported to the server during initialisation.
func New(ctx context.Context, transport Transport, version string) (*Client, error) {
	client := &Client{transport: transport, version: version}
	if err := client.initialize(ctx); err != nil {
		_ = transport.Close()
		return nil, fmt.Errorf("mcp initialize: %w", err)
	}
	if err := client.listTools(ctx); err != nil {
		_ = transport.Close()
		return nil, fmt.Errorf("mcp list tools: %w", err)
	}
	return client, nil
}

func (c *Client) Tools() []Tool {
	c.mu.Lock()
	defer c.mu.Unlock()

	tools := make([]Tool, len(c.tools))
	for i, tool := range c.tools {
		tools[i] = tool
		tools[i].InputSchema = append(json.RawMessage(nil), tool.InputSchema...)
	}

	return tools
}

func (c *Client) Call(ctx context.Context, name string, args map[string]any) (*CallResult, error) {
	if args == nil {
		args = map[string]any{}
	}
	params, err := json.Marshal(map[string]any{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return nil, err
	}

	response, err := c.request(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}

	var result CallResult
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("unmarshal call result: %w", err)
	}
	if result.IsError {
		return nil, &ToolError{Tool: name, Content: result.Content}
	}
	return &result, nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.transport.Close()
}

func (c *Client) initialize(ctx context.Context) error {
	params, err := json.Marshal(map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "vigil", "version": c.version},
	})
	if err != nil {
		return fmt.Errorf("marshal initialize params: %w", err)
	}
	if _, err := c.request(ctx, "initialize", params); err != nil {
		return err
	}
	return c.notify(ctx, "notifications/initialized")
}

func (c *Client) listTools(ctx context.Context) error {
	result, err := c.request(ctx, "tools/list", json.RawMessage("{}"))
	if err != nil {
		return err
	}
	var resp struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return fmt.Errorf("unmarshal tools: %w", err)
	}
	c.tools = resp.Tools
	return nil
}

func (c *Client) request(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID
	c.nextID++

	if err := c.transport.Send(ctx, &Message{
		JSONRPC: jsonRPCVersion,
		ID:      &id,
		Method:  method,
		Params:  params,
	}); err != nil {
		return nil, fmt.Errorf("send %s: %w", method, err)
	}

	for {
		msg, err := c.transport.Receive(ctx)
		if err != nil {
			return nil, fmt.Errorf("receive %s: %w", method, err)
		}
		if msg.ID == nil {
			continue // notification, skip
		}
		if *msg.ID != id {
			return nil, fmt.Errorf("receive %s: unexpected response id %d (want %d)", method, *msg.ID, id)
		}
		if msg.Error != nil {
			return nil, msg.Error
		}
		return msg.Result, nil
	}
}

func (c *Client) notify(ctx context.Context, method string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.transport.Send(ctx, &Message{
		JSONRPC: jsonRPCVersion,
		Method:  method,
	})
}
