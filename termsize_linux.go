//go:build linux

package termimage

import (
	"os"

	"golang.org/x/sys/unix"
)

func termPixels(f *os.File) (int, int) {
	ws, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 1920, 1080
	}
	// Prefer pixel dimensions reported by the terminal.
	if ws.Xpixel > 0 && ws.Ypixel > 0 {
		return int(ws.Xpixel), int(ws.Ypixel)
	}
	// Fallback: estimate from cell count (8×16 px per cell is a safe default).
	return int(ws.Col) * 8, int(ws.Row) * 16
}

// termChars returns the terminal dimensions as (cols, rows).
func termChars(f *os.File) (int, int) {
	ws, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 || ws.Row == 0 {
		return 220, 50
	}
	return int(ws.Col), int(ws.Row)
}
