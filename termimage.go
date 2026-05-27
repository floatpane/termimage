// Package termimage renders images in a terminal using Kitty graphics, Sixel,
// or Unicode half-block characters as fallback. Image decoding runs inside a
// sandboxed subprocess (Landlock + seccomp on Linux) so untrusted bytes never
// touch the parent process.
//
// The consuming binary must call MaybeRunWorker() at the very top of main():
//
//	func main() {
//	    termimage.MaybeRunWorker()
//	    // ... rest of your app
//	}
package termimage

import (
	"context"
	"image"
	"io"
	"os"

	"github.com/floatpane/termimage/decode"
	"github.com/floatpane/termimage/detect"
	"github.com/floatpane/termimage/internal/resize"
	"github.com/floatpane/termimage/internal/source"
	"github.com/floatpane/termimage/render"
	"github.com/floatpane/termimage/sandbox"
)

// MaybeRunWorker must be called at the top of main(). If this process was
// spawned as a sandbox worker it applies OS restrictions, decodes the image,
// writes pixels to stdout, and exits. Otherwise it returns immediately.
func MaybeRunWorker() { sandbox.MaybeRunWorker() }

// Protocol selects a terminal rendering protocol.
type Protocol = detect.Protocol

const (
	Auto      Protocol = -1
	HalfBlock          = detect.HalfBlock
	Sixel              = detect.Sixel
	Kitty              = detect.Kitty
)

// Options configures image display.
type Options struct {
	// MaxWidth / MaxHeight in pixels. 0 = detect from terminal.
	MaxWidth, MaxHeight int

	// Protocol selects the rendering protocol. Auto detects from $TERM etc.
	Protocol Protocol

	// Sandboxed runs the decoder in a subprocess with Landlock + seccomp.
	// Requires the consuming binary to call MaybeRunWorker() in main().
	Sandboxed bool
}

// Display decodes the image at src and writes terminal graphics to w.
// src may be a local file path, a data: URI, or an http(s):// URL.
func Display(w io.Writer, src string, opts Options) error {
	return DisplayContext(context.Background(), w, src, opts)
}

// DisplayContext is Display with caller-supplied context for cancellation of
// remote fetches and sandboxed decoding.
func DisplayContext(ctx context.Context, w io.Writer, src string, opts Options) error {
	maxW, maxH := effectiveDimensions(opts)

	proto := opts.Protocol
	if proto == Auto {
		proto = detect.Best()
	}

	resolved, err := source.Resolve(ctx, src)
	if err != nil {
		return err
	}

	var img *image.NRGBA
	switch resolved.Kind {
	case source.KindFile:
		if opts.Sandboxed {
			img, err = sandbox.DecodeContext(ctx, resolved.Path)
		} else {
			img, err = decode.File(resolved.Path)
		}
	case source.KindBytes:
		if opts.Sandboxed {
			img, err = sandbox.DecodeBytesContext(ctx, resolved.Bytes)
		} else {
			img, err = decode.Bytes(resolved.Bytes)
		}
	}
	if err != nil {
		return err
	}

	scaled := resize.Fit(img, maxW, maxH)
	return renderWith(w, scaled, proto)
}

func renderWith(w io.Writer, img *image.NRGBA, proto Protocol) error {
	switch proto {
	case Kitty:
		return render.Kitty(w, img)
	case Sixel:
		return render.Sixel(w, img)
	default:
		return render.HalfBlock(w, img)
	}
}

func effectiveDimensions(opts Options) (int, int) {
	w, h := opts.MaxWidth, opts.MaxHeight
	if w > 0 && h > 0 {
		return w, h
	}
	tw, th := detectTermPixels()
	if w <= 0 {
		w = tw
	}
	if h <= 0 {
		h = th
	}
	return w, h
}

func detectTermPixels() (int, int) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return 1920, 1080
	}
	defer func() { _ = f.Close() }()
	return termPixels(f)
}
