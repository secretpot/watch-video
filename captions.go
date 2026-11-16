package main

import (
	"context"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

type captionCue struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

type captionsResult struct {
	Source   string       `json:"source"`
	Start    float64      `json:"start"`
	End      float64      `json:"end"`
	Language string       `json:"language,omitempty"`
	Cues     []captionCue `json:"cues"`
}

var (
	captionTimestamp = regexp.MustCompile(`^(?:([0-9]{2,}):)?([0-5][0-9]):([0-5][0-9])[.,]([0-9]{3})$`)
	captionMarkup    = regexp.MustCompile(`(?i)</?(?:b|i|u|font|c|v|ruby|rt|lang)(?:[. 	][^<>]*)?>|<(?:[0-9]{2,}:)?[0-9]{2}:[0-9]{2}\.[0-9]{3}>`)
)

func collectCaptions(ctx context.Context, tools toolset, info mediaInfo, start, end float64, out, subtitle, model, language string) (any, error) {
	result := captionsResult{Source: "none", Start: start, End: end, Cues: []captionCue{}}
	if subtitle != "" {
		switch strings.ToLower(filepath.Ext(subtitle)) {
		case ".srt", ".vtt":
		default:
			return nil, fmt.Errorf("subtitle must be an SRT or WebVTT file: %s", subtitle)
		}
		data, err := os.ReadFile(subtitle)
		if err != nil {
			return nil, err
		}
		cues, err := captionsInWindow(data, start, end, 0)
		if err != nil {
			return nil, fmt.Errorf("read subtitle %s: %w", subtitle, err)
		}
		if len(cues) > 0 {
			result.Source, result.Cues = "sidecar", cues
			return result, nil
		}
	}
	for _, track := range captionTracks(info.Subtitles, language) {
		data, _, err := tools.run(ctx, "ffmpeg", "-nostdin", "-v", "error", "-copyts", "-start_at_zero",
			"-protocol_whitelist", "file,pipe", "-i", info.Input, "-map", "0:"+strconv.Itoa(track.Index),
			"-avoid_negative_ts", "disabled", "-c:s", "srt", "-f", "srt", "pipe:1")
		if err != nil {
			return nil, fmt.Errorf("extract embedded subtitle stream %d: %w", track.Index, err)
		}
		cues, err := captionsInWindow(data, start, end, 0)
		if err != nil {
			return nil, fmt.Errorf("read embedded subtitle stream %d: %w", track.Index, err)
		}
		if len(cues) > 0 {
			result.Source, result.Language, result.Cues = "embedded", track.Language, cues
			return result, nil
		}
	}
	if model == "" || len(info.Audio) == 0 {
		return result, nil
	}
	if end > start+120 {
		return nil, fmt.Errorf("Whisper requires a window of at most 120 seconds; select a smaller interval with --start and --end")
	}
	cues, err := transcribeWindow(ctx, tools, info, start, end, out, model, language)
	if err != nil {
		return nil, err
	}
	result.Source, result.Cues = "whisper", cues
	if language != "auto" {
		result.Language = language
	}
	return result, nil
}

func captionTracks(streams []streamInfo, language string) []streamInfo {
	tracks := make([]streamInfo, 0, len(streams))
	for _, track := range streams {
		switch track.Codec {
		case "ass", "eia_608", "jacosub", "microdvd", "mpl2", "mov_text", "pjs", "realtext", "sami", "scc", "ssa", "stl", "subrip", "subviewer", "subviewer1", "text", "vplayer", "webvtt":
			tracks = append(tracks, track)
		}
	}
	if language != "" && language != "auto" {
		sort.SliceStable(tracks, func(i, j int) bool {
			return strings.EqualFold(tracks[i].Language, language) && !strings.EqualFold(tracks[j].Language, language)
		})
	}
	return tracks
}

func transcribeWindow(ctx context.Context, tools toolset, info mediaInfo, start, end float64, out, model, language string) ([]captionCue, error) {
	// Keep PCM and Whisper intermediates out of the result, including after a failed invocation.
	tmp, err := os.MkdirTemp(out, ".asr-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	wav := filepath.Join(tmp, "audio.wav")
	prefix := filepath.Join(tmp, "transcript")
	startSeconds := strconv.FormatFloat(start, 'f', 6, 64)
	endSeconds := strconv.FormatFloat(end, 'f', 6, 64)
	duration := strconv.FormatFloat(end-start, 'f', 6, 64)
	// Preserve gaps between the requested start and a delayed audio stream. Whisper reads this WAV from zero.
	filter := "atrim=start=" + startSeconds + ":end=" + endSeconds + ",asetpts=PTS-" + startSeconds + "/TB,aresample=16000:async=1:first_pts=0"
	_, _, err = tools.run(ctx, "ffmpeg", "-nostdin", "-v", "error", "-copyts", "-start_at_zero", "-ss", startSeconds,
		"-protocol_whitelist", "file,pipe", "-i", info.Input, "-map", "0:"+strconv.Itoa(info.Audio[0].Index),
		"-af", filter, "-t", duration, "-ac", "1", "-c:a", "pcm_s16le", "-f", "wav", wav)
	if err != nil {
		return nil, fmt.Errorf("extract audio for Whisper: %w", err)
	}
	if language == "" {
		language = "auto"
	}
	// Agent processes may lack Metal access even when Whisper can see the GPU.
	_, _, err = tools.run(ctx, "whisper-cli", "-ng", "-m", model, "-f", wav, "-l", language, "-osrt", "-of", prefix)
	if err != nil {
		return nil, fmt.Errorf("transcribe selected audio with Whisper: %w", err)
	}
	data, err := os.ReadFile(prefix + ".srt")
	if err != nil {
		return nil, fmt.Errorf("read Whisper transcript: %w", err)
	}
	return whisperCues(data, start, end)
}

func whisperCues(data []byte, start, end float64) ([]captionCue, error) {
	cues, err := parseCaptions(data, start)
	if err != nil {
		return nil, err
	}
	// Whisper's estimated timestamp can outlast its WAV. It only received audio up to end.
	for i := range cues {
		cues[i].End = min(cues[i].End, end)
	}
	return slices.DeleteFunc(cues, func(cue captionCue) bool { return cue.Start >= cue.End }), nil
}

func captionsInWindow(data []byte, start, end, offset float64) ([]captionCue, error) {
	cues, err := parseCaptions(data, offset)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(cues, func(cue captionCue) bool { return cue.Start < start || cue.End > end }), nil
}

func parseCaptions(data []byte, offset float64) ([]captionCue, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("subtitle content must be UTF-8")
	}
	text := strings.TrimPrefix(string(data), "\ufeff")
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(text, "\n")
	var cues []captionCue
	for i := 0; i < len(lines); {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) != "" {
			j++
		}
		block := lines[i:j]
		lineNumber := i + 1
		i = j
		intro := strings.Fields(block[0])[0]
		if intro == "WEBVTT" || intro == "NOTE" || intro == "STYLE" || intro == "REGION" {
			continue
		}
		if !strings.Contains(block[0], "-->") && len(block) > 1 {
			block = block[1:]
		}
		from, tail, ok := strings.Cut(block[0], "-->")
		fields := strings.Fields(tail)
		if !ok || len(fields) == 0 {
			return nil, fmt.Errorf("invalid subtitle cue at line %d", lineNumber)
		}
		cueStart, startOK := parseCaptionTimestamp(strings.TrimSpace(from), offset)
		cueEnd, endOK := parseCaptionTimestamp(fields[0], offset)
		if !startOK || !endOK || cueEnd <= cueStart {
			return nil, fmt.Errorf("invalid subtitle timing at line %d", lineNumber)
		}
		payload := strings.TrimSpace(html.UnescapeString(captionMarkup.ReplaceAllString(strings.Join(block[1:], "\n"), "")))
		if payload != "" {
			cues = append(cues, captionCue{Start: cueStart, End: cueEnd, Text: payload})
		}
	}
	sort.SliceStable(cues, func(i, j int) bool { return cues[i].Start < cues[j].Start })
	if cues == nil {
		cues = []captionCue{}
	}
	return cues, nil
}

func parseCaptionTimestamp(value string, offset float64) (float64, bool) {
	parts := captionTimestamp.FindStringSubmatch(value)
	if parts == nil {
		return 0, false
	}
	var hours int64
	if parts[1] != "" {
		var err error
		hours, err = strconv.ParseInt(parts[1], 10, 32)
		if err != nil {
			return 0, false
		}
	}
	minutes, _ := strconv.Atoi(parts[2])
	seconds, _ := strconv.Atoi(parts[3])
	milliseconds, _ := strconv.Atoi(parts[4])
	// Add offset on the decimal microsecond grid also used for FFmpeg arguments.
	ticks := int64(offset*1e6+0.5) + hours*3_600_000_000 + int64(minutes*60+seconds)*1_000_000 + int64(milliseconds)*1000
	return float64(ticks) / 1e6, true
}
