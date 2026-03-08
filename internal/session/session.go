package session

import "time"

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

	// PTY output buffer (last N bytes)
	Output []byte
}

func (s *Session) StateIndicator() string {
	switch s.State {
	case StateBusy:
		return "[yellow]●[-]"
	case StateIdle:
		return "[green]○[-]"
	case StateDone:
		return "[gray]✓[-]"
	default:
		return " "
	}
}
