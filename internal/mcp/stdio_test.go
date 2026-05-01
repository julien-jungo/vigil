package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func newPipedTransport(t *testing.T) (*StdioTransport, io.Reader, *io.PipeWriter) {
	t.Helper()
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	transport := newTransport(nil, stdinW, stdoutR, io.NopCloser(strings.NewReader("")))

	t.Cleanup(func() {
		_ = stdinR.Close()
		_ = stdoutW.Close()
	})

	return transport, stdinR, stdoutW
}

func TestStdioTransport_Send(t *testing.T) {
	transport, stdinR, _ := newPipedTransport(t)

	id := int64(1)
	msg := &Message{JSONRPC: jsonRPCVersion, ID: &id, Method: "tools/list"}

	done := make(chan error, 1)
	go func() {
		done <- transport.Send(context.Background(), msg)
	}()

	scanner := bufio.NewScanner(stdinR)
	if !scanner.Scan() {
		t.Fatal("expected a line from stdin pipe")
	}

	if err := <-done; err != nil {
		t.Fatalf("Send: %v", err)
	}

	var got Message
	if err := json.Unmarshal(scanner.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Method != "tools/list" {
		t.Errorf("Method = %q, want tools/list", got.Method)
	}
	if got.ID == nil || *got.ID != 1 {
		t.Errorf("ID = %v, want 1", got.ID)
	}
}

func TestStdioTransport_Receive(t *testing.T) {
	transport, _, stdoutW := newPipedTransport(t)

	id := int64(42)
	msg := &Message{JSONRPC: jsonRPCVersion, ID: &id, Result: json.RawMessage(`{"ok":true}`)}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	go func() {
		_, _ = stdoutW.Write(append(data, '\n'))
	}()

	got, err := transport.Receive(context.Background())
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if got.ID == nil || *got.ID != 42 {
		t.Errorf("ID = %v, want 42", got.ID)
	}
}

func TestStdioTransport_Receive_EOF(t *testing.T) {
	transport, _, stdoutW := newPipedTransport(t)
	_ = stdoutW.Close()

	_, err := transport.Receive(context.Background())
	if err != io.EOF {
		t.Errorf("err = %v, want io.EOF", err)
	}
}

func TestStdioTransport_Receive_InvalidJSON(t *testing.T) {
	transport, _, stdoutW := newPipedTransport(t)

	go func() {
		_, _ = io.WriteString(stdoutW, "not-json\n")
	}()

	_, err := transport.Receive(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got %v", err)
	}
}

func TestStdioTransport_Receive_ContextCancelled(t *testing.T) {
	transport, _, _ := newPipedTransport(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := transport.Receive(ctx)
	if err != context.Canceled {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestStdioTransport_Send_ContextCancelled(t *testing.T) {
	transport, _, _ := newPipedTransport(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	id := int64(1)
	err := transport.Send(ctx, &Message{JSONRPC: jsonRPCVersion, ID: &id, Method: "tools/list"})
	if err != context.Canceled {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestStdioTransport_Send_ContextCancelledDuringWrite(t *testing.T) {
	// stdinR is never read, so io.Pipe blocks the first Write unconditionally.
	transport, _, _ := newPipedTransport(t)

	ctx, cancel := context.WithCancel(context.Background())

	id := int64(1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- transport.Send(ctx, &Message{JSONRPC: jsonRPCVersion, ID: &id, Method: "ping"})
	}()

	cancel()

	if err := <-errCh; err != context.Canceled {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestStdioTransport_Close_NilCmd(t *testing.T) {
	transport, _, _ := newPipedTransport(t)
	if err := transport.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestStdioTransport_Send_AfterClose(t *testing.T) {
	transport, _, _ := newPipedTransport(t)
	_ = transport.Close()

	id := int64(1)
	err := transport.Send(context.Background(), &Message{JSONRPC: jsonRPCVersion, ID: &id, Method: "ping"})
	if !errors.Is(err, ErrTransportClosed) {
		t.Errorf("err = %v, want ErrTransportClosed", err)
	}
}

func TestStdioTransport_Receive_AfterClose(t *testing.T) {
	transport, _, _ := newPipedTransport(t)
	_ = transport.Close()

	_, err := transport.Receive(context.Background())
	if !errors.Is(err, ErrTransportClosed) {
		t.Errorf("err = %v, want ErrTransportClosed", err)
	}
}
