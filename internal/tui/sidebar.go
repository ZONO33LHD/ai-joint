package tui

import (
	"cmp"
	"fmt"
	"log/slog"
	"slices"

	"github.com/rivo/tview"
	"github.com/shunsuke/ai-joint/internal/session"
	"github.com/shunsuke/ai-joint/internal/store"
)

type Sidebar struct {
	*tview.Flex
	sessionList  *tview.List
	activityView *tview.TextView
	sessions     []*session.Session
	st           *store.Store
	onSelect     func(s *session.Session)
}

func NewSidebar(st *store.Store, onSelect func(s *session.Session)) *Sidebar {
	sb := &Sidebar{
		Flex:         tview.NewFlex(),
		sessionList:  tview.NewList(),
		activityView: tview.NewTextView(),
		st:           st,
		onSelect:     onSelect,
	}

	sb.sessionList.SetBorder(true)
	sb.sessionList.SetTitle(" Sessions ")
	sb.sessionList.ShowSecondaryText(true)
	sb.sessionList.SetChangedFunc(func(idx int, _, _ string, _ rune) {
		if sb.onSelect != nil && idx >= 0 && idx < len(sb.sessions) {
			sb.onSelect(sb.sessions[idx])
		}
	})

	sb.activityView.SetBorder(true)
	sb.activityView.SetTitle(" Activity ")
	sb.activityView.SetDynamicColors(true)
	sb.activityView.SetScrollable(true)

	sb.Flex.SetDirection(tview.FlexRow).
		AddItem(sb.sessionList, 0, 2, true).
		AddItem(sb.activityView, 0, 1, false)

	return sb
}

func (sb *Sidebar) Refresh(sessions []*session.Session) {
	slices.SortFunc(sessions, func(a, b *session.Session) int {
		return cmp.Compare(b.CreatedAt.UnixNano(), a.CreatedAt.UnixNano())
	})

	selectedIdx := sb.sessionList.GetCurrentItem()
	sb.sessions = sessions
	sb.sessionList.Clear()

	for _, s := range sessions {
		indicator := stateIndicator(s.State)
		main := fmt.Sprintf("%s %s", indicator, s.Name)
		secondary := fmt.Sprintf("   %s · %s", s.State, s.UpdatedAt.Format("15:04"))
		sb.sessionList.AddItem(main, secondary, 0, nil)
	}

	if selectedIdx >= 0 && selectedIdx < len(sessions) {
		sb.sessionList.SetCurrentItem(selectedIdx)
	}

	activities, err := sb.st.ListActivities(20)
	if err != nil {
		slog.Warn("list activities", "err", err)
		return
	}
	sb.activityView.Clear()
	for _, a := range activities {
		name := a.SessionName
		if name == "" {
			name = a.SessionID
			if len(name) > 8 {
				name = name[:8]
			}
		}
		fmt.Fprintf(sb.activityView, "[green]%s[-] [yellow]%s[-] %s %s\n",
			name, a.Kind, a.Value, a.OccurredAt.Format("15:04"))
	}
}

func stateIndicator(state session.State) string {
	switch state {
	case session.StateBusy:
		return "●"
	case session.StateIdle:
		return "○"
	case session.StateDone:
		return "✓"
	default:
		return " "
	}
}

