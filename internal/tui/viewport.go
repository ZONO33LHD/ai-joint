package tui

import (
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/shunsuke/ai-joint/internal/session"
)

// Viewport wraps TermView and adds session binding + socket-based input forwarding.
type Viewport struct {
	*TermView
	current        *session.Session
	conn           net.Conn
	connMu         sync.Mutex
	lastResizeCols int
	lastResizeRows int
}

func NewViewport() *Viewport {
	tv := NewTermView()
	tv.SetBorder(true)
	tv.SetTitle(" (no session) ")
	v := &Viewport{TermView: tv}
	tv.onDisplayResize = func(cols, rows int) {
		v.sendResize(cols, rows)
	}
	return v
}

// sendResize sends a PTY resize command over the session's input socket.
// It is a no-op if the size hasn't changed or there is no current session.
func (v *Viewport) sendResize(cols, rows int) {
	if v.current == nil {
		return
	}
	if cols == v.lastResizeCols && rows == v.lastResizeRows {
		return
	}
	v.lastResizeCols = cols
	v.lastResizeRows = rows

	socketPath := session.SocketPath(v.current.ID)
	go func() {
		conn, err := net.Dial("unix", socketPath)
		if err != nil {
			return
		}
		defer conn.Close()
		// Protocol: ESC NUL cols_hi cols_lo rows_hi rows_lo
		packet := []byte{
			0x1B, 0x00,
			byte(cols >> 8), byte(cols),
			byte(rows >> 8), byte(rows),
		}
		conn.Write(packet)
	}()
}

func (v *Viewport) SetSession(s *session.Session) {
	v.current = s
	if s == nil {
		v.SetTitle(" (no session) ")
		v.TermView.SetContent(nil, 0, 0)
		v.disconnect()
		return
	}
	v.SetTitle(fmt.Sprintf(" %s ", s.Name))
	v.TermView.SetContent(s.Output, s.Cols, s.Rows)
}

func (v *Viewport) Refresh(s *session.Session) {
	if v.current == nil || s == nil || v.current.ID != s.ID {
		return
	}
	v.current = s
	v.TermView.SetContent(s.Output, s.Cols, s.Rows)
}

// Connect opens a Unix socket to the session's PTY input server.
func (v *Viewport) Connect() {
	if v.current == nil || v.current.State == session.StateDone {
		return
	}
	v.connMu.Lock()
	defer v.connMu.Unlock()

	if v.conn != nil {
		v.conn.Close()
		v.conn = nil
	}
	conn, err := net.Dial("unix", session.SocketPath(v.current.ID))
	if err != nil {
		slog.Debug("connect to session socket", "err", err)
		return
	}
	v.conn = conn
}

func (v *Viewport) disconnect() {
	v.connMu.Lock()
	defer v.connMu.Unlock()
	if v.conn != nil {
		v.conn.Close()
		v.conn = nil
	}
}

// SendInput converts a tcell key event to bytes and writes to the PTY socket.
func (v *Viewport) SendInput(ev *tcell.EventKey) {
	data := keyEventToBytes(ev)
	if len(data) == 0 {
		return
	}
	v.connMu.Lock()
	defer v.connMu.Unlock()
	if v.conn != nil {
		v.conn.Write(data)
	}
}

// keyEventToBytes converts a tcell key event into the byte sequence
// that a terminal would send for that key.
func keyEventToBytes(ev *tcell.EventKey) []byte {
	if ev.Key() == tcell.KeyRune {
		return []byte(string(ev.Rune()))
	}
	switch ev.Key() {
	case tcell.KeyEnter:
		return []byte{'\r'}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return []byte{'\x7f'}
	case tcell.KeyTab:
		return []byte{'\t'}
	case tcell.KeyEscape:
		return []byte{'\x1b'}
	case tcell.KeyUp:
		return []byte{'\x1b', '[', 'A'}
	case tcell.KeyDown:
		return []byte{'\x1b', '[', 'B'}
	case tcell.KeyRight:
		return []byte{'\x1b', '[', 'C'}
	case tcell.KeyLeft:
		return []byte{'\x1b', '[', 'D'}
	case tcell.KeyHome:
		return []byte{'\x1b', '[', 'H'}
	case tcell.KeyEnd:
		return []byte{'\x1b', '[', 'F'}
	case tcell.KeyDelete:
		return []byte{'\x1b', '[', '3', '~'}
	case tcell.KeyPgUp:
		return []byte{'\x1b', '[', '5', '~'}
	case tcell.KeyPgDn:
		return []byte{'\x1b', '[', '6', '~'}
	case tcell.KeyCtrlA:
		return []byte{'\x01'}
	case tcell.KeyCtrlB:
		return []byte{'\x02'}
	case tcell.KeyCtrlC:
		return []byte{'\x03'}
	case tcell.KeyCtrlD:
		return []byte{'\x04'}
	case tcell.KeyCtrlE:
		return []byte{'\x05'}
	case tcell.KeyCtrlF:
		return []byte{'\x06'}
	case tcell.KeyCtrlK:
		return []byte{'\x0b'}
	case tcell.KeyCtrlL:
		return []byte{'\x0c'}
	case tcell.KeyCtrlN:
		return []byte{'\x0e'}
	case tcell.KeyCtrlP:
		return []byte{'\x10'}
	case tcell.KeyCtrlU:
		return []byte{'\x15'}
	case tcell.KeyCtrlW:
		return []byte{'\x17'}
	case tcell.KeyCtrlZ:
		return []byte{'\x1a'}
	}
	return nil
}
