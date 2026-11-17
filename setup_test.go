package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

func TestSetupDownloadFallsBackWithoutInstallingBadBytes(t *testing.T) {
	const payload = "pinned installer payload"
	digest := sha256.Sum256([]byte(payload))
	var mu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.URL.Path)
		mu.Unlock()
		if r.URL.Path == "/upstream" {
			io.WriteString(w, "unexpected bytes")
		} else {
			io.WriteString(w, payload)
		}
	}))
	defer server.Close()
	asset := setupAsset{
		name: "installer", sha256: hex.EncodeToString(digest[:]),
		official: []string{server.URL + "/upstream"}, china: []string{server.URL + "/mirror"},
	}
	dist := t.TempDir()
	dest := filepath.Join(dist, "installer.bin")
	if err := os.WriteFile(dest, []byte("previous installation"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := downloadSetupAsset(t.Context(), "official", asset, dest, io.Discard); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("expected a bad upstream checksum, got %v", err)
	}
	if data, err := os.ReadFile(dest); err != nil || string(data) != "previous installation" {
		t.Fatalf("failed download replaced the existing file: %q, %v", data, err)
	}
	if err := downloadSetupAsset(t.Context(), "auto", asset, dest, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := downloadSetupAsset(t.Context(), "cn", asset, dest, io.Discard); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(dest); err != nil || string(data) != payload {
		t.Fatalf("fallback did not install verified bytes: %q, %v", data, err)
	}
	mu.Lock()
	recorded := slices.Clone(requests)
	mu.Unlock()
	if !slices.Equal(recorded, []string{"/upstream", "/upstream", "/mirror"}) {
		t.Fatalf("unexpected routes with fallback and installed-file reuse: %v", recorded)
	}
	if files, err := os.ReadDir(dist); err != nil || len(files) != 1 {
		t.Fatalf("partial download files remain: %v, %v", files, err)
	}
}
