package termimage

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/floatpane/termimage/detect"
)

func makePNG(t *testing.T) []byte {
	t.Helper()
	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 64), G: uint8(y * 64), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

func TestProtocolConstants(t *testing.T) {
	// Public re-exports should match the detect package.
	if HalfBlock != detect.HalfBlock {
		t.Errorf("HalfBlock mismatch: %v vs %v", HalfBlock, detect.HalfBlock)
	}
	if Sixel != detect.Sixel {
		t.Errorf("Sixel mismatch: %v vs %v", Sixel, detect.Sixel)
	}
	if Kitty != detect.Kitty {
		t.Errorf("Kitty mismatch: %v vs %v", Kitty, detect.Kitty)
	}
	if Auto != -1 {
		t.Errorf("Auto = %v, want -1", Auto)
	}
}

func TestEffectiveDimensions_ExplicitWidthAndHeight(t *testing.T) {
	w, h := effectiveDimensions(Options{MaxWidth: 800, MaxHeight: 600})
	if w != 800 || h != 600 {
		t.Errorf("got %dx%d, want 800x600", w, h)
	}
}

func TestEffectiveDimensions_ZerosFallBackToDetected(t *testing.T) {
	// With both 0, both come from detection. detectTermPixels returns 1920x1080
	// when /dev/tty isn't available — accept any positive values.
	w, h := effectiveDimensions(Options{})
	if w <= 0 || h <= 0 {
		t.Errorf("expected positive dimensions, got %dx%d", w, h)
	}
}

func TestEffectiveDimensions_PartialOverride(t *testing.T) {
	w, h := effectiveDimensions(Options{MaxWidth: 500})
	if w != 500 {
		t.Errorf("MaxWidth=500 not honoured: got %d", w)
	}
	if h <= 0 {
		t.Errorf("MaxHeight should fall back to detected positive value, got %d", h)
	}
}

func TestDisplay_HalfBlockRendersPNG(t *testing.T) {
	// End-to-end without sandbox: write a PNG, render it half-block, assert
	// the output contains the half-block glyph and an ANSI reset.
	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 64), G: uint8(y * 64), A: 255})
		}
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, src); err != nil {
		t.Fatalf("encode: %v", err)
	}
	path := filepath.Join(t.TempDir(), "img.png")
	if err := os.WriteFile(path, pngBuf.Bytes(), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var out bytes.Buffer
	err := Display(&out, path, Options{
		MaxWidth:  80,
		MaxHeight: 24,
		Protocol:  HalfBlock,
	})
	if err != nil {
		t.Fatalf("Display: %v", err)
	}
	s := out.String()
	if !bytes.Contains(out.Bytes(), []byte("▀")) {
		t.Errorf("expected half-block glyph in output; got %q", s)
	}
	if !bytes.Contains(out.Bytes(), []byte("\x1b[0m")) {
		t.Errorf("expected ANSI reset in output; got %q", s)
	}
}

func TestDisplay_AutoProtocolPicksOne(t *testing.T) {
	// With Auto, detection runs; we just verify it doesn't fail and produces output.
	src := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, src); err != nil {
		t.Fatalf("encode: %v", err)
	}
	path := filepath.Join(t.TempDir(), "img.png")
	if err := os.WriteFile(path, pngBuf.Bytes(), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Force halfblock via env so the test is deterministic across CI runners.
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("TERM", "dumb")
	t.Setenv("TERM_PROGRAM", "")

	var out bytes.Buffer
	if err := Display(&out, path, Options{MaxWidth: 40, MaxHeight: 20, Protocol: Auto}); err != nil {
		t.Fatalf("Display: %v", err)
	}
	if out.Len() == 0 {
		t.Errorf("Display produced no output")
	}
}

func TestDisplay_DataURI(t *testing.T) {
	pngBytes := makePNG(t)
	uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)

	var out bytes.Buffer
	err := Display(&out, uri, Options{
		MaxWidth:  80,
		MaxHeight: 24,
		Protocol:  HalfBlock,
	})
	if err != nil {
		t.Fatalf("Display: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("▀")) {
		t.Errorf("expected half-block glyph in output")
	}
}

func TestDisplay_RemoteHTTP(t *testing.T) {
	pngBytes := makePNG(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(pngBytes)
	}))
	defer srv.Close()

	var out bytes.Buffer
	err := Display(&out, srv.URL+"/cat.png", Options{
		MaxWidth:  80,
		MaxHeight: 24,
		Protocol:  HalfBlock,
	})
	if err != nil {
		t.Fatalf("Display: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("▀")) {
		t.Errorf("expected half-block glyph in output")
	}
}

func TestDisplay_MissingFile(t *testing.T) {
	var out bytes.Buffer
	err := Display(&out, "/no/such/image.png", Options{Protocol: HalfBlock, MaxWidth: 80, MaxHeight: 24})
	if err == nil {
		t.Error("expected error for missing file")
	}
}
