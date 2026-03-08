package tui

import (
	"log/slog"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/shunsuke/ai-joint/internal/session"
	"github.com/shunsuke/ai-joint/internal/store"
)

const (
	footerNav    = " [green]Tab[-]:Switch  [green]↑↓[-]:Navigate  [green]i[-]:Input mode  [green]q[-]:Quit"
	footerBrowse = " [yellow]Tab[-]:Switch  [yellow]↑↓[-]:Scroll  [yellow]i[-]:Input mode  [yellow]q[-]:Quit"
	footerInput  = " [red]INPUT MODE[-]  [red]Esc[-]:Exit  (all keystrokes → session)"
)

type App struct {
	app       *tview.Application
	sidebar   *Sidebar
	viewport  *Viewport
	footer    *tview.TextView
	manager   *session.Manager
	store     *store.Store
	inputMode bool
}

func NewApp(mgr *session.Manager, st *store.Store) *App {
	a := &App{
		app:     tview.NewApplication(),
		manager: mgr,
		store:   st,
	}

	a.viewport = NewViewport()

	a.footer = tview.NewTextView()
	a.footer.SetDynamicColors(true)
	a.footer.SetText(footerNav)

	a.sidebar = NewSidebar(st, func(s *session.Session) {
		a.viewport.SetSession(s)
	})

	mainFlex := tview.NewFlex().
		AddItem(a.sidebar, 30, 0, true).
		AddItem(a.viewport, 0, 1, false)

	title := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetText("── ai-joint ──")

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(title, 1, 0, false).
		AddItem(mainFlex, 0, 1, true).
		AddItem(a.footer, 1, 0, false)

	a.app.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		// Input mode: forward all keys to the PTY; Esc exits.
		if a.inputMode {
			if ev.Key() == tcell.KeyEscape {
				a.inputMode = false
				a.footer.SetText(footerBrowse)
				return nil
			}
			a.viewport.SendInput(ev)
			return nil
		}

		// Navigation mode.
		switch ev.Key() {
		case tcell.KeyTab:
			if a.app.GetFocus() == a.sidebar.sessionList {
				a.app.SetFocus(a.viewport)
				a.footer.SetText(footerBrowse)
			} else {
				a.viewport.disconnect()
				a.app.SetFocus(a.sidebar.sessionList)
				a.footer.SetText(footerNav)
			}
			return nil
		}

		switch ev.Rune() {
		case 'q':
			a.app.Stop()
			return nil
		case 'i':
			// Enter input mode only when the viewport is focused.
			if a.app.GetFocus() == a.viewport {
				a.viewport.Connect()
				a.inputMode = true
				a.footer.SetText(footerInput)
				return nil
			}
		}
		return ev
	})

	a.app.SetRoot(root, true)

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

	if cur := a.viewport.current; cur != nil {
		if updated := a.manager.Get(cur.ID); updated != nil {
			a.viewport.Refresh(updated)
		}
	} else if len(sessions) > 0 {
		a.viewport.SetSession(sessions[0])
	}
}

func (a *App) Run() error {
	a.refresh()

	// Poll the store every 2 seconds to pick up changes made by external
	// processes (e.g. aj hook writing new activities).
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := a.manager.Reload(); err != nil {
				slog.Warn("reload sessions", "err", err)
				continue
			}
			a.app.QueueUpdateDraw(func() {
				a.refresh()
			})
		}
	}()

	return a.app.Run()
}
