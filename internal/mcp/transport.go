package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

type Transport interface {
	Send(ctx context.Context, msg *Message) error
	Receive(ctx context.Context) (*Message, error)
	Close() error
}

type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("mcp error %d: %s", e.Code, e.Message)
}

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type CallResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError"`
}

type ToolError struct {
	Tool    string
	Content []ContentItem
}

func (e *ToolError) Error() string {
	for _, c := range e.Content {
		if c.Type == "text" && c.Text != "" {
			return fmt.Sprintf("tool %s: %s", e.Tool, c.Text)
		}
	}
	return fmt.Sprintf("tool %s: error", e.Tool)
}

type ContentItem struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"` // base64-encoded for images
	MIMEType string `json:"mimeType,omitempty"`
}
