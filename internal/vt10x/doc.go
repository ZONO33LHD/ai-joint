// Package vt10x is a fork of github.com/hinshun/vt10x with wide character
// (CJK / emoji) support. The original library treats every rune as occupying
// a single cell; this fork uses go-runewidth to advance the cursor by the
// correct display width and fills placeholder cells for the trailing half of
// double-width characters.
package vt10x
