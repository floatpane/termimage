package detect

import (
	"testing"
)

func TestBest(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want Protocol
	}{
		{
			name: "kitty via KITTY_WINDOW_ID",
			env:  map[string]string{"KITTY_WINDOW_ID": "1"},
			want: Kitty,
		},
		{
			name: "kitty via ghostty TERM=ghostty",
			env:  map[string]string{"TERM": "ghostty"},
			want: Kitty,
		},
		{
			name: "kitty via ghostty TERM_PROGRAM=ghostty",
			env:  map[string]string{"TERM_PROGRAM": "ghostty"},
			want: Kitty,
		},
		{
			name: "kitty via ghostty TERM=xterm-ghostty",
			env:  map[string]string{"TERM": "xterm-ghostty"},
			want: Kitty,
		},
		{
			name: "kitty via wezterm",
			env:  map[string]string{"TERM_PROGRAM": "WezTerm"},
			want: Kitty,
		},
		{
			name: "sixel via foot",
			env:  map[string]string{"TERM_PROGRAM": "foot"},
			want: Sixel,
		},
		{
			name: "sixel via mlterm",
			env:  map[string]string{"TERM_PROGRAM": "mlterm"},
			want: Sixel,
		},
		{
			name: "sixel via contour",
			env:  map[string]string{"TERM_PROGRAM": "contour"},
			want: Sixel,
		},
		{
			name: "sixel via TERM substring",
			env:  map[string]string{"TERM": "xterm-sixel"},
			want: Sixel,
		},
		{
			name: "halfblock fallback",
			env:  map[string]string{"TERM": "dumb"},
			want: HalfBlock,
		},
		{
			name: "halfblock when nothing set",
			env:  map[string]string{},
			want: HalfBlock,
		},
		{
			name: "kitty takes priority over sixel TERM",
			env:  map[string]string{"KITTY_WINDOW_ID": "1", "TERM": "xterm-sixel"},
			want: Kitty,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range []string{"KITTY_WINDOW_ID", "TERM", "TERM_PROGRAM"} {
				t.Setenv(k, "")
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			if got := Best(); got != tc.want {
				t.Errorf("Best() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProtocolString(t *testing.T) {
	tests := []struct {
		p    Protocol
		want string
	}{
		{Kitty, "kitty"},
		{Sixel, "sixel"},
		{HalfBlock, "halfblock"},
		{Protocol(99), "halfblock"},
	}
	for _, tc := range tests {
		if got := tc.p.String(); got != tc.want {
			t.Errorf("Protocol(%d).String() = %q, want %q", tc.p, got, tc.want)
		}
	}
}
