package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const publicCacheTTL = 7 * 24 * time.Hour
const downloadReady = "ready"
const downloadManifest = "fetch.json"

type publicVideoCache struct {
	dir string
}

func openPublicVideoCache(dir string) (*publicVideoCache, error) {
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(base, "watch-video", "public-v1")
	}
	path, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	return &publicVideoCache{dir: path}, nil
}

func cacheDiagnostic(err error) {
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "watch-video: public video cache:", err)
	}
}

func cacheDigest(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		fmt.Fprintf(hash, "%d:%s", len(part), part)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (cache *publicVideoCache) fetch(ctx context.Context, tools toolset, input, out string, options fetchOptions) (fetchedVideo, mediaInfo, string, error) {
	if options.subtitleLanguage != "" {
		options.autoSubtitle = false
	}
	// Keep the query string. It can select a different recording or part.
	key := cacheDigest(input, options.subtitleLanguage, strconv.FormatBool(options.autoSubtitle))
	generation, err := cache.latestDownload(key, time.Now())
	cacheDiagnostic(err)
	if generation != "" {
		for _, tool := range []string{"ffmpeg", "ffprobe"} {
			if _, err := tools.version(ctx, tool); err != nil {
				return fetchedVideo{}, mediaInfo{}, "", err
			}
		}
		download, err := restoreDownload(ctx, generation, out)
		if err == nil {
			var info mediaInfo
			info, err = probeVideo(ctx, tools, filepath.Join(out, download.Video))
			if err == nil {
				return download, info, generation, nil
			}
		}
		if ctx.Err() != nil {
			return fetchedVideo{}, mediaInfo{}, "", ctx.Err()
		}
		cacheDiagnostic(err)
		// This is a new request's output directory. Discard any partial copy
		// before letting the downloader write to it.
		if err := os.RemoveAll(out); err != nil {
			return fetchedVideo{}, mediaInfo{}, "", err
		}
		if err := os.Mkdir(out, 0700); err != nil {
			return fetchedVideo{}, mediaInfo{}, "", err
		}
	}
	download, err := fetchVideo(ctx, tools, input, out, options)
	if err != nil {
		return fetchedVideo{}, mediaInfo{}, "", err
	}
	info, err := probeVideo(ctx, tools, filepath.Join(out, download.Video))
	if err != nil {
		return fetchedVideo{}, mediaInfo{}, "", err
	}
	if download.subtitleIncomplete {
		// A failed optional subtitle download is recoverable on a later request.
		return download, info, "", nil
	}
	generation, err = cache.storeDownload(ctx, key, out, download)
	if ctx.Err() != nil {
		return fetchedVideo{}, mediaInfo{}, "", ctx.Err()
	}
	cacheDiagnostic(err)
	return download, info, generation, nil
}

func (cache *publicVideoCache) latestDownload(key string, now time.Time) (string, error) {
	entries, err := os.ReadDir(cache.dir)
	if err != nil {
		return "", err
	}
	var path string
	var newest time.Time
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), key+"-") || !isDownloadGeneration(entry) {
			continue
		}
		generation := filepath.Join(cache.dir, entry.Name())
		marker, err := os.Lstat(filepath.Join(generation, downloadReady))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if marker.Mode().IsRegular() && now.Sub(marker.ModTime()) < publicCacheTTL && marker.ModTime().After(newest) {
			path, newest = generation, marker.ModTime()
		}
	}
	return path, nil
}

func isDownloadGeneration(entry fs.DirEntry) bool {
	key, suffix, ok := strings.Cut(entry.Name(), "-")
	if !entry.IsDir() || !ok || len(key) != 2*sha256.Size {
		return false
	}
	_, keyErr := hex.DecodeString(key)
	_, suffixErr := strconv.ParseUint(suffix, 10, 64)
	return keyErr == nil && suffixErr == nil
}

func restoreDownload(ctx context.Context, generation, out string) (fetchedVideo, error) {
	root, err := os.OpenRoot(generation)
	if err != nil {
		return fetchedVideo{}, err
	}
	defer root.Close()
	data, err := root.ReadFile(downloadManifest)
	if err != nil {
		return fetchedVideo{}, err
	}
	var download fetchedVideo
	if err := json.Unmarshal(data, &download); err != nil {
		return fetchedVideo{}, fmt.Errorf("read cached download: %w", err)
	}
	if !filepath.IsLocal(download.Video) || filepath.Base(download.Video) != download.Video ||
		(download.Video != "video.mkv" && !strings.HasPrefix(download.Video, "source.")) ||
		(download.Subtitle != "" && download.Subtitle != strings.TrimSuffix(download.Video, filepath.Ext(download.Video))+".srt") {
		return fetchedVideo{}, errors.New("cached download contains an invalid file name")
	}
	return download, copyDownload(ctx, root, out, download)
}

func (cache *publicVideoCache) storeDownload(ctx context.Context, key, out string, download fetchedVideo) (string, error) {
	if err := os.MkdirAll(cache.dir, 0700); err != nil {
		return "", err
	}
	generation, err := os.MkdirTemp(cache.dir, key+"-")
	if err != nil {
		return "", err
	}
	ready := false
	defer func() {
		if !ready {
			cacheDiagnostic(os.RemoveAll(generation))
		}
	}()
	root, err := os.OpenRoot(out)
	if err != nil {
		return "", err
	}
	defer root.Close()
	if err := copyDownload(ctx, root, generation, download); err != nil {
		return "", err
	}
	if err := writeJSONFile(filepath.Join(generation, downloadManifest), download); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// Other processes only read a generation once all its files are closed.
	if err := os.WriteFile(filepath.Join(generation, downloadReady), nil, 0600); err != nil {
		return "", err
	}
	ready = true
	return generation, nil
}

func copyDownload(ctx context.Context, source *os.Root, destination string, download fetchedVideo) error {
	for _, name := range []string{download.Video, download.Subtitle} {
		if name == "" {
			continue
		}
		if err := copyCacheFile(ctx, source, destination, name); err != nil {
			return err
		}
	}
	return nil
}

func copyCacheFile(ctx context.Context, source *os.Root, destination, name string) error {
	input, err := source.Open(name)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("%s is not a nonempty regular file", name)
	}
	output, err := os.OpenFile(filepath.Join(destination, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(output, cacheReader{ctx, input})
	return errors.Join(err, output.Close())
}

type cacheReader struct {
	ctx   context.Context
	input io.Reader
}

func (reader cacheReader) Read(p []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.input.Read(p)
}

func (cache *publicVideoCache) prune(now time.Time) error {
	entries, err := os.ReadDir(cache.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var result error
	for _, entry := range entries {
		if !isDownloadGeneration(entry) {
			continue
		}
		path := filepath.Join(cache.dir, entry.Name())
		info, err := os.Lstat(filepath.Join(path, downloadReady))
		if errors.Is(err, os.ErrNotExist) {
			// An interrupted writer can leave an unpublished directory.
			info, err = entry.Info()
		}
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			result = errors.Join(result, err)
		} else if now.Sub(info.ModTime()) >= publicCacheTTL {
			result = errors.Join(result, os.RemoveAll(path))
		}
	}
	return result
}

func cachedTranscription(ctx context.Context, tools toolset, info mediaInfo, captions captionsResult, out, model, language, generation string) (captionsResult, error) {
	weights, err := os.Stat(model)
	if err != nil {
		return captionsResult{}, err
	}
	languageKey := language
	if languageKey == "auto" {
		languageKey = ""
	}
	key := cacheDigest(
		strconv.FormatFloat(captions.Start, 'g', -1, 64),
		strconv.FormatFloat(captions.End, 'g', -1, 64),
		languageKey, model, strconv.FormatInt(weights.Size(), 10),
		strconv.FormatInt(weights.ModTime().UnixNano(), 10),
	)
	name := "captions-" + key + ".json"
	path := filepath.Join(generation, name)
	data, err := readCachedTranscription(generation, name)
	if err == nil {
		var cached captionsResult
		err = json.Unmarshal(data, &cached)
		if err == nil && cached.Source == "whisper" && len(cached.Cues) > 0 &&
			cached.Start == captions.Start && cached.End == captions.End {
			return cached, nil
		}
		if err == nil {
			err = errors.New("cached transcription does not match the request")
		}
	}
	cacheDiagnostic(err)
	result, err := transcribeBatch(ctx, tools, info, captions, out, model, language)
	if err == nil && ctx.Err() == nil && result.Source == "whisper" {
		cacheDiagnostic(storeTranscription(path, result))
	}
	return result, err
}

func readCachedTranscription(generation, name string) ([]byte, error) {
	root, err := os.OpenRoot(generation)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.ReadFile(name)
}

func storeTranscription(path string, result captionsResult) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".captions-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	err = writeJSON(file, result)
	if err := errors.Join(err, file.Close()); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
