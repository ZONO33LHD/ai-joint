package vt10x

import (
	"fmt"
	"testing"
)

func TestWideCharCells(t *testing.T) {
	term := New(WithSize(40, 5))
	term.Write([]byte("こんにちは"))
	term.Lock()
	defer term.Unlock()

	// Dump all cells in row 0
	fmt.Println("=== Cell dump for 'こんにちは' ===")
	for i := 0; i < 20; i++ {
		g := term.Cell(i, 0)
		fmt.Printf("Cell[%2d]: Char=%q (U+%04X) IsWideDummy=%v Mode=%d\n",
			i, string(g.Char), g.Char, g.IsWideDummy(), g.Mode)
	}

	// Expected: こ dummy ん dummy に dummy ち dummy は dummy
	expected := []struct {
		char    rune
		isDummy bool
	}{
		{'こ', false}, {0, true},
		{'ん', false}, {0, true},
		{'に', false}, {0, true},
		{'ち', false}, {0, true},
		{'は', false}, {0, true},
	}

	for i, exp := range expected {
		g := term.Cell(i, 0)
		if g.IsWideDummy() != exp.isDummy {
			t.Errorf("Cell[%d]: IsWideDummy=%v, want %v", i, g.IsWideDummy(), exp.isDummy)
		}
		if !exp.isDummy && g.Char != exp.char {
			t.Errorf("Cell[%d]: Char=%q, want %q", i, string(g.Char), string(exp.char))
		}
	}

	// Check cursor position
	cur := term.Cursor()
	if cur.X != 10 {
		t.Errorf("Cursor.X=%d, want 10", cur.X)
	}
}

func TestWideCharWithAbsolutePositioning(t *testing.T) {
	term := New(WithSize(40, 5))
	// Write "AB" then position cursor at column 4 (0-indexed: 3) and write "CD"
	// This simulates a TUI app using absolute positioning
	term.Write([]byte("AB"))                      // A at 0, B at 1, cursor at 2
	term.Write([]byte("\033[1;5H"))                // Move to row 1, col 5 (0-indexed: col 4)
	term.Write([]byte("こんにちは"))               // Wide chars starting at col 4

	term.Lock()
	defer term.Unlock()

	fmt.Println("\n=== Cell dump for absolute positioning test ===")
	for i := 0; i < 20; i++ {
		g := term.Cell(i, 0)
		fmt.Printf("Cell[%2d]: Char=%q (U+%04X) IsWideDummy=%v\n",
			i, string(g.Char), g.Char, g.IsWideDummy())
	}

	// A at 0, B at 1, spaces at 2-3, こ at 4, dummy at 5, ん at 6, ...
	g := term.Cell(0, 0)
	if g.Char != 'A' {
		t.Errorf("Cell[0]: Char=%q, want 'A'", string(g.Char))
	}
	g = term.Cell(4, 0)
	if g.Char != 'こ' {
		t.Errorf("Cell[4]: Char=%q, want 'こ'", string(g.Char))
	}
	g = term.Cell(5, 0)
	if !g.IsWideDummy() {
		t.Errorf("Cell[5]: IsWideDummy=%v, want true", g.IsWideDummy())
	}
	g = term.Cell(6, 0)
	if g.Char != 'ん' {
		t.Errorf("Cell[6]: Char=%q, want 'ん'", string(g.Char))
	}
}

func TestMixedASCIIAndWide(t *testing.T) {
	term := New(WithSize(40, 5))
	term.Write([]byte("Aこ B"))

	term.Lock()
	defer term.Unlock()

	fmt.Println("\n=== Cell dump for 'Aこ B' ===")
	for i := 0; i < 10; i++ {
		g := term.Cell(i, 0)
		fmt.Printf("Cell[%2d]: Char=%q (U+%04X) IsWideDummy=%v\n",
			i, string(g.Char), g.Char, g.IsWideDummy())
	}

	// A at 0, こ at 1, dummy at 2, ' ' at 3, B at 4
	checks := []struct {
		pos     int
		char    rune
		isDummy bool
	}{
		{0, 'A', false},
		{1, 'こ', false},
		{2, 0, true},
		{3, ' ', false},
		{4, 'B', false},
	}
	for _, c := range checks {
		g := term.Cell(c.pos, 0)
		if g.IsWideDummy() != c.isDummy {
			t.Errorf("Cell[%d]: IsWideDummy=%v, want %v", c.pos, g.IsWideDummy(), c.isDummy)
		}
		if !c.isDummy && g.Char != c.char {
			t.Errorf("Cell[%d]: Char=%q, want %q", c.pos, string(g.Char), string(c.char))
		}
	}
}
