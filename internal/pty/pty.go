package pty

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
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
				// process ended unexpectedly
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

// StartInputServer listens on a Unix domain socket at socketPath.
// Data received is forwarded to the PTY; a special 6-byte sequence
// (ESC NUL cols_hi cols_lo rows_hi rows_lo) triggers a PTY resize instead.
// onResize, if non-nil, is called after each successful resize.
func (p *PTY) StartInputServer(socketPath string, onResize func(cols, rows uint16)) error {
	os.Remove(socketPath) // remove stale socket from previous run
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listen %s: %w", socketPath, err)
	}
	go func() {
		defer os.Remove(socketPath)
		defer ln.Close()
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveInput(conn, p, onResize)
		}
	}()
	return nil
}

// serveInput forwards bytes from conn to the PTY, intercepting the 6-byte
// resize command ESC NUL <cols_hi> <cols_lo> <rows_hi> <rows_lo>.
func serveInput(conn net.Conn, p *PTY, onResize func(cols, rows uint16)) {
	defer conn.Close()
	r := bufio.NewReader(conn)

	for {
		b, err := r.ReadByte()
		if err != nil {
			return
		}
		if b != 0x1B {
			p.Write([]byte{b})
			continue
		}
		next, err := r.ReadByte()
		if err != nil {
			p.Write([]byte{0x1B})
			return
		}
		if next == 0x00 {
			// Resize command: read 4 bytes (cols BE + rows BE).
			var dims [4]byte
			if _, err := io.ReadFull(r, dims[:]); err != nil {
				return
			}
			cols := uint16(dims[0])<<8 | uint16(dims[1])
			rows := uint16(dims[2])<<8 | uint16(dims[3])
			p.Resize(rows, cols)
			if onResize != nil {
				onResize(cols, rows)
			}
		} else if next == '[' || next == 'O' {
			// CSI (ESC [) or SS3 (ESC O) sequence: buffer until the final byte
			// (0x40–0x7E) to write the entire sequence atomically.
			buf := []byte{0x1B, next}
			for {
				b2, err := r.ReadByte()
				if err != nil {
					p.Write(buf)
					return
				}
				buf = append(buf, b2)
				if b2 >= 0x40 && b2 <= 0x7E {
					break
				}
			}
			p.Write(buf)
		} else {
			// Single-char escape (e.g. bare ESC, Alt+key).
			p.Write([]byte{0x1B, next})
		}
	}
}
