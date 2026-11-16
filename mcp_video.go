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
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const videoEvidenceTool = "video_evidence"
const videoFrameTool = "video_frame"
const videoCaptionsTool = "video_captions"
const transcriptBudget = 12000
const chapterTitleBudget = 120

var videoMCPTools = []mcpTool{
	{
		Name:        videoEvidenceTool,
		Description: "Watch a video in one call. Returns a timestamped storyboard, source chapter labels when available, and captions. Reuse the returned local video path for follow-up.",
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
	{
		Name:        videoCaptionsTool,
		Description: "Read cached dialogue. For a known quote, pass its words in quote to get the exact span of its first and last cues.",
		InputSchema: json.RawMessage(`{
			"type":"object","required":["evidence","start","end"],"additionalProperties":false,
			"properties":{
				"evidence":{"type":"string","description":"Caption reference returned by video_evidence"},
				"start":{"type":"string","description":"Source start time in seconds, MM:SS or HH:MM:SS"},
				"end":{"type":"string","description":"Exclusive source end time"},
				"quote":{"type":"string","description":"Optional words to align across one or more caption cues, as supplied"}
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
		Evidence     string `json:"evidence"`
		At           string `json:"at"`
		Start        string `json:"start"`
		End          string `json:"end"`
		Quote        string `json:"quote"`
		NoASR        bool   `json:"no_asr"`
		SubtitleLang string `json:"subtitle_lang"`
		Language     string `json:"language"`
	}
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("read tool arguments: %w", err)
	}
	if name == videoCaptionsTool {
		return videoCaptionCues(scratch, input.Evidence, input.Start, input.End, input.Quote)
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
		if len(result.Chapters) != 0 {
			body.WriteString("\nSource chapter labels and time ranges (publisher metadata)")
			for _, chapter := range result.Chapters {
				fmt.Fprintf(&body, "\n- %s–%s", formatCueTime(chapter.Start), formatCueTime(chapter.End))
				if label := chapterLabel(chapter.Title); label != "" {
					fmt.Fprintf(&body, "  %s", label)
				}
			}
		}
		if result.Captions.Text != "" {
			data, err := os.ReadFile(filepath.Join(out, result.Captions.Text))
			if err != nil {
				return nil, err
			}
			transcript, sampled := boundedTranscript(string(data))
			fmt.Fprintf(&body, "\nCaption reference: %s\nCaptions (%s):\n%s", filepath.Base(request), result.Captions.Source, transcript)
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

func chapterLabel(title string) string {
	title = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return ' '
		}
		return r
	}, title)
	label := []rune(strings.Join(strings.Fields(title), " "))
	if len(label) > chapterTitleBudget {
		return string(label[:chapterTitleBudget-1]) + "…"
	}
	return string(label)
}

func videoCaptionCues(scratch, reference, startArg, endArg, quote string) ([]mcpContent, error) {
	suffix, ok := strings.CutPrefix(reference, "request-")
	if !ok || suffix == "" || strings.ContainsFunc(suffix, func(r rune) bool { return r < '0' || r > '9' }) {
		return nil, errors.New("evidence must be a caption reference from video_evidence")
	}
	if startArg == "" || endArg == "" {
		return nil, errors.New("start and end are required")
	}
	root, err := os.OpenRoot(scratch)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	data, err := root.ReadFile(filepath.Join(reference, "output", "captions.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("caption reference has expired or has no captions in this MCP session")
	}
	if err != nil {
		return nil, fmt.Errorf("read caption index: %w", err)
	}
	var index captionsResult
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("decode caption index: %w", err)
	}
	start, end, err := parseWindow(startArg, endArg, index.End)
	if err != nil {
		return nil, err
	}
	if start < index.Start {
		return nil, fmt.Errorf("caption evidence starts at %s", formatCueTime(index.Start))
	}
	var cues strings.Builder
	var quoteCues []captionCue
	for _, cue := range index.Cues {
		if cue.Start >= start && cue.End <= end {
			fmt.Fprintf(&cues, "%s --> %s %s\n", formatCueTime(cue.Start), formatCueTime(cue.End), strings.ReplaceAll(cue.Text, "\n", " "))
			if quote != "" {
				quoteCues = append(quoteCues, cue)
			}
		}
	}
	captions, sampled := boundedTranscript(cues.String())
	if captions == "" {
		captions = "No complete caption cues in this interval."
	}
	if sampled {
		captions += "Caption excerpts sampled across this interval. Request a narrower window for complete dialogue."
	}
	var text strings.Builder
	if quote != "" {
		match, err := findCaptionQuote(quoteCues, quote)
		if err != nil {
			fmt.Fprintf(&text, "Quote alignment unavailable: %s.\n", err)
		} else {
			fmt.Fprintf(&text, "Matched quote: %s\nQuote span: %s --> %s\n", match.Text, formatCueTime(match.Start), formatCueTime(match.End))
		}
	}
	fmt.Fprintf(&text, "Captions (%s), %s to %s:\n%s", index.Source, formatCueTime(start), formatCueTime(end), captions)
	return []mcpContent{{Type: "text", Text: text.String()}}, nil
}

func formatCueTime(seconds float64) string {
	micros := int64(math.Round(seconds * 1_000_000))
	whole := micros / 1_000_000
	return fmt.Sprintf("%02d:%02d:%02d.%06d", whole/3600, whole/60%60, whole%60, micros%1_000_000)
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
