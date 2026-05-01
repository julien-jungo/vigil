package mcp

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

type mockTransport struct {
	sent []*Message
	recv []*Message
	idx  int
}

func (m *mockTransport) Send(_ context.Context, msg *Message) error {
	m.sent = append(m.sent, msg)
	return nil
}

func (m *mockTransport) Receive(_ context.Context) (*Message, error) {
	if m.idx >= len(m.recv) {
		return nil, io.EOF
	}
	msg := m.recv[m.idx]
	m.idx++
	return msg, nil
}

func (m *mockTransport) Close() error { return nil }

var testTools = []Tool{
	{Name: "browser_navigate", Description: "Navigate to a URL", InputSchema: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string"}},"required":["url"]}`)},
	{Name: "browser_click", Description: "Click an element", InputSchema: json.RawMessage(`{"type":"object","properties":{"selector":{"type":"string"}},"required":["selector"]}`)},
}

func initResponses(extra ...*Message) []*Message {
	toolsJSON, _ := json.Marshal(testTools)
	id0, id1 := int64(0), int64(1)
	msgs := []*Message{
		{
			JSONRPC: jsonRPCVersion,
			ID:      &id0,
			Result:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"playwright","version":"1.0"}}`),
		},
		{
			JSONRPC: jsonRPCVersion,
			ID:      &id1,
			Result:  json.RawMessage(`{"tools":` + string(toolsJSON) + `}`),
		},
	}
	return append(msgs, extra...)
}

func newTestClient(t *testing.T, extra ...*Message) (*Client, *mockTransport) {
	t.Helper()
	mock := &mockTransport{recv: initResponses(extra...)}
	client, err := New(context.Background(), mock, "test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client, mock
}

func TestNew_handshake(t *testing.T) {
	_, mock := newTestClient(t)

	if len(mock.sent) != 3 {
		t.Fatalf("sent %d messages, want 3", len(mock.sent))
	}
	if mock.sent[0].Method != "initialize" {
		t.Errorf("msg[0].Method = %q, want initialize", mock.sent[0].Method)
	}
	if mock.sent[1].Method != "notifications/initialized" {
		t.Errorf("msg[1].Method = %q, want notifications/initialized", mock.sent[1].Method)
	}
	if mock.sent[2].Method != "tools/list" {
		t.Errorf("msg[2].Method = %q, want tools/list", mock.sent[2].Method)
	}
}

func TestNew_initializeSetsProtocolVersion(t *testing.T) {
	_, mock := newTestClient(t)

	var params map[string]any
	if err := json.Unmarshal(mock.sent[0].Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params["protocolVersion"] != protocolVersion {
		t.Errorf("protocolVersion = %q, want %q", params["protocolVersion"], protocolVersion)
	}
}

func TestTools(t *testing.T) {
	client, _ := newTestClient(t)
	tools := client.Tools()
	if len(tools) != 2 {
		t.Fatalf("got %d tools, want 2", len(tools))
	}
	if tools[0].Name != "browser_navigate" {
		t.Errorf("tools[0].Name = %q", tools[0].Name)
	}
	if tools[1].Name != "browser_click" {
		t.Errorf("tools[1].Name = %q", tools[1].Name)
	}
}

func TestCall_success(t *testing.T) {
	id := int64(2)
	client, _ := newTestClient(t, &Message{
		JSONRPC: jsonRPCVersion,
		ID:      &id,
		Result:  json.RawMessage(`{"content":[{"type":"text","text":"Navigated"}],"isError":false}`),
	})

	result, err := client.Call(context.Background(), "browser_navigate", map[string]any{"url": "https://example.com"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "Navigated" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestCall_rpcError(t *testing.T) {
	id := int64(2)
	client, _ := newTestClient(t, &Message{
		JSONRPC: jsonRPCVersion,
		ID:      &id,
		Error:   &RPCError{Code: -32601, Message: "method not found"},
	})

	_, err := client.Call(context.Background(), "unknown_tool", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCall_toolError(t *testing.T) {
	id := int64(2)
	client, _ := newTestClient(t, &Message{
		JSONRPC: jsonRPCVersion,
		ID:      &id,
		Result:  json.RawMessage(`{"content":[{"type":"text","text":"element not found"}],"isError":true}`),
	})

	_, err := client.Call(context.Background(), "browser_click", map[string]any{"selector": "#missing"})
	if err == nil {
		t.Fatal("expected error for isError=true")
	}
	if !strings.Contains(err.Error(), "element not found") {
		t.Errorf("expected text content in error, got %v", err)
	}
}

func TestCall_toolError_nonTextContent(t *testing.T) {
	id := int64(2)
	client, _ := newTestClient(t, &Message{
		JSONRPC: jsonRPCVersion,
		ID:      &id,
		Result:  json.RawMessage(`{"content":[{"type":"image","data":"abc=","mimeType":"image/png"}],"isError":true}`),
	})

	_, err := client.Call(context.Background(), "browser_screenshot", nil)
	if err == nil {
		t.Fatal("expected error for isError=true")
	}
	if !strings.Contains(err.Error(), "browser_screenshot error") {
		t.Errorf("expected generic fallback error, got %v", err)
	}
}

func TestCall_skipsNotifications(t *testing.T) {
	id := int64(2)
	client, _ := newTestClient(t,
		&Message{JSONRPC: "2.0", Method: "notifications/progress"},
		&Message{
			JSONRPC: jsonRPCVersion,
			ID:      &id,
			Result:  json.RawMessage(`{"content":[{"type":"text","text":"done"}],"isError":false}`),
		},
	)

	result, err := client.Call(context.Background(), "browser_navigate", map[string]any{"url": "https://example.com"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if result.Content[0].Text != "done" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestClose(t *testing.T) {
	client, _ := newTestClient(t)
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
