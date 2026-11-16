package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const videoEvidenceTool = "video_evidence"
const videoFrameTool = "video_frame"
const transcriptBudget = 12000

var videoMCPTools = []mcpTool{
	{
		Name:        videoEvidenceTool,
		Description: "Watch a video in one call. Returns one storyboard and timestamped captions. Reuse the returned local video path for follow-up.",
		InputSchema: json.RawMessage(`{
			"type":"object","required":["input"],"additionalProperties":false,
			"properties":{
				"input":{"type":"string","description":"Local video path or public recording URL"},
				"start":{"type":"string","description":"Optional source time in seconds, MM:SS or HH:MM:SS"},
				"end":{"type":"string","description":"Optional exclusive playback cutoff"},
				"no_asr":{"type":"boolean","description":"Skip local speech recognition for visual tasks"},
				"subtitle_lang":{"type":"string","description":"Exact public platform subtitle tag if known"},
				"language":{"type":"string","description":"Preferred subtitle or speech recognition language code if known"}
			}
		}`),
	},
	{
		Name:        videoFrameTool,
		Description: "Open one detailed frame for a known moment. Pass the timestamp the user gave unchanged, even when they say around that time.",
		InputSchema: json.RawMessage(`{
			"type":"object","required":["input","at"],"additionalProperties":false,
			"properties":{
				"input":{"type":"string","description":"Local video path returned by video_evidence"},
				"at":{"type":"string","description":"Source time as written, for example 00:16"},
				"end":{"type":"string","description":"Optional exclusive playback cutoff"}
			}
		}`),
	},
}

func runVideoMCP(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer) error {
	flags := flag.NewFlagSet("mcp", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	toolsDir := flags.String("tools-dir", "", "directory containing media tools")
	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		_, err = fmt.Fprintln(stdout, "Usage: watch-video mcp [--tools-dir DIR]")
		return err
	} else if err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("mcp takes no input file on the command line")
	}
	scratch, err := os.MkdirTemp("", "watch-video-mcp-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	return serveMCP(ctx, stdin, stdout, videoMCPTools, func(ctx context.Context, name string, input json.RawMessage) ([]mcpContent, error) {
		return runVideoMCPTool(ctx, scratch, *toolsDir, name, input)
	})
}

func runVideoMCPTool(ctx context.Context, scratch, toolsDir, name string, data json.RawMessage) ([]mcpContent, error) {
	var input struct {
		Input        string `json:"input"`
		At           string `json:"at"`
		Start        string `json:"start"`
		End          string `json:"end"`
		NoASR        bool   `json:"no_asr"`
		SubtitleLang string `json:"subtitle_lang"`
		Language     string `json:"language"`
	}
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("read tool arguments: %w", err)
	}
	if input.Input == "" {
		return nil, errors.New("input is required")
	}
	command := "evidence"
	if name == videoFrameTool {
		if input.At == "" {
			return nil, errors.New("at is required")
		}
		command = "focus"
	}
	request, err := os.MkdirTemp(scratch, "request-")
	if err != nil {
		return nil, err
	}
	out := filepath.Join(request, "output")
	args := []string{command, "--out", out}
	for _, option := range []struct{ flag, value string }{
		{"--tools-dir", toolsDir}, {"--end", input.End},
	} {
		if option.value != "" {
			args = append(args, option.flag, option.value)
		}
	}
	if command == "focus" {
		args = append(args, "--at", input.At)
	} else {
		for _, option := range []struct{ flag, value string }{
			{"--start", input.Start}, {"--subtitle-lang", input.SubtitleLang}, {"--language", input.Language},
		} {
			if option.value != "" {
				args = append(args, option.flag, option.value)
			}
		}
		if input.NoASR {
			args = append(args, "--no-asr")
		}
	}
	if err := run(ctx, append(args, "--", input.Input), io.Discard); err != nil {
		_ = os.RemoveAll(request)
		return nil, err
	}
	manifest, err := os.ReadFile(filepath.Join(out, command+".json"))
	if err != nil {
		return nil, err
	}
	var result evidenceResult
	if err := json.Unmarshal(manifest, &result); err != nil {
		return nil, err
	}
	var body strings.Builder
	if command == "focus" {
		fmt.Fprintf(&body, "Source frame time: %.6f seconds.", result.Frames[0].Time)
	} else {
		fmt.Fprintf(&body, "Local video: %s\nWindow: %.3f to %.3f seconds (duration %.3f).\nStoryboard tiles:", result.Video, result.Start, result.End, result.Duration)
		for _, frame := range result.Frames {
			fmt.Fprintf(&body, " row %d col %d = %.3f s;", frame.Row, frame.Column, frame.Time)
		}
		if result.Captions.Text != "" {
			data, err := os.ReadFile(filepath.Join(out, result.Captions.Text))
			if err != nil {
				return nil, err
			}
			transcript, sampled := boundedTranscript(string(data))
			fmt.Fprintf(&body, "\nCaptions (%s):\n%s", result.Captions.Source, transcript)
			if sampled {
				body.WriteString("\nCaption excerpts sampled across this window. Request a narrower window for missing dialogue.")
			}
		} else {
			fmt.Fprintf(&body, "\nCaptions unavailable (%s).", result.Captions.Status)
			if result.Captions.Error != "" {
				fmt.Fprintf(&body, " %s", result.Captions.Error)
			}
		}
	}
	image, err := os.ReadFile(filepath.Join(out, result.Images[0].Path))
	if err != nil {
		return nil, err
	}
	return []mcpContent{
		{Type: "text", Text: body.String()},
		{Type: "image", MimeType: "image/jpeg", Data: image},
	}, nil
}

func boundedTranscript(text string) (string, bool) {
	if utf8.RuneCountInString(text) <= transcriptBudget {
		return text, false
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	count := min(len(lines), transcriptBudget/120)
	perLine := transcriptBudget/count - 1
	var result strings.Builder
	for i := range count {
		line := []rune(lines[i*(len(lines)-1)/max(1, count-1)])
		if len(line) > perLine {
			line = append(line[:perLine-1], '…')
		}
		result.WriteString(string(line))
		result.WriteByte('\n')
	}
	return result.String(), true
}
