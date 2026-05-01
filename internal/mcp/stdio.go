package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"time"
)

var ErrTransportClosed = errors.New("mcp: transport closed")

const gracePeriod = 5 * time.Second

type msgResult struct {
	msg *Message
	err error
}

// It is not safe for concurrent use; synchronisation is the caller's responsibility.
type StdioTransport struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	msgs      <-chan msgResult
	done      chan struct{}
	exited    chan struct{} // closed when cmd.Wait returns
	closeOnce sync.Once
	writeMu   sync.Mutex
	waitErr   error
}

func newTransport(cmd *exec.Cmd, stdin io.WriteCloser, stdout io.ReadCloser, stderr io.ReadCloser) *StdioTransport {
	reader := bufio.NewReader(stdout)

	msgs := make(chan msgResult, 64)
	done := make(chan struct{})
	go func() {
		defer close(msgs)
		for {
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				var msg Message
				if jsonErr := json.Unmarshal(line, &msg); jsonErr != nil {
					select {
					case msgs <- msgResult{err: fmt.Errorf("unmarshal message: %w", jsonErr)}:
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
			if err != nil {
				if err != io.EOF {
					select {
					case msgs <- msgResult{err: err}:
					case <-done:
					}
				}
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

	exited := make(chan struct{})
	t := &StdioTransport{cmd: cmd, stdin: stdin, stdout: stdout, stderr: stderr, msgs: msgs, done: done, exited: exited}
	if cmd != nil {
		go func() {
			t.waitErr = cmd.Wait()
			close(exited)
		}()
	} else {
		close(exited)
	}
	return t
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

func (t *StdioTransport) shutdown() {
	t.closeOnce.Do(func() {
		close(t.done)
		if t.stdin != nil {
			_ = t.stdin.Close()
		}
		if t.stdout != nil {
			_ = t.stdout.Close()
		}
		if t.stderr != nil {
			_ = t.stderr.Close()
		}
	})
}

func (t *StdioTransport) kill() {
	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
}

func (t *StdioTransport) Send(ctx context.Context, msg *Message) error {
	select {
	case <-t.done:
		return ErrTransportClosed
	default:
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	data = append(data, '\n')

	errCh := make(chan error, 1)
	go func() {
		t.writeMu.Lock()
		defer t.writeMu.Unlock()

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
		return ctx.Err()
	case <-t.done:
		return ErrTransportClosed
	case err := <-errCh:
		return err
	}
}

func (t *StdioTransport) Receive(ctx context.Context) (*Message, error) {
	select {
	case <-t.done:
		return nil, ErrTransportClosed
	default:
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.done:
		return nil, ErrTransportClosed
	case res, ok := <-t.msgs:
		if !ok {
			return nil, io.EOF
		}
		return res.msg, res.err
	}
}

func (t *StdioTransport) Close() error {
	t.shutdown()
	if t.cmd == nil {
		return nil
	}
	select {
	case <-t.exited:
	case <-time.After(gracePeriod):
		t.kill()
		<-t.exited
	}
	return t.waitErr
}
