package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type evidenceOptions struct {
	start, end, subtitle, model, language, subtitleLanguage string
	noASR                                                   bool
}

type evidenceResult struct {
	Video    string  `json:"video"`
	Duration float64 `json:"duration"`
	renderedVideo
	Captions evidenceCaptions `json:"captions"`
}

type evidenceCaptions struct {
	Source string `json:"source"`
	Status string `json:"status,omitempty"`
	Text   string `json:"text,omitempty"`
	Index  string `json:"index,omitempty"`
	Error  string `json:"error,omitempty"`
}

func prepareEvidence(ctx context.Context, tools toolset, input, out string, options evidenceOptions) (evidenceResult, error) {
	var result evidenceResult
	var err error
	if options.model != "" {
		options.model, err = regularFile(options.model)
		if err != nil {
			return result, err
		}
	}
	if options.subtitle != "" {
		options.subtitle, err = regularFile(options.subtitle)
		if err != nil {
			return result, err
		}
	}
	if strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://") {
		download, err := fetchVideo(ctx, tools, input, options.subtitleLanguage, out)
		if err != nil {
			return result, err
		}
		input = filepath.Join(out, download.Video)
	} else {
		input, err = regularFile(input)
		if err != nil {
			return result, err
		}
		for _, tool := range []string{"ffprobe", "ffmpeg"} {
			if _, err := tools.version(ctx, tool); err != nil {
				return result, err
			}
		}
	}
	info, err := probeVideo(ctx, tools, input)
	if err != nil {
		return result, err
	}
	start, end, err := parseWindow(options.start, options.end, info.Duration)
	if err != nil {
		return result, err
	}
	result.Video, result.Duration = input, info.Duration
	result.renderedVideo, err = renderVideo(ctx, tools, info, start, end, out, false, 0)
	if err != nil {
		return result, err
	}
	captions, status, err := evidenceCueData(ctx, tools, info, start, end, out, options)
	if err != nil {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		result.Captions = evidenceCaptions{Source: "none", Status: "error", Error: err.Error()}
		return result, nil
	}
	if captions.Source == "none" {
		result.Captions = evidenceCaptions{Source: "none", Status: status}
		return result, nil
	}
	const index = "captions.json"
	if err := writeJSONFile(filepath.Join(out, index), captions); err != nil {
		return result, err
	}
	if err := writeTranscript(out, captions.Cues); err != nil {
		return result, err
	}
	result.Captions = evidenceCaptions{Source: captions.Source, Text: batchTranscript, Index: index}
	return result, nil
}

func evidenceCueData(ctx context.Context, tools toolset, info mediaInfo, start, end float64, out string, options evidenceOptions) (captionsResult, string, error) {
	subtitle, err := resolveSubtitle(info.Input, options.subtitle)
	if err != nil {
		return captionsResult{}, "", err
	}
	captions, err := collectCaptions(ctx, tools, info, start, end, out, subtitle, "", options.language)
	if err != nil || captions.Source != "none" {
		return captions, "", err
	}
	if len(info.Audio) == 0 {
		return captions, "no_audio", nil
	}
	if options.noASR {
		return captions, "asr_disabled", nil
	}
	model, status, err := evidenceModel(tools, options.model)
	if err != nil || model == "" {
		return captions, status, err
	}
	captions, err = transcribeBatch(ctx, tools, info, captions, out, model, options.language)
	return captions, "no_cues", err
}

func evidenceModel(tools toolset, model string) (string, string, error) {
	if model != "" {
		return model, "", nil
	}
	configDir := os.UserConfigDir
	if runtime.GOOS == "windows" {
		configDir = os.UserCacheDir
	}
	dir, err := configDir()
	if err != nil {
		return "", "model_missing", nil
	}
	model, err = regularFile(filepath.Join(dir, "watch-video", "ggml-base-q5_1.bin"))
	if errors.Is(err, os.ErrNotExist) {
		return "", "model_missing", nil
	}
	if err != nil {
		return "", "", err
	}
	if _, err := tools.path("whisper-cli"); errors.Is(err, exec.ErrNotFound) {
		return "", "whisper_missing", nil
	} else if err != nil {
		return "", "", err
	}
	return model, "", nil
}
