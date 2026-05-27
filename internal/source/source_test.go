package source

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolve_File(t *testing.T) {
	r, err := Resolve(context.Background(), "/tmp/cat.png")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if r.Kind != KindFile {
		t.Errorf("Kind = %v, want KindFile", r.Kind)
	}
	if r.Path != "/tmp/cat.png" {
		t.Errorf("Path = %q", r.Path)
	}
}

func TestResolve_DataURI(t *testing.T) {
	payload := []byte("hello-bytes")
	uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString(payload)
	r, err := Resolve(context.Background(), uri)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if r.Kind != KindBytes {
		t.Fatalf("Kind = %v, want KindBytes", r.Kind)
	}
	if string(r.Bytes) != string(payload) {
		t.Errorf("Bytes mismatch: %q", r.Bytes)
	}
}

func TestResolve_DataURI_RejectsNonBase64(t *testing.T) {
	_, err := Resolve(context.Background(), "data:text/plain,hello")
	if err == nil {
		t.Error("expected error for non-base64 data URI")
	}
}

func TestResolve_DataURI_BadBase64(t *testing.T) {
	_, err := Resolve(context.Background(), "data:image/png;base64,!!!not-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestResolve_HTTP(t *testing.T) {
	body := []byte("pretend-png-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	r, err := Resolve(context.Background(), srv.URL+"/x.png")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if r.Kind != KindBytes {
		t.Fatalf("Kind = %v, want KindBytes", r.Kind)
	}
	if string(r.Bytes) != string(body) {
		t.Errorf("body mismatch: %q", r.Bytes)
	}
}

func TestResolve_HTTP_NonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := Resolve(context.Background(), srv.URL+"/missing.png")
	if err == nil {
		t.Error("expected error for HTTP 404")
	}
	if err != nil && !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention status: %v", err)
	}
}
