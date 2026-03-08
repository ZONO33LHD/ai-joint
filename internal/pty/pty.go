package pty

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"sync"

	"github.com/creack/pty"
)

type PTY struct {
	mu      sync.Mutex
	ptmx    *os.File
	cmd     *exec.Cmd
	onData  func([]byte)
	onClose func()
}

// Spawn starts a Claude Code process in a PTY.
// The context controls the process lifetime; cancelling it sends SIGKILL.
func Spawn(ctx context.Context, ccBin, dir string, env []string, onData func([]byte), onClose func()) (*PTY, error) {
	cmd := exec.CommandContext(ctx, ccBin)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("pty start: %w", err)
	}

	p := &PTY{
		ptmx:    ptmx,
		cmd:     cmd,
		onData:  onData,
		onClose: onClose,
	}

	go p.readLoop()
	return p, nil
}

func (p *PTY) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := p.ptmx.Read(buf)
		if n > 0 && p.onData != nil {
			p.onData(slices.Clone(buf[:n]))
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				// process ended unexpectedly; ignore OS-level EOF variants
			}
			break
		}
	}
	if p.onClose != nil {
		p.onClose()
	}
}

// Write implements io.Writer, forwarding data to the PTY master.
func (p *PTY) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ptmx.Write(data)
}

func (p *PTY) Resize(rows, cols uint16) error {
	return pty.Setsize(p.ptmx, &pty.Winsize{Rows: rows, Cols: cols})
}

func (p *PTY) Kill() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd.Process != nil {
		return p.cmd.Process.Kill()
	}
	return nil
}

func (p *PTY) Wait() error {
	return p.cmd.Wait()
}
