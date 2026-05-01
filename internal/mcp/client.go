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
		return nil, fmt.Errorf("mcp initialize: %w", err)
	}
	if err := client.listTools(ctx); err != nil {
		return nil, fmt.Errorf("mcp list tools: %w", err)
	}
	return client, nil
}

func (c *Client) Tools() []Tool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Tool(nil), c.tools...)
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
		msg := name + " error"
		if len(result.Content) > 0 {
			msg = result.Content[0].Text
		}
		return &result, fmt.Errorf("tool %s: %s", name, msg)
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
		if msg.ID == nil || *msg.ID != id {
			continue // skip notifications and mismatched responses
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
