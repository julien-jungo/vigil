package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
)

const (
	scannerInitBufSize = 64 * 1024        // 64 KB
	maxMessageSize     = 10 * 1024 * 1024 // 10 MB — screenshots can be large
)

// It is not safe for concurrent use; synchronisation is the caller's responsibility.
type StdioTransport struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
}

func NewStdioTransport(ctx context.Context, bin string, args ...string) (*StdioTransport, error) {
	cmd := exec.CommandContext(ctx, bin, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("start %s: %w", bin, err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, scannerInitBufSize), maxMessageSize)

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			slog.Debug("playwright-mcp", "msg", scanner.Text())
		}
	}()

	return &StdioTransport{cmd: cmd, stdin: stdin, scanner: scanner}, nil
}

func (t *StdioTransport) Send(_ context.Context, msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	data = append(data, '\n')

	for written := 0; written < len(data); {
		n, err := t.stdin.Write(data[written:])
		if err != nil {
			return fmt.Errorf("write stdin: %w", err)
		}
		if n == 0 {
			return fmt.Errorf("write stdin: %w", io.ErrShortWrite)
		}
		written += n
	}
	return nil
}

func (t *StdioTransport) Receive(_ context.Context) (*Message, error) {
	if !t.scanner.Scan() {
		if err := t.scanner.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
	var msg Message
	if err := json.Unmarshal(t.scanner.Bytes(), &msg); err != nil {
		return nil, fmt.Errorf("unmarshal message: %w", err)
	}
	return &msg, nil
}

func (t *StdioTransport) Close() error {
	_ = t.stdin.Close()
	if t.cmd == nil {
		return nil
	}
	return t.cmd.Wait()
}
