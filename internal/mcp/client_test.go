package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockTransport dispatches a pre-configured response for each request (identified
// by the presence of an ID field) in the order they are sent. Responses are only
// made available to Receive after the matching Send, mirroring real transport
// semantics and avoiding races with the background receive loop.
type mockTransport struct {
	mu        sync.Mutex
	sent      []*Message
	responses []*Message
	respIdx   int
	respCh    chan *Message
	closeCh   chan struct{}
	closeOnce sync.Once
}

func newMockTransport(responses ...*Message) *mockTransport {
	return &mockTransport{
		responses: responses,
		respCh:    make(chan *Message, len(responses)+1),
		closeCh:   make(chan struct{}),
	}
}

func (m *mockTransport) Send(_ context.Context, msg *Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, msg)
	if msg.ID != nil && m.respIdx < len(m.responses) {
		resp := m.responses[m.respIdx]
		if resp.ID != nil && *resp.ID == *msg.ID {
			m.respCh <- resp
			m.respIdx++
		}
	}
	return nil
}

func (m *mockTransport) Receive(ctx context.Context) (*Message, error) {
	select {
	case msg := <-m.respCh:
		return msg, nil
	case <-m.closeCh:
		return nil, ErrTransportClosed
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *mockTransport) Close() error {
	m.closeOnce.Do(func() { close(m.closeCh) })
	return nil
}

var testTools = []Tool{
	{Name: "browser_navigate", Description: "Navigate to a URL", InputSchema: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string"}},"required":["url"]}`)},
	{Name: "browser_click", Description: "Click an element", InputSchema: json.RawMessage(`{"type":"object","properties":{"selector":{"type":"string"}},"required":["selector"]}`)},
}

func initResponses() []*Message {
	toolsJSON, _ := json.Marshal(testTools)
	id0, id1 := int64(0), int64(1)
	return []*Message{
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
}

func newTestClient(t *testing.T, extra ...*Message) (*Client, *mockTransport) {
	t.Helper()
	mock := newMockTransport(append(initResponses(), extra...)...)
	client, err := New(context.Background(), mock, "test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client, mock
}

func TestNew_handshake(t *testing.T) {
	_, mock := newTestClient(t)

	mock.mu.Lock()
	sent := mock.sent
	mock.mu.Unlock()

	if len(sent) != 3 {
		t.Fatalf("sent %d messages, want 3", len(sent))
	}
	if sent[0].Method != "initialize" {
		t.Errorf("msg[0].Method = %q, want initialize", sent[0].Method)
	}
	if sent[1].Method != "notifications/initialized" {
		t.Errorf("msg[1].Method = %q, want notifications/initialized", sent[1].Method)
	}
	if sent[2].Method != "tools/list" {
		t.Errorf("msg[2].Method = %q, want tools/list", sent[2].Method)
	}
}

func TestNew_initializeSetsProtocolVersion(t *testing.T) {
	_, mock := newTestClient(t)

	mock.mu.Lock()
	params := mock.sent[0].Params
	mock.mu.Unlock()

	var p map[string]any
	if err := json.Unmarshal(params, &p); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if p["protocolVersion"] != protocolVersion {
		t.Errorf("protocolVersion = %q, want %q", p["protocolVersion"], protocolVersion)
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

	result, err := client.Call(context.Background(), "browser_click", map[string]any{"selector": "#missing"})
	if result != nil {
		t.Error("expected nil result for isError=true")
	}
	var toolErr *ToolError
	if !errors.As(err, &toolErr) {
		t.Fatalf("expected *ToolError, got %T: %v", err, err)
	}
	if toolErr.Tool != "browser_click" {
		t.Errorf("Tool = %q, want browser_click", toolErr.Tool)
	}
	if !strings.Contains(toolErr.Error(), "element not found") {
		t.Errorf("expected text content in error, got %v", toolErr)
	}
}

func TestCall_toolError_nonTextContent(t *testing.T) {
	id := int64(2)
	client, _ := newTestClient(t, &Message{
		JSONRPC: jsonRPCVersion,
		ID:      &id,
		Result:  json.RawMessage(`{"content":[{"type":"image","data":"abc=","mimeType":"image/png"}],"isError":true}`),
	})

	result, err := client.Call(context.Background(), "browser_screenshot", nil)
	if result != nil {
		t.Error("expected nil result for isError=true")
	}
	var toolErr *ToolError
	if !errors.As(err, &toolErr) {
		t.Fatalf("expected *ToolError, got %T: %v", err, err)
	}
	if !strings.Contains(toolErr.Error(), "browser_screenshot") {
		t.Errorf("expected tool name in fallback error, got %v", toolErr)
	}
}

func TestCall_skipsNotifications(t *testing.T) {
	id := int64(2)
	client, _ := newTestClient(t,
		// The notification has no ID so it does not consume a response slot.
		// The actual response is returned for the tools/call request.
		&Message{
			JSONRPC: jsonRPCVersion,
			ID:      &id,
			Result:  json.RawMessage(`{"content":[{"type":"text","text":"done"}],"isError":false}`),
		},
	)

	// Inject a notification directly into the response channel so the loop sees it
	// before the real response.
	client.mu.Lock()
	transport := client.transport.(*mockTransport)
	client.mu.Unlock()
	transport.respCh <- &Message{JSONRPC: "2.0", Method: "notifications/progress"}

	result, err := client.Call(context.Background(), "browser_navigate", map[string]any{"url": "https://example.com"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if result.Content[0].Text != "done" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestClose(t *testing.T) {
	mock := newMockTransport(initResponses()...)
	client, err := New(context.Background(), mock, "test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// blockingReceiveTransport serves the configured responses on Send (same as
// mockTransport), then blocks on Receive until the transport is closed, signalling
// receiveBlocked each time it enters the blocked state. closeErr is the error
// returned from Receive when the transport is closed; defaults to ErrTransportClosed.
type blockingReceiveTransport struct {
	mu             sync.Mutex
	sent           []*Message
	responses      []*Message
	respIdx        int
	respCh         chan *Message
	closeCh        chan struct{}
	closeOnce      sync.Once
	receiveBlocked chan struct{}
	closeErr       error
}

func (t *blockingReceiveTransport) Send(_ context.Context, msg *Message) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sent = append(t.sent, msg)
	if msg.ID != nil && t.respIdx < len(t.responses) {
		resp := t.responses[t.respIdx]
		if resp.ID != nil && *resp.ID == *msg.ID {
			t.respCh <- resp
			t.respIdx++
		}
	}
	return nil
}

func (t *blockingReceiveTransport) closeError() error {
	if t.closeErr != nil {
		return t.closeErr
	}
	return ErrTransportClosed
}

func (t *blockingReceiveTransport) Receive(ctx context.Context) (*Message, error) {
	// Return any already-queued responses immediately.
	select {
	case msg := <-t.respCh:
		return msg, nil
	default:
	}
	// Signal that we are entering the blocked state, then wait.
	select {
	case t.receiveBlocked <- struct{}{}:
	case msg := <-t.respCh:
		return msg, nil
	case <-t.closeCh:
		return nil, t.closeError()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case msg := <-t.respCh:
		return msg, nil
	case <-t.closeCh:
		return nil, t.closeError()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t *blockingReceiveTransport) Close() error {
	t.closeOnce.Do(func() { close(t.closeCh) })
	return nil
}

func TestClient_cancelDuringReceive_sessionRemainsUsable(t *testing.T) {
	// id=2: first Call — ctx will be canceled before the response arrives (no
	// response is configured so the loop blocks, keeping the session in a clean
	// state when the caller gives up).
	// id=3: second Call — has a response; must succeed after the first is canceled.
	id3 := int64(3)
	callResp := &Message{
		JSONRPC: jsonRPCVersion,
		ID:      &id3,
		Result:  json.RawMessage(`{"content":[{"type":"text","text":"ok"}],"isError":false}`),
	}

	transport := &blockingReceiveTransport{
		responses:      append(initResponses(), callResp),
		respCh:         make(chan *Message, 10),
		closeCh:        make(chan struct{}),
		receiveBlocked: make(chan struct{}, 1),
	}

	client, err := New(context.Background(), transport, "test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callDone := make(chan error, 1)
	go func() {
		_, err := client.Call(ctx, "browser_navigate", map[string]any{"url": "http://example.com"})
		callDone <- err
	}()

	// Wait until the receive loop is blocked (i.e. first Call's request is in
	// flight and no response has arrived yet), then cancel the context.
	select {
	case <-transport.receiveBlocked:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for receive loop to block")
	}
	cancel()

	select {
	case err := <-callDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("first Call err = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for first Call to return")
	}

	// The second Call should succeed: the session is still alive and id=3 has a
	// configured response.
	result, err := client.Call(context.Background(), "browser_navigate", map[string]any{"url": "http://example.com"})
	if err != nil {
		t.Fatalf("second Call: %v", err)
	}
	if result.Content[0].Text != "ok" {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestClient_CloseDeadlock(t *testing.T) {
	transport := &blockingReceiveTransport{
		responses:      initResponses(),
		respCh:         make(chan *Message, 10),
		closeCh:        make(chan struct{}),
		receiveBlocked: make(chan struct{}, 1),
	}

	client, err := New(context.Background(), transport, "test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	callDone := make(chan struct{})
	go func() {
		_, _ = client.Call(context.Background(), "browser_navigate", map[string]any{"url": "http://example.com"})
		close(callDone)
	}()

	select {
	case <-transport.receiveBlocked:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for Receive to be called")
	}

	closeDone := make(chan struct{})
	go func() {
		_ = client.Close()
		close(closeDone)
	}()

	select {
	case <-closeDone:
	case <-time.After(2 * time.Second):
		t.Error("Close() blocked for too long")
	}
}

func TestClient_transportError_failsPendingRequests(t *testing.T) {
	// Simulate the server process exiting: Receive returns io.EOF.
	transport := &blockingReceiveTransport{
		responses:      initResponses(),
		respCh:         make(chan *Message, 10),
		closeCh:        make(chan struct{}),
		receiveBlocked: make(chan struct{}, 1),
		closeErr:       io.EOF,
	}

	client, err := New(context.Background(), transport, "test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	callDone := make(chan error, 1)
	go func() {
		_, err := client.Call(context.Background(), "browser_navigate", map[string]any{"url": "http://example.com"})
		callDone <- err
	}()

	select {
	case <-transport.receiveBlocked:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for Receive to block")
	}

	// Simulate server process exit.
	_ = transport.Close()

	// In-flight call must surface the real transport error, not ErrTransportClosed.
	select {
	case err := <-callDone:
		if !errors.Is(err, io.EOF) {
			t.Errorf("in-flight Call err = %v, want io.EOF", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for Call to return after transport error")
	}

	// Subsequent calls must also surface the real error immediately.
	_, err = client.Call(context.Background(), "browser_navigate", nil)
	if !errors.Is(err, io.EOF) {
		t.Errorf("subsequent Call err = %v, want io.EOF", err)
	}
}
