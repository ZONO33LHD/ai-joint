package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

	// Terminal dimensions recorded at launch time.
	// vt10x must be initialised with these to replay output correctly.
	Cols int
	Rows int

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

// SizePath returns the path of the terminal-size file ("cols rows") for a session.
func SizePath(id string) string {
	dir := filepath.Join(os.Getenv("HOME"), ".local", "share", "ai-joint", "logs")
	os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, id+".size")
}

// WriteSize persists the terminal dimensions so the dashboard can replay
// PTY output with the same size that Claude Code originally drew for.
func WriteSize(id string, cols, rows int) error {
	return os.WriteFile(SizePath(id), []byte(fmt.Sprintf("%d %d", cols, rows)), 0o644)
}

// ReadSize reads the persisted terminal dimensions for a session.
// Returns 220, 50 as safe defaults if the file is missing or unreadable.
func ReadSize(id string) (cols, rows int) {
	data, err := os.ReadFile(SizePath(id))
	if err != nil {
		return 220, 50
	}
	parts := strings.Fields(string(data))
	if len(parts) != 2 {
		return 220, 50
	}
	c, err1 := strconv.Atoi(parts[0])
	r, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 220, 50
	}
	return c, r
}
