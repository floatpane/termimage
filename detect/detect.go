// Package detect identifies terminal graphics capabilities.
package detect

import (
	"os"
	"strings"
)

// Protocol is a terminal graphics protocol.
type Protocol int

const (
	HalfBlock Protocol = iota // always works: Unicode ▀▄
	Sixel                     // DEC Sixel — xterm, mlterm, foot, WezTerm
	Kitty                     // Kitty graphics — kitty, WezTerm ≥0.29, Ghostty
)

// Best returns the best protocol the current terminal supports.
// It checks environment variables; it does NOT send ANSI queries (no TTY req).
func Best() Protocol {
	// KITTY_WINDOW_ID is set by kitty itself.
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return Kitty
	}

	// Ghostty sets TERM=ghostty or TERM=xterm-ghostty, and TERM_PROGRAM=ghostty.
	term := os.Getenv("TERM")
	termProg := strings.ToLower(os.Getenv("TERM_PROGRAM"))

	if term == "ghostty" || strings.HasPrefix(term, "xterm-ghostty") || termProg == "ghostty" {
		return Kitty
	}

	// WezTerm supports Kitty graphics protocol.
	if termProg == "wezterm" {
		return Kitty
	}

	// foot, mlterm, xterm (with sixel patch), Contour support Sixel.
	switch termProg {
	case "foot", "mlterm", "contour":
		return Sixel
	}
	if strings.Contains(term, "sixel") {
		return Sixel
	}

	return HalfBlock
}

func (p Protocol) String() string {
	switch p {
	case Kitty:
		return "kitty"
	case Sixel:
		return "sixel"
	case HalfBlock:
		return "halfblock"
	default:
		return "halfblock"
	}
}
