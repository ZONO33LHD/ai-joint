package tui

import (
	"fmt"

	"github.com/rivo/tview"
	"github.com/shunsuke/ai-joint/internal/session"
)

type Viewport struct {
	*tview.TextView
	current *session.Session
}

func NewViewport() *Viewport {
	tv := tview.NewTextView()
	tv.SetBorder(true)
	tv.SetTitle(" (no session) ")
	tv.SetDynamicColors(true)
	tv.SetScrollable(true)
	return &Viewport{TextView: tv}
}

func (v *Viewport) SetSession(s *session.Session) {
	v.current = s
	if s == nil {
		v.SetTitle(" (no session) ")
		v.Clear()
		return
	}
	v.SetTitle(fmt.Sprintf(" %s ", s.Name))
	v.refresh()
}

func (v *Viewport) refresh() {
	if v.current == nil {
		return
	}
	v.Clear()
	// Write raw output (may contain ANSI codes — tview handles basic ones)
	fmt.Fprintf(v, "%s", string(v.current.Output))
	v.ScrollToEnd()
}

func (v *Viewport) Refresh(s *session.Session) {
	if v.current == nil || s == nil {
		return
	}
	if v.current.ID != s.ID {
		return
	}
	v.current = s
	v.refresh()
}
