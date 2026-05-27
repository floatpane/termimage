<div align="center">

# termimage

**Render images in the terminal — Kitty, Sixel, or Unicode half-blocks — with a sandboxed decoder.**

[![Go Version](https://img.shields.io/github/go-mod/go-version/floatpane/termimage)](https://golang.org)
[![Go Reference](https://pkg.go.dev/badge/github.com/floatpane/termimage.svg)](https://pkg.go.dev/github.com/floatpane/termimage)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/floatpane/termimage)](https://github.com/floatpane/termimage/releases)
[![CI](https://github.com/floatpane/termimage/actions/workflows/ci.yml/badge.svg)](https://github.com/floatpane/termimage/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

</div>

`termimage` is a small Go library that displays images in a terminal. It auto-detects the best supported protocol (Kitty graphics, DEC Sixel, or Unicode half-blocks as fallback) and decodes images in a sandboxed subprocess (Landlock + seccomp on Linux) so untrusted bytes never touch the parent process.

## Features

- **Auto-detected protocols** — Kitty, Sixel, half-block fallback. Works on any modern terminal.
- **Multiple sources** — local files, `data:` URIs (base64), and `http(s)://` URLs.
- **Sandboxed decoder** — image bytes parsed in an isolated subprocess with Landlock + seccomp on Linux.
- **No CGO required for the consumer** — pure-Go API surface; the C decoder is contained in the worker subprocess.
- **Terminal pixel detection** — sizes output to the actual cell pixel dimensions when available.

## Install

```bash
go get github.com/floatpane/termimage
```

Requires Go 1.26+.

## Usage

```go
package main

import (
    "os"

    "github.com/floatpane/termimage"
)

func main() {
    // Required: must be the first call in main() so sandbox workers can
    // re-exec into the decoder without running the rest of your program.
    termimage.MaybeRunWorker()

    err := termimage.Display(os.Stdout, "cat.png", termimage.Options{
        Protocol:  termimage.Auto,
        Sandboxed: true,
    })
    if err != nil {
        panic(err)
    }
}
```

### Sources

The `src` argument accepts any of:

```go
termimage.Display(os.Stdout, "/path/to/cat.png", opts)
termimage.Display(os.Stdout, "https://example.com/cat.png", opts)
termimage.Display(os.Stdout, "data:image/png;base64,iVBORw0KGgo...", opts)
```

Remote URLs and data URIs are fetched/decoded in the parent process, then handed
to the sandboxed worker over stdin — the worker still runs with Landlock denying
all filesystem access. Remote payloads are capped at 64 MiB.

Use `DisplayContext` to pass a `context.Context` for cancellation of HTTP fetches
and decoding.

### Options

| Field | Description |
|-------|-------------|
| `MaxWidth`, `MaxHeight` | Pixel bounds. `0` = detect from terminal. |
| `Protocol` | `Auto`, `Kitty`, `Sixel`, or `HalfBlock`. |
| `Sandboxed` | Decode in a Landlock + seccomp subprocess. |

### Protocol detection

`detect.Best()` inspects `$TERM`, `$TERM_PROGRAM`, and `$KITTY_WINDOW_ID`. No ANSI queries are sent, so it works without a TTY.

| Terminal | Protocol |
|----------|----------|
| kitty, Ghostty, WezTerm | Kitty graphics |
| foot, mlterm, Contour, xterm-sixel | Sixel |
| everything else | half-block |

## Sandbox

On Linux, the worker subprocess applies:

- **Landlock** — restricts filesystem access to the single image path.
- **seccomp-bpf** — blocks syscalls outside a read/decode allowlist.

On other platforms, the worker runs without OS-level restrictions but is still process-isolated.

## Documentation

Full API reference: [pkg.go.dev/github.com/floatpane/termimage](https://pkg.go.dev/github.com/floatpane/termimage)

## Contributing

PRs welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Security

Report vulnerabilities privately via [SECURITY.md](SECURITY.md).

## License

MIT. See [LICENSE](LICENSE).
