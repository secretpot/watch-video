package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// The sample is a color card with one sentence synthesized by Flite 2.2 (slt).
// It contains no subtitle stream, so a passing transcript comes from audio.
//
//go:embed fixtures/install-check.mp4
var installSample []byte

type installVerification struct {
	Ready    bool                      `json:"ready"`
	ToolsDir string                    `json:"tools_dir,omitempty"`
	Video    installVideoVerification  `json:"video"`
	Speech   installSpeechVerification `json:"speech"`
}

type installVideoVerification struct {
	Status  string `json:"status"`
	FFmpeg  string `json:"ffmpeg,omitempty"`
	FFprobe string `json:"ffprobe,omitempty"`
	Error   string `json:"error,omitempty"`
}

type installSpeechVerification struct {
	Status     string `json:"status"`
	Whisper    string `json:"whisper,omitempty"`
	Model      string `json:"model,omitempty"`
	Transcript string `json:"transcript,omitempty"`
	Error      string `json:"error,omitempty"`
}

func verifyInstall(ctx context.Context, tools toolset, stdout io.Writer, model string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	report := installVerification{ToolsDir: tools.dir, Speech: installSpeechVerification{Status: "not_run"}}
	var info mediaInfo
	out, err := os.MkdirTemp("", "watch-video-verify-")
	if err == nil {
		defer os.RemoveAll(out)
		info, err = verifyInstalledVideo(ctx, tools, out, &report.Video)
	}
	if err != nil {
		report.Video.Status, report.Video.Error = "error", err.Error()
	} else {
		err = verifyInstalledSpeech(ctx, tools, info, out, model, &report.Speech)
		if err != nil {
			report.Speech.Error = err.Error()
		}
	}
	report.Ready = err == nil
	if writeErr := writeJSON(stdout, report); writeErr != nil {
		return writeErr
	}
	if err != nil {
		return fmt.Errorf("installation verification failed: %w", err)
	}
	return nil
}

func verifyInstalledVideo(ctx context.Context, tools toolset, out string, result *installVideoVerification) (mediaInfo, error) {
	var err error
	result.FFmpeg, err = tools.version(ctx, "ffmpeg")
	if err != nil {
		return mediaInfo{}, err
	}
	result.FFprobe, err = tools.version(ctx, "ffprobe")
	if err != nil {
		return mediaInfo{}, err
	}
	input := filepath.Join(out, "install-check.mp4")
	if err := os.WriteFile(input, installSample, 0600); err != nil {
		return mediaInfo{}, err
	}
	info, err := probeVideo(ctx, tools, input)
	if err != nil {
		return mediaInfo{}, fmt.Errorf("probe bundled video: %w", err)
	}
	if _, err := renderFocusAt(ctx, tools, info, 0, info.Duration, out); err != nil {
		return mediaInfo{}, fmt.Errorf("decode bundled video: %w", err)
	}
	result.Status = "verified"
	return info, nil
}

func verifyInstalledSpeech(ctx context.Context, tools toolset, info mediaInfo, out, model string, result *installSpeechVerification) error {
	result.Status = "error"
	var err error
	if model == "" {
		model, err = defaultWhisperModelPath()
		if err != nil {
			return err
		}
	}
	result.Model, err = regularFile(model)
	if errors.Is(err, os.ErrNotExist) {
		result.Status = "model_missing"
		result.Model = model
	}
	if err != nil {
		return fmt.Errorf("open Whisper model: %w", err)
	}
	result.Whisper, err = tools.path("whisper-cli")
	if errors.Is(err, exec.ErrNotFound) {
		result.Status = "whisper_missing"
	}
	if err != nil {
		return err
	}
	captions, err := collectCaptions(ctx, tools, info, 0, info.Duration, out, "", result.Model, "en")
	if err != nil {
		return err
	}
	var lines []string
	for _, cue := range captions.Cues {
		lines = append(lines, cue.Text)
	}
	result.Transcript = strings.Join(lines, " ")
	words := strings.FieldsFunc(strings.ToLower(result.Transcript), func(r rune) bool { return !unicode.IsLetter(r) })
	transcript := " " + strings.Join(words, " ") + " "
	if captions.Source != "whisper" || !strings.Contains(transcript, " quick brown fox ") || !strings.Contains(transcript, " lazy dog ") {
		return errors.New("local Whisper did not recognize the expected speech in the bundled video")
	}
	result.Status = "verified"
	return nil
}
