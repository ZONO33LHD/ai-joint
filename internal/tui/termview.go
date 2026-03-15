package tui

import (
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/tview"
	"github.com/shunsuke/ai-joint/internal/vt10x"
)

// attrXxx mirrors the unexported constants in vt10x/state.go.
const (
	attrReverse   = 1 << 0
	attrUnderline = 1 << 1
	attrBold      = 1 << 2
)

// TermView is a tview.Primitive that renders PTY output through a vt10x
// terminal emulator, correctly handling cursor movement, screen clears, and
// full-screen TUI apps like Claude Code.
//
// Follow mode (default on): view automatically tracks the cursor.
// When the user scrolls manually, follow mode is suspended until
// FollowCursor() is called again (or Tab switches back to sidebar).
type TermView struct {
	*tview.Box
	mu              sync.Mutex
	vt              vt10x.Terminal
	vtCols          int
	vtRows          int
	rowOffset       int
	colOffset       int
	follow          bool // true = snap to cursor on each SetContent
	onDisplayResize func(cols, rows int)
	lastDrawW       int
	lastDrawH       int
}

func NewTermView() *TermView {
	tv := &TermView{
		vtCols: 220,
		vtRows: 50,
		follow: true,
		Box:    tview.NewBox(),
	}
	tv.vt = vt10x.New(vt10x.WithSize(tv.vtCols, tv.vtRows))
	return tv
}

// SetContent resets the terminal emulator and replays all raw PTY bytes.
// vtCols/vtRows should match the terminal size used when the PTY was launched.
func (tv *TermView) SetContent(data []byte, vtCols, vtRows int) {
	tv.mu.Lock()
	defer tv.mu.Unlock()

	if vtCols > 0 && vtRows > 0 {
		tv.vtCols, tv.vtRows = vtCols, vtRows
	}
	tv.vt = vt10x.New(vt10x.WithSize(tv.vtCols, tv.vtRows))
	tv.vt.Write(data)

	if tv.follow {
		tv.snapToCursor()
	}
}

// FollowCursor re-enables follow mode and immediately snaps to the cursor.
func (tv *TermView) FollowCursor() {
	tv.mu.Lock()
	defer tv.mu.Unlock()
	tv.follow = true
	tv.snapToCursor()
}

// Scroll moves the viewport by (dRow, dCol) cells and suspends follow mode.
func (tv *TermView) Scroll(dRow, dCol int) {
	tv.mu.Lock()
	defer tv.mu.Unlock()
	_, _, w, h := tv.GetInnerRect()
	tv.rowOffset = clampInt(tv.rowOffset+dRow, 0, max(0, tv.vtRows-h))
	tv.colOffset = clampInt(tv.colOffset+dCol, 0, max(0, tv.vtCols-w))
	tv.follow = false // user is browsing — stop auto-following
}

// snapToCursor adjusts offsets so the cursor is in view. Must hold tv.mu.
func (tv *TermView) snapToCursor() {
	_, _, w, h := tv.GetInnerRect()
	if w <= 0 || h <= 0 {
		return
	}
	tv.vt.Lock()
	cur := tv.vt.Cursor()
	tv.vt.Unlock()

	if cur.Y < tv.rowOffset {
		tv.rowOffset = cur.Y
	} else if cur.Y >= tv.rowOffset+h {
		tv.rowOffset = cur.Y - h + 1
	}
	if tv.rowOffset < 0 {
		tv.rowOffset = 0
	}

	if cur.X < tv.colOffset {
		tv.colOffset = cur.X
	} else if cur.X >= tv.colOffset+w {
		tv.colOffset = cur.X - w + 1
	}
	if tv.colOffset < 0 {
		tv.colOffset = 0
	}
}

// Draw implements tview.Primitive.
func (tv *TermView) Draw(screen tcell.Screen) {
	tv.Box.DrawForSubclass(screen, tv)
	x, y, w, h := tv.GetInnerRect()

	// Notify when the display area changes so the PTY can be resized to match.
	if (w != tv.lastDrawW || h != tv.lastDrawH) && w > 0 && h > 0 {
		tv.lastDrawW, tv.lastDrawH = w, h
		if tv.onDisplayResize != nil {
			tv.onDisplayResize(w, h)
		}
	}

	tv.mu.Lock()
	defer tv.mu.Unlock()

	// On the first draw (or any draw where follow is on), snap to cursor now
	// that widget dimensions are known. SetContent's snapToCursor is a no-op
	// before the first draw because GetInnerRect returns zero.
	if tv.follow {
		tv.snapToCursor()
	}

	tv.vt.Lock()
	defer tv.vt.Unlock()

	for row := 0; row < h; row++ {
		vtRow := tv.rowOffset + row
		if vtRow >= tv.vtRows {
			break
		}
		screenCol := 0
		for vtCol := tv.colOffset; vtCol < tv.vtCols && screenCol < w; vtCol++ {
			g := tv.vt.Cell(vtCol, vtRow)

			// Skip wide-character placeholder cells.
			if g.IsWideDummy() {
				continue
			}

			ch := g.Char
			if ch == 0 {
				ch = ' '
			}
			screen.SetContent(x+screenCol, y+row, ch, nil, glyphStyle(g))
			rw := runewidth.RuneWidth(ch)
			if rw < 1 {
				rw = 1
			}
			screenCol += rw
		}
	}

	// Draw cursor if visible and within the current view.
	if tv.vt.CursorVisible() {
		cur := tv.vt.Cursor()
		sc := cur.X - tv.colOffset
		sr := cur.Y - tv.rowOffset
		if sc >= 0 && sc < w && sr >= 0 && sr < h {
			_, _, style, _ := screen.GetContent(x+sc, y+sr)
			screen.SetContent(x+sc, y+sr, ' ', nil, style.Reverse(true))
		}
	}
}

func glyphStyle(g vt10x.Glyph) tcell.Style {
	style := tcell.StyleDefault.
		Foreground(vtColor(g.FG)).
		Background(vtColor(g.BG))
	if g.Mode&attrBold != 0 {
		style = style.Bold(true)
	}
	if g.Mode&attrUnderline != 0 {
		style = style.Underline(true)
	}
	if g.Mode&attrReverse != 0 {
		style = style.Reverse(true)
	}
	return style
}

func vtColor(c vt10x.Color) tcell.Color {
	if c == vt10x.DefaultFG || c == vt10x.DefaultBG {
		return tcell.ColorDefault
	}
	if c < 256 {
		return tcell.PaletteColor(int(c))
	}
	raw := uint32(c) & 0x00FFFFFF
	r := (raw >> 16) & 0xFF
	g := (raw >> 8) & 0xFF
	b := raw & 0xFF
	return tcell.NewRGBColor(int32(r), int32(g), int32(b))
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
