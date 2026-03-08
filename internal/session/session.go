package session

import (
	"os"
	"path/filepath"
	"time"
)

type State string

const (
	StateBusy State = "busy"
	StateIdle State = "idle"
	StateDone State = "done"
)

type Session struct {
	ID        string
	Name      string
	Dir       string
	State     State
	CreatedAt time.Time
	UpdatedAt time.Time

	// PTY output buffer (last 64 KiB)
	Output []byte
}

// SocketPath returns the Unix domain socket path used to forward
// keyboard input from the dashboard to this session's PTY.
func SocketPath(id string) string {
	dir := filepath.Join(os.Getenv("HOME"), ".local", "share", "ai-joint", "socks")
	os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, id+".sock")
}

// OutputPath returns the path of the raw PTY output log for a session.
// aj launch appends to this file; aj dashboard reads from it.
func OutputPath(id string) string {
	dir := filepath.Join(os.Getenv("HOME"), ".local", "share", "ai-joint", "logs")
	os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, id+".log")
}
