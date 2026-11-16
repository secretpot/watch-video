package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const usage = `Usage: watch-video <command> [options] [input]

  doctor                                 Check media dependencies
  probe INPUT                            Inspect a local video
  fetch URL --out NEWDIR                 Download one public recorded video
  overview INPUT --out NEWDIR            Create a timestamped contact sheet
  focus INPUT --start SEC --end SEC --out NEWDIR
                                         Extract up to six high-resolution frames
  captions INPUT --out NEWDIR            Read or locally transcribe dialogue

Use watch-video <command> --help for command options.`

type toolset struct {
	dir string
}

func (tools toolset) path(name string) (string, error) {
	if tools.dir != "" {
		executable := name
		if runtime.GOOS == "windows" {
			executable += ".exe"
		}
		path, err := exec.LookPath(filepath.Join(tools.dir, executable))
		if err == nil {
			return path, nil
		}
		if !errors.Is(err, exec.ErrNotFound) && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("find %s in PATH or --tools-dir: %w", name, err)
	}
	return filepath.Abs(path)
}

func (tools toolset) run(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	path, err := tools.path(name)
	if err != nil {
		return nil, nil, err
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.WaitDelay = 2 * time.Second
	var stderr strings.Builder
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if len(detail) > 3000 {
			detail = detail[len(detail)-3000:]
		}
		if detail != "" {
			err = fmt.Errorf("%s: %w\n%s", name, err, detail)
		} else {
			err = fmt.Errorf("%s: %w", name, err)
		}
	}
	return stdout, []byte(stderr.String()), err
}

var ffmpegVersion = regexp.MustCompile(`^\S+ version n?(\d+)\.`)

func (tools toolset) version(ctx context.Context, name string) (string, error) {
	option := "--version"
	switch name {
	case "ffmpeg", "ffprobe":
		option = "-version"
	case "whisper-cli":
		// whisper-cli has no version switch. --help starts no model and
		// confirms that the executable and any companion DLLs can load.
		_, _, err := tools.run(ctx, name, "--help")
		return "version unavailable from whisper-cli", err
	}
	stdout, _, err := tools.run(ctx, name, option)
	if err != nil {
		return "", err
	}
	version, _, _ := strings.Cut(strings.TrimSpace(string(stdout)), "\n")
	if name == "ffmpeg" || name == "ffprobe" {
		match := ffmpegVersion.FindStringSubmatch(version)
		if len(match) != 2 {
			return version, fmt.Errorf("%s: could not read release version (8.0+ required)", name)
		}
		major, err := strconv.Atoi(match[1])
		if err != nil || major < 8 {
			return version, fmt.Errorf("%s 8.0+ required, found %s", name, version)
		}
	}
	return strings.TrimSpace(version), nil
}

type toolStatus struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Path     string `json:"path,omitempty"`
	Version  string `json:"version,omitempty"`
	Error    string `json:"error,omitempty"`
}

func doctor(ctx context.Context, tools toolset, stdout io.Writer) error {
	statuses := make([]toolStatus, 0, 4)
	ready := true
	for _, name := range []string{"ffmpeg", "ffprobe", "yt-dlp", "whisper-cli"} {
		status := toolStatus{Name: name, Required: name == "ffmpeg" || name == "ffprobe"}
		path, err := tools.path(name)
		if err == nil {
			status.Path = path
			status.Version, err = tools.version(ctx, name)
		}
		if err != nil {
			status.Error = err.Error()
			ready = ready && !status.Required
		}
		statuses = append(statuses, status)
	}
	report := struct {
		Ready bool         `json:"ready"`
		Tools []toolStatus `json:"tools"`
	}{ready, statuses}
	if err := writeJSON(stdout, report); err != nil {
		return err
	}
	if !ready {
		return errors.New("required media tools are missing or unsupported")
	}
	return nil
}

func writeJSON(dst io.Writer, result any) error {
	encoder := json.NewEncoder(dst)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func regularFile(path string) (string, error) {
	if path == "" {
		return "", errors.New("a file path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%q is not a regular file", path)
	}
	return absolute, nil
}

// parseInput allows options before and after the single positional input.
func parseInput(flags *flag.FlagSet, args []string) ([]string, error) {
	var positionals []string
	for len(args) > 0 {
		arg := args[0]
		if arg == "--" {
			return append(positionals, args[1:]...), nil
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			args = args[1:]
			continue
		}
		// FlagSet also handles --help and boolean options without a value.
		count := 1
		if len(args) > 1 && !strings.Contains(arg, "=") {
			count++
			if option := flags.Lookup(strings.TrimLeft(arg, "-")); option != nil {
				if boolean, ok := option.Value.(interface{ IsBoolFlag() bool }); ok && boolean.IsBoolFlag() {
					count = 1
				}
			}
		}
		if err := flags.Parse(args[:count]); err != nil {
			return nil, err
		}
		args = args[count:]
	}
	return positionals, nil
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	command := args[0]
	if command == "--help" || command == "-h" || command == "help" {
		_, err := fmt.Fprintln(stdout, usage)
		return err
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	toolsDir := flags.String("tools-dir", "", "directory containing media tools, searched before PATH")
	var out, startArg, endArg, subtitle, model, language string
	var frames int
	var batch bool
	switch command {
	case "doctor", "probe":
	case "fetch", "overview", "focus", "captions":
		flags.StringVar(&out, "out", "", "new output directory with an existing parent")
	default:
		return fmt.Errorf("unknown command %q\n%s", command, usage)
	}
	switch command {
	case "overview", "focus", "captions":
		flags.StringVar(&startArg, "start", "", "start time in seconds")
		flags.StringVar(&endArg, "end", "", "exclusive end time in seconds")
	}
	switch command {
	case "focus":
		flags.IntVar(&frames, "frames", 6, "number of frames (1–6)")
	case "fetch":
		flags.StringVar(&language, "subtitle-lang", "", "one exact platform subtitle language tag")
	case "captions":
		flags.StringVar(&subtitle, "subtitle", "", "SRT or VTT file on the source timeline")
		flags.StringVar(&model, "whisper-model", "", "local whisper.cpp model (enables fallback transcription)")
		flags.StringVar(&language, "language", "", "preferred language or Whisper language code")
		flags.BoolVar(&batch, "batch", false, "process longer intervals locally and write compact timestamped text")
	}
	positionals, err := parseInput(flags, args[1:])
	if errors.Is(err, flag.ErrHelp) {
		flags.SetOutput(stdout)
		fmt.Fprintf(stdout, "Usage: watch-video %s [options]", command)
		switch command {
		case "fetch":
			fmt.Fprintln(stdout, " URL")
		case "doctor":
			fmt.Fprintln(stdout)
		default:
			fmt.Fprintln(stdout, " INPUT")
		}
		flags.PrintDefaults()
		return nil
	}
	if err != nil {
		return err
	}
	var tools toolset
	if *toolsDir != "" {
		tools.dir, err = filepath.Abs(*toolsDir)
		if err != nil {
			return err
		}
		info, err := os.Stat(tools.dir)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("--tools-dir %q is not a directory", *toolsDir)
		}
	}
	if command == "doctor" {
		if len(positionals) != 0 {
			return errors.New("doctor takes no input")
		}
		return doctor(ctx, tools, stdout)
	}
	if len(positionals) != 1 {
		return fmt.Errorf("%s requires exactly one input", command)
	}
	input := positionals[0]
	if command != "probe" && out == "" {
		return errors.New("--out is required")
	}
	if command == "fetch" {
		return withOutput(out, command, stdout, func(dir string) (any, error) {
			return fetchVideo(ctx, tools, input, language, dir)
		})
	}
	input, err = regularFile(input)
	if err != nil {
		return err
	}
	if _, err := tools.version(ctx, "ffprobe"); err != nil {
		return err
	}
	info, err := probeVideo(ctx, tools, input)
	if err != nil {
		return err
	}
	if command == "probe" {
		return writeJSON(stdout, info)
	}
	if command == "focus" && (startArg == "" || endArg == "" || frames < 1 || frames > 6) {
		return errors.New("focus requires --start, --end and 1–6 frames")
	}
	start, end, err := parseWindow(startArg, endArg, info.Duration)
	if err != nil {
		return err
	}
	if _, err := tools.version(ctx, "ffmpeg"); err != nil {
		return err
	}
	if command == "captions" {
		if subtitle == "" {
			subtitle, err = adjacentSubtitle(input)
			if err != nil {
				return err
			}
		} else if subtitle, err = regularFile(subtitle); err != nil {
			return err
		}
		if model != "" {
			model, err = regularFile(model)
			if err != nil {
				return err
			}
		}
		if batch {
			return runCaptionBatch(ctx, tools, info, start, end, out, subtitle, model, language, stdout)
		}
		return withOutput(out, command, stdout, func(dir string) (any, error) {
			return collectCaptions(ctx, tools, info, start, end, dir, subtitle, model, language)
		})
	}
	return withOutput(out, command, stdout, func(dir string) (any, error) {
		return renderVideo(ctx, tools, info, start, end, dir, command == "focus", frames)
	})
}

func parseWindow(startArg, endArg string, duration float64) (float64, float64, error) {
	parse := func(value, option string, fallback float64) (float64, error) {
		if value == "" {
			return fallback, nil
		}
		seconds, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
			return 0, fmt.Errorf("%s requires a finite time in seconds", option)
		}
		return seconds, nil
	}
	start, err := parse(startArg, "--start", 0)
	if err != nil {
		return 0, 0, err
	}
	end, err := parse(endArg, "--end", duration)
	if err != nil {
		return 0, 0, err
	}
	if start < 0 || end <= start || end > duration {
		return 0, 0, fmt.Errorf("require 0 <= start < end <= %.6f seconds", duration)
	}
	return start, end, nil
}

func adjacentSubtitle(input string) (string, error) {
	stem := strings.TrimSuffix(input, filepath.Ext(input))
	for _, extension := range []string{".srt", ".vtt"} {
		if filepath.Ext(input) == extension {
			continue
		}
		path, err := regularFile(stem + extension)
		if !errors.Is(err, os.ErrNotExist) {
			return path, err
		}
	}
	return "", nil
}

func withOutput(path, command string, stdout io.Writer, work func(string) (any, error)) error {
	out, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if err := os.Mkdir(out, 0o700); err != nil {
		return fmt.Errorf("create new output directory: %w", err)
	}
	complete := false
	defer func() {
		if !complete {
			if err := os.RemoveAll(out); err != nil {
				fmt.Fprintln(os.Stderr, "watch-video: clean incomplete output:", err)
			}
		}
	}()
	result, err := work(out)
	if err != nil {
		return err
	}
	manifest, err := os.Create(filepath.Join(out, command+".json"))
	if err != nil {
		return err
	}
	writeErr := writeJSON(manifest, result)
	if err := errors.Join(writeErr, manifest.Close()); err != nil {
		return err
	}
	complete = true
	return writeJSON(stdout, result)
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "watch-video:", err)
		os.Exit(1)
	}
}
