// Package source resolves image sources: file paths, data URIs, and remote URLs.
package source

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Kind tags how the source should be loaded.
type Kind int

const (
	KindFile Kind = iota
	KindBytes
)

// Resolved is the resolver result. Exactly one of Path or Bytes is set.
type Resolved struct {
	Kind  Kind
	Path  string // when Kind == KindFile
	Bytes []byte // when Kind == KindBytes
}

// MaxRemoteBytes caps remote/data URI payloads to prevent OOM on hostile servers.
const MaxRemoteBytes = 64 * 1024 * 1024

// Resolve inspects src and returns either a file path or pre-loaded bytes.
// HTTP(S) URLs and data: URIs are fetched/decoded here; everything else is
// treated as a file path.
func Resolve(ctx context.Context, src string) (*Resolved, error) {
	if strings.HasPrefix(src, "data:") {
		b, err := decodeDataURI(src)
		if err != nil {
			return nil, err
		}
		return &Resolved{Kind: KindBytes, Bytes: b}, nil
	}

	if u, err := url.Parse(src); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		b, err := fetch(ctx, src)
		if err != nil {
			return nil, err
		}
		return &Resolved{Kind: KindBytes, Bytes: b}, nil
	}

	return &Resolved{Kind: KindFile, Path: src}, nil
}

// decodeDataURI parses RFC 2397 data: URIs. Only base64-encoded payloads are
// supported (the common form for images); plain percent-encoded payloads are
// rejected — callers wanting that should pre-decode.
func decodeDataURI(s string) ([]byte, error) {
	rest := strings.TrimPrefix(s, "data:")
	comma := strings.IndexByte(rest, ',')
	if comma < 0 {
		return nil, fmt.Errorf("data URI: missing comma")
	}
	meta, payload := rest[:comma], rest[comma+1:]
	if !strings.Contains(meta, "base64") {
		return nil, fmt.Errorf("data URI: only base64 payloads supported")
	}
	b, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("data URI: base64 decode: %w", err)
	}
	if len(b) > MaxRemoteBytes {
		return nil, fmt.Errorf("data URI: payload exceeds %d bytes", MaxRemoteBytes)
	}
	return b, nil
}

func fetch(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("remote: build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remote: fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remote: HTTP %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, MaxRemoteBytes+1))
	if err != nil {
		return nil, fmt.Errorf("remote: read body: %w", err)
	}
	if len(b) > MaxRemoteBytes {
		return nil, fmt.Errorf("remote: response exceeds %d bytes", MaxRemoteBytes)
	}
	return b, nil
}
