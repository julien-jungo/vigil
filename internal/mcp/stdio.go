package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

var ErrTransportClosed = errors.New("mcp: transport closed")

const gracePeriod = 5 * time.Second

type response struct {
	msg *Message
	err error
}

type request struct {
	msg  *Message
	done chan<- error
	ctx  context.Context
}

type StdioTransport struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	reader    *bufio.Reader
	writer    *bufio.Writer
	encoder   *json.Encoder
	requests  chan request
	responses chan response
	closed    chan struct{}
	exited    chan struct{}
	closeOnce sync.Once
	exitErr   error
}

func (t *StdioTransport) sendLoop() {
	defer func() {
		for {
			select {
			case req := <-t.requests:
				req.done <- ErrTransportClosed
			default:
				return
			}
		}
	}()
	for {
		select {
		case <-t.closed:
			return
		case req := <-t.requests:
			select {
			case <-req.ctx.Done():
				req.done <- req.ctx.Err()
				continue
			default:
				var writeErr error
				if err := t.encoder.Encode(req.msg); err != nil {
					writeErr = fmt.Errorf("write stdin: %w", err)
				} else if err := t.writer.Flush(); err != nil {
					writeErr = fmt.Errorf("write stdin: %w", err)
				}
				req.done <- writeErr
				if writeErr != nil {
					t.shutdown()
					return
				}
			}
		}
	}
}

func (t *StdioTransport) recvLoop() {
	defer close(t.responses)
	for {
		line, err := t.reader.ReadBytes('\n')
		if len(line) > 0 {
			var msg Message
			if jsonErr := json.Unmarshal(line, &msg); jsonErr != nil {
				select {
				case t.responses <- response{err: fmt.Errorf("unmarshal message: %w", jsonErr)}:
				case <-t.closed:
				}
				return
			}
			select {
			case t.responses <- response{msg: &msg}:
			case <-t.closed:
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				select {
				case t.responses <- response{err: err}:
				case <-t.closed:
				}
			}
			return
		}
	}
}

func newTransport(cmd *exec.Cmd, stdin io.WriteCloser, stdout io.ReadCloser, stderr io.ReadCloser) *StdioTransport {
	reader := bufio.NewReader(stdout)
	writer := bufio.NewWriter(stdin)
	exited := make(chan struct{})

	t := &StdioTransport{
		cmd:       cmd,
		stdin:     stdin,
		stdout:    stdout,
		stderr:    stderr,
		reader:    reader,
		writer:    writer,
		encoder:   json.NewEncoder(writer),
		responses: make(chan response, 64),
		requests:  make(chan request, 64),
		closed:    make(chan struct{}),
		exited:    exited,
	}
	if cmd != nil {
		go func() {
			t.exitErr = cmd.Wait()
			close(exited)
		}()
	} else {
		close(exited)
	}

	go t.sendLoop()
	go t.recvLoop()

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

func (t *StdioTransport) Send(ctx context.Context, msg *Message) error {
	done := make(chan error, 1)
	select {
	case t.requests <- request{msg: msg, done: done, ctx: ctx}:
	case <-ctx.Done():
		return ctx.Err()
	case <-t.closed:
		return ErrTransportClosed
	}

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-t.closed:
		return ErrTransportClosed
	}
}

func (t *StdioTransport) Receive(ctx context.Context) (*Message, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.closed:
		return nil, ErrTransportClosed
	case res, ok := <-t.responses:
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
	return t.exitErr
}

func (t *StdioTransport) shutdown() {
	t.closeOnce.Do(func() {
		close(t.closed)
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
