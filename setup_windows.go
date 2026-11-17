package main

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	windowsFFmpegVersion = "9.0.2"
	windowsFFmpegFolder  = "ffmpeg-" + windowsFFmpegVersion + "-essentials_build"
	windowsFFmpegZip     = windowsFFmpegFolder + ".zip"
	windowsFFmpegURL     = "https://github.com/GyanD/codexffmpeg/releases/download/" + windowsFFmpegVersion + "/" + windowsFFmpegZip
	windowsWhisperZip    = "whisper-bin-x64.zip"
	windowsWhisperURL    = "https://github.com/ggml-org/whisper.cpp/releases/download/b5454/" + windowsWhisperZip
)

type windowsToolArchive struct {
	asset  setupAsset
	prefix string
	files  []string
}

var windowsToolArchives = []windowsToolArchive{
	{
		asset: setupAsset{
			name:   windowsFFmpegZip,
			sha256: "60f467265b1e312373dbcd92200c2618a74850f98d3d078e94296bb3fa2047ba",
			official: []string{
				"https://www.gyan.dev/ffmpeg/builds/packages/" + windowsFFmpegZip,
				windowsFFmpegURL,
			},
			china: []string{"https://gh-proxy.com/" + windowsFFmpegURL},
		},
		prefix: windowsFFmpegFolder + "/bin/",
		files:  []string{"ffmpeg.exe", "ffprobe.exe"},
	},
	{
		asset: setupAsset{
			name:     windowsWhisperZip,
			sha256:   "6ba69e3482d7826214f90a6a9c84ca07782aec1e1d0c6a7c30c994fd5d816ccb",
			official: []string{windowsWhisperURL},
			china:    []string{"https://gh-proxy.com/" + windowsWhisperURL},
		},
		prefix: "Release/",
		files: []string{
			"ggml-base.dll", "ggml.dll",
			"ggml-cpu-alderlake.dll", "ggml-cpu-cannonlake.dll", "ggml-cpu-cascadelake.dll",
			"ggml-cpu-haswell.dll", "ggml-cpu-icelake.dll", "ggml-cpu-sandybridge.dll",
			"ggml-cpu-skylakex.dll", "ggml-cpu-sse42.dll", "ggml-cpu-x64.dll",
			"llama.dll", "parakeet.dll", "SDL2.dll", "whisper.dll", "whisper-cli.exe",
		},
	},
}

var errWindowsWhisperRuntime = errors.New("Whisper could not load its Windows DLLs. Install the Microsoft Visual C++ v14 Redistributable (x64) from https://aka.ms/vc14/vc_redist.x64.exe, then retry watch-video setup")

func setupMediaTools(ctx context.Context, mode setupMirror, dataDir string, progress io.Writer) (toolset, error) {
	if runtime.GOARCH != "amd64" {
		return toolset{}, errors.New("Windows setup requires an x64 build of watch-video")
	}
	tools := toolset{dir: filepath.Join(dataDir, "tools")}
	if err := os.MkdirAll(tools.dir, 0o700); err != nil {
		return toolset{}, err
	}
	for _, bundle := range windowsToolArchives {
		if err := checkWindowsToolArchive(ctx, tools, bundle); err == nil {
			continue
		} else if errors.Is(err, errWindowsWhisperRuntime) {
			return toolset{}, err
		}
		if err := installWindowsToolArchive(ctx, mode, tools, bundle, progress); err != nil {
			return toolset{}, fmt.Errorf("install %s: %w", bundle.asset.name, err)
		}
	}
	return tools, nil
}

func checkWindowsToolArchive(ctx context.Context, tools toolset, bundle windowsToolArchive) error {
	for _, name := range bundle.files {
		if _, err := regularFile(filepath.Join(tools.dir, name)); err != nil {
			return err
		}
	}
	for _, name := range bundle.files {
		if command, ok := strings.CutSuffix(name, ".exe"); ok {
			if _, err := tools.version(ctx, command); err != nil {
				var exitErr *exec.ExitError
				if command == "whisper-cli" && errors.As(err, &exitErr) {
					switch uint32(exitErr.ExitCode()) {
					case 0xc0000135, 0xc0000139: // DLL or entry point missing.
						return fmt.Errorf("%w: %w", errWindowsWhisperRuntime, err)
					}
				}
				return err
			}
		}
	}
	return nil
}

func installWindowsToolArchive(ctx context.Context, mode setupMirror, tools toolset, bundle windowsToolArchive, progress io.Writer) error {
	stage, err := os.MkdirTemp(filepath.Dir(tools.dir), "setup-tools-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	archive := filepath.Join(stage, bundle.asset.name)
	if err := downloadSetupAsset(ctx, mode, bundle.asset, archive, progress); err != nil {
		return err
	}
	if err := extractWindowsToolArchive(archive, stage, bundle); err != nil {
		return err
	}
	if err := checkWindowsToolArchive(ctx, toolset{dir: stage}, bundle); err != nil {
		return err
	}
	for _, name := range bundle.files {
		if err := os.Rename(filepath.Join(stage, name), filepath.Join(tools.dir, name)); err != nil {
			return err
		}
	}
	return nil
}

func extractWindowsToolArchive(archive, destination string, bundle windowsToolArchive) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer reader.Close()
	entries := make(map[string]*zip.File, len(bundle.files))
	for _, name := range bundle.files {
		entries[bundle.prefix+name] = nil
	}
	for _, file := range reader.File {
		if previous, selected := entries[file.Name]; selected {
			if previous != nil {
				return fmt.Errorf("duplicate archive member %q", file.Name)
			}
			entries[file.Name] = file
		}
	}
	for _, name := range bundle.files {
		file := entries[bundle.prefix+name]
		if file == nil {
			return fmt.Errorf("missing archive member %q", bundle.prefix+name)
		}
		if err := extractWindowsToolFile(file, filepath.Join(destination, name)); err != nil {
			return fmt.Errorf("extract %s: %w", name, err)
		}
	}
	return nil
}

func extractWindowsToolFile(file *zip.File, destination string) error {
	const maxToolSize = 256 << 20
	if !file.Mode().IsRegular() || file.UncompressedSize64 > maxToolSize {
		return errors.New("invalid file type or size")
	}
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		return err
	}
	_, err = io.Copy(output, reader)
	return errors.Join(err, output.Close())
}
