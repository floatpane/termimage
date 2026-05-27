//go:build linux

package sandbox

import (
	"github.com/landlock-lsm/go-landlock/landlock"
)

// apply locks down the worker process before any untrusted bytes are read.
// Landlock (kernel ≥5.13) restricts filesystem access to the target file only,
// or denies all fs access when path is empty (stdin mode).
// Seccomp syscall filtering is a TODO — see sandbox_seccomp_linux.go once the
// allowlist is tuned for the Go runtime + stb_image.
func apply(path string) error {
	cfg := landlock.V3.BestEffort()
	if path == "" {
		// Deny all filesystem access — bytes arrive via stdin.
		return cfg.RestrictPaths()
	}
	return cfg.RestrictPaths(
		landlock.ROFiles(path),
	)
}
