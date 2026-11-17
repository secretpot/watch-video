package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPublicVideoCacheExpiresDownloadsAndInterruptedWrites(t *testing.T) {
	cache, err := openPublicVideoCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	download := fetchedVideo{Video: "source.mp4"}
	if err := os.WriteFile(filepath.Join(out, download.Video), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Truncate(time.Second)
	expiredAt := now.Add(-publicCacheTTL)
	store := func(key string, created time.Time) string {
		t.Helper()
		path, err := cache.storeDownload(t.Context(), key, out, download)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(filepath.Join(path, downloadReady), created, created); err != nil {
			t.Fatal(err)
		}
		return path
	}
	expiredKey := cacheDigest("expired.example")
	expired := store(expiredKey, expiredAt)
	freshKey := cacheDigest("fresh.example")
	fresh := store(freshKey, expiredAt.Add(time.Second))
	interrupted, err := os.MkdirTemp(cache.dir, cacheDigest("interrupted.example")+"-")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(interrupted, expiredAt, expiredAt); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(cache.dir, "user-files")
	if err := os.Mkdir(unrelated, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(unrelated, expiredAt, expiredAt); err != nil {
		t.Fatal(err)
	}

	if hit, err := cache.latestDownload(expiredKey, now); err != nil || hit != "" {
		t.Fatalf("seven-day-old entry was still available: %q, %v", hit, err)
	}
	if hit, err := cache.latestDownload(freshKey, now); err != nil || hit != fresh {
		t.Fatalf("unexpired entry was unavailable: %q, %v", hit, err)
	}
	if err := cache.prune(now); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{expired, interrupted} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expired cache directory %s remains: %v", path, err)
		}
	}
	for _, path := range []string{fresh, unrelated} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("cleanup removed %s: %v", path, err)
		}
	}
}
