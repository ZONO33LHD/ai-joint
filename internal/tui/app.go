package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/shunsuke/ai-joint/internal/session"
	"github.com/shunsuke/ai-joint/internal/store"
)

type App struct {
	app      *tview.Application
	sidebar  *Sidebar
	viewport *Viewport
	manager  *session.Manager
	store    *store.Store
}

func NewApp(mgr *session.Manager, st *store.Store) *App {
	a := &App{
		app:     tview.NewApplication(),
		manager: mgr,
		store:   st,
	}

	a.viewport = NewViewport()
	a.sidebar = NewSidebar(st, func(s *session.Session) {
		a.viewport.SetSession(s)
	})

	root := tview.NewFlex().
		AddItem(a.sidebar, 30, 0, true).
		AddItem(a.viewport, 0, 1, false)

	frame := tview.NewFrame(root).
		AddText("ai-joint", true, tview.AlignCenter, 0).
		AddText("Tab=パネル切替  q=終了", false, tview.AlignCenter, 0)

	// Tab switches focus between panels
	a.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'q':
			a.app.Stop()
			return nil
		case '\t':
			if a.app.GetFocus() == a.sidebar.sessionList {
				a.app.SetFocus(a.viewport)
			} else {
				a.app.SetFocus(a.sidebar.sessionList)
			}
			return nil
		}
		return event
	})

	a.app.SetRoot(frame, true)

	// Register change callback
	mgr.SetOnChange(func() {
		a.app.QueueUpdateDraw(func() {
			a.refresh()
		})
	})

	return a
}

func (a *App) refresh() {
	sessions := a.manager.List()
	a.sidebar.Refresh(sessions)
	// Refresh viewport if a session is selected
	if cur := a.viewport.current; cur != nil {
		updated := a.manager.Get(cur.ID)
		if updated != nil {
			a.viewport.Refresh(updated)
		}
	} else if len(sessions) > 0 {
		a.viewport.SetSession(sessions[0])
	}
}

func (a *App) Run() error {
	a.refresh()
	return a.app.Run()
}
