package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type setupMirror string

type setupAsset struct {
	name, sha256    string
	official, china []string
}

const setupDownloadLimit = 256 << 20

const modelRevision = "/ggerganov/whisper.cpp/resolve/f281eb45af861ab5e5297d23694b7d46e090c02c/ggml-base-q5_1.bin"

var setupWhisperModel = setupAsset{
	name:     "Whisper multilingual model",
	sha256:   "422f1ae452ade6f30a004d7e5c6a43195e4433bc370bf23fac9cc591f01a8898",
	official: []string{"https://huggingface.co" + modelRevision},
	china:    []string{"https://hf-mirror.com" + modelRevision},
}

func runSetup(ctx context.Context, args []string, stdout, progress io.Writer) error {
	flags := flag.NewFlagSet("setup", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	mirror := flags.String("mirror", "auto", "download route: auto, cn, or official")
	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		flags.SetOutput(stdout)
		fmt.Fprintln(stdout, "Usage: watch-video setup [--mirror auto|cn|official]")
		flags.PrintDefaults()
		return nil
	} else if err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("setup takes no input file")
	}
	mode := setupMirror(*mirror)
	switch mode {
	case "auto", "cn", "official":
	default:
		return fmt.Errorf("unknown setup mirror %q: use auto, cn, or official", mode)
	}
	if err := provisionInstallation(ctx, mode, stdout, progress); err != nil {
		return fmt.Errorf("local transcription is not ready: %w\nRetry with watch-video setup; use watch-video setup --mirror cn to try mainland China routes first", err)
	}
	return nil
}

func provisionInstallation(ctx context.Context, mode setupMirror, stdout, progress io.Writer) error {
	modelPath, err := defaultWhisperModelPath()
	if err != nil {
		return err
	}
	dataDir := filepath.Dir(modelPath)
	tools, err := setupMediaTools(ctx, mode, dataDir, progress)
	if err != nil {
		return fmt.Errorf("prepare FFmpeg and Whisper media tools: %w", err)
	}
	if err := downloadSetupAsset(ctx, mode, setupWhisperModel, modelPath, progress); err != nil {
		return err
	}
	return verifyInstall(ctx, tools, stdout, "")
}

func (mode setupMirror) sources(asset setupAsset) []string {
	urls := append([]string(nil), asset.official...)
	switch mode {
	case "official":
		return urls
	case "cn":
		return append(append([]string(nil), asset.china...), urls...)
	default:
		return append(urls, asset.china...)
	}
}

func downloadSetupAsset(ctx context.Context, mode setupMirror, asset setupAsset, dest string, progress io.Writer) error {
	if existing, err := setupFileDigest(dest); err == nil && existing == asset.sha256 {
		return nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check existing %s: %w", asset.name, err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 25 * time.Second
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   6 * time.Minute,
		CheckRedirect: func(request *http.Request, redirects []*http.Request) error {
			if len(redirects) >= 10 || request.URL.Scheme != "https" {
				return errors.New("unsafe or excessive download redirects")
			}
			return nil
		},
	}
	var failures []error
	for _, source := range mode.sources(asset) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fmt.Fprintf(progress, "Downloading %s from %s\n", asset.name, source)
		if err := fetchSetupAsset(ctx, client, source, asset.sha256, dest); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", source, err))
			continue
		}
		return nil
	}
	return fmt.Errorf("download %s: %w", asset.name, errors.Join(failures...))
}

func setupFileDigest(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > setupDownloadLimit {
		return "", fmt.Errorf("%q must be a regular file no larger than %d bytes", path, setupDownloadLimit)
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func fetchSetupAsset(ctx context.Context, client *http.Client, source, digest, dest string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status %s", response.Status)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".watch-video-download-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(response.Body, setupDownloadLimit+1))
	if err := errors.Join(copyErr, tmp.Close()); err != nil {
		return err
	}
	if written > setupDownloadLimit || hex.EncodeToString(hash.Sum(nil)) != digest {
		return errors.New("SHA-256 mismatch or download exceeded the size limit")
	}
	return os.Rename(tmp.Name(), dest)
}
