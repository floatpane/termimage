// Package sandbox runs image decoding in an isolated subprocess.
//
// The caller's binary must call MaybeRunWorker() at the very top of main()
// so the child process can identify itself and apply OS-level restrictions
// before doing any work:
//
//	func main() {
//	    termimage.MaybeRunWorker()
//	    // ... normal startup
//	}
package sandbox

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	workerEnv     = "TERMIMAGE_WORKER"
	workerModeEnv = "TERMIMAGE_WORKER_MODE"
	workerPathEnv = "TERMIMAGE_WORKER_PATH"

	modePath  = "path"
	modeStdin = "stdin"

	// maxStdinBytes caps the payload the worker will read from stdin to avoid
	// unbounded allocation if the parent misbehaves.
	maxStdinBytes = 256 * 1024 * 1024
)

// IsWorker reports whether this process was spawned as a sandbox worker.
func IsWorker() bool {
	return os.Getenv(workerEnv) == "1"
}

// MaybeRunWorker checks if this process is a worker. If so, it applies
// OS-level sandbox restrictions, decodes the image from stdin, writes the
// raw pixel data to stdout, and exits. Call this at the start of main().
func MaybeRunWorker() {
	if !IsWorker() {
		return
	}

	if err := runWorker(); err != nil {
		fmt.Fprintf(os.Stderr, "termimage worker: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// Decode spawns a sandboxed worker subprocess to decode the image at path.
// Returns an NRGBA image whose pixels were produced inside the sandbox.
func Decode(path string) (*image.NRGBA, error) {
	return DecodeContext(context.Background(), path)
}

// DecodeContext is Decode with caller-supplied context for cancellation.
func DecodeContext(ctx context.Context, path string) (*image.NRGBA, error) {
	return spawn(ctx, modePath, path, nil)
}

// DecodeBytes spawns a sandboxed worker subprocess to decode raw image bytes
// piped over stdin. Used for data: URIs and remote URLs where there is no
// on-disk file to grant access to.
func DecodeBytes(data []byte) (*image.NRGBA, error) {
	return DecodeBytesContext(context.Background(), data)
}

// DecodeBytesContext is DecodeBytes with caller-supplied context.
func DecodeBytesContext(ctx context.Context, data []byte) (*image.NRGBA, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("sandbox: empty input")
	}
	if len(data) > maxStdinBytes {
		return nil, fmt.Errorf("sandbox: input exceeds %d bytes", maxStdinBytes)
	}
	return spawn(ctx, modeStdin, "", data)
}

func spawn(ctx context.Context, mode, path string, stdinData []byte) (*image.NRGBA, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("sandbox: resolve self: %w", err)
	}

	cmd := exec.CommandContext(ctx, self)
	env := append(os.Environ(), workerEnv+"=1", workerModeEnv+"="+mode)
	if mode == modePath {
		env = append(env, workerPathEnv+"="+path)
	}
	cmd.Env = env
	cmd.Stderr = os.Stderr

	if mode == modeStdin {
		cmd.Stdin = bytes.NewReader(stdinData)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("sandbox: spawn worker: %w", err)
	}

	img, err := readPixels(stdout)
	werr := cmd.Wait()
	if err != nil {
		return nil, err
	}
	if werr != nil {
		return nil, fmt.Errorf("sandbox: worker exited: %w", werr)
	}
	return img, nil
}

// runWorker is called inside the sandboxed child process.
func runWorker() error {
	mode := os.Getenv(workerModeEnv)
	if mode == "" {
		mode = modePath // legacy default
	}

	switch mode {
	case modePath:
		return runPathWorker()
	case modeStdin:
		return runStdinWorker()
	default:
		return fmt.Errorf("unknown worker mode %q", mode)
	}
}

func runPathWorker() error {
	path := os.Getenv(workerPathEnv)
	if path == "" {
		return fmt.Errorf("%s not set", workerPathEnv)
	}

	clean := filepath.Clean(path)

	// Apply OS restrictions BEFORE touching the file.
	if err := apply(clean); err != nil {
		return fmt.Errorf("sandbox apply: %w", err)
	}

	data, err := os.ReadFile(clean) //#nosec G304,G703 -- worker reads attacker-controlled path by design; Landlock restricts access to this single file
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	img, err := decodeBytes(data)
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	return writePixels(os.Stdout, img)
}

func runStdinWorker() error {
	// No filesystem access needed — lock down before reading bytes.
	if err := apply(""); err != nil {
		return fmt.Errorf("sandbox apply: %w", err)
	}

	data, err := io.ReadAll(io.LimitReader(os.Stdin, maxStdinBytes+1))
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	if len(data) == 0 {
		return fmt.Errorf("read stdin: empty")
	}
	if len(data) > maxStdinBytes {
		return fmt.Errorf("read stdin: payload exceeds %d bytes", maxStdinBytes)
	}

	img, err := decodeBytes(data)
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	return writePixels(os.Stdout, img)
}

// Wire protocol: 4B width + 4B height (little-endian uint32) then raw RGBA bytes.
func writePixels(w io.Writer, img *image.NRGBA) error {
	b := img.Bounds()
	width := uint32(b.Dx())
	height := uint32(b.Dy())

	var hdr [8]byte
	binary.LittleEndian.PutUint32(hdr[0:], width)
	binary.LittleEndian.PutUint32(hdr[4:], height)
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(img.Pix)
	return err
}

func readPixels(r io.Reader) (*image.NRGBA, error) {
	var hdr [8]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	width := int(binary.LittleEndian.Uint32(hdr[0:]))
	height := int(binary.LittleEndian.Uint32(hdr[4:]))

	if width <= 0 || height <= 0 || width > 32768 || height > 32768 {
		return nil, fmt.Errorf("suspicious dimensions %dx%d", width, height)
	}

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	if _, err := io.ReadFull(r, img.Pix); err != nil {
		return nil, fmt.Errorf("read pixels: %w", err)
	}
	return img, nil
}
