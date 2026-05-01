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

type msgResult struct {
	msg *Message
	err error
}

// It is not safe for concurrent use; synchronisation is the caller's responsibility.
type StdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	msgs   <-chan msgResult
	done   chan struct{}
}

func newTransport(cmd *exec.Cmd, stdin io.WriteCloser, stdout io.ReadCloser, stderr io.Reader) *StdioTransport {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, scannerInitBufSize), maxMessageSize)

	msgs := make(chan msgResult, 1)
	done := make(chan struct{})
	go func() {
		defer close(msgs)
		for {
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					select {
					case msgs <- msgResult{err: err}:
					case <-done:
					}
				}
				return
			}
			var msg Message
			if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
				select {
				case msgs <- msgResult{err: fmt.Errorf("unmarshal message: %w", err)}:
				case <-done:
				}
				return
			}
			select {
			case msgs <- msgResult{msg: &msg}:
			case <-done:
				return
			}
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			slog.Debug("playwright-mcp", "msg", scanner.Text())
		}
	}()

	return &StdioTransport{cmd: cmd, stdin: stdin, stdout: stdout, msgs: msgs, done: done}
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

	return newTransport(cmd, stdin, stdout, stderr), nil
}

func (t *StdioTransport) Send(ctx context.Context, msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	data = append(data, '\n')

	errCh := make(chan error, 1)
	go func() {
		for written := 0; written < len(data); {
			n, err := t.stdin.Write(data[written:])
			if err != nil {
				errCh <- fmt.Errorf("write stdin: %w", err)
				return
			}
			if n == 0 {
				errCh <- fmt.Errorf("write stdin: %w", io.ErrShortWrite)
				return
			}
			written += n
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		_ = t.stdin.Close() // unblock the write goroutine
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (t *StdioTransport) Receive(ctx context.Context) (*Message, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res, ok := <-t.msgs:
		if !ok {
			return nil, io.EOF
		}
		return res.msg, res.err
	}
}

func (t *StdioTransport) Close() error {
	if t.done != nil {
		close(t.done)
	}
	if t.stdin != nil {
		_ = t.stdin.Close()
	}
	if t.stdout != nil {
		_ = t.stdout.Close()
	}
	if t.cmd == nil {
		return nil
	}
	return t.cmd.Wait()
}
