package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	batchTranscript = "captions.txt"
	transcriptBlock = 15
)

type captionBatchFiles struct {
	Source     string  `json:"source"`
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	Cues       int     `json:"cue_count"`
	Manifest   string  `json:"manifest"`
	Transcript string  `json:"transcript"`
}

func runCaptionBatch(ctx context.Context, tools toolset, info mediaInfo, start, end float64, out, subtitle, model, language string, stdout io.Writer) error {
	var result captionsResult
	err := withOutput(out, "captions", io.Discard, func(dir string) (any, error) {
		// Read existing captions once, before starting any Whisper windows.
		captions, err := collectCaptions(ctx, tools, info, start, end, dir, subtitle, "", language)
		if err != nil {
			return nil, err
		}
		result = captions
		if result.Source == "none" && model != "" && len(info.Audio) > 0 {
			result, err = transcribeBatch(ctx, tools, info, result, dir, model, language)
			if err != nil {
				return nil, err
			}
		}
		if err := writeTranscript(dir, result.Cues); err != nil {
			return nil, err
		}
		return result, nil
	})
	if err != nil {
		return err
	}
	return writeJSON(stdout, captionBatchFiles{
		Source: result.Source, Start: result.Start, End: result.End, Cues: len(result.Cues),
		Manifest: "captions.json", Transcript: batchTranscript,
	})
}

func transcribeBatch(ctx context.Context, tools toolset, info mediaInfo, result captionsResult, dir, model, language string) (captionsResult, error) {
	for from := result.Start; from < result.End; {
		to := min(from+whisperWindowSeconds, result.End)
		cues, err := transcribeWindow(ctx, tools, info, from, to, dir, model, language)
		if err != nil {
			return captionsResult{}, fmt.Errorf("transcribe %.3f–%.3f s: %w", from, to, err)
		}
		result.Cues = append(result.Cues, cues...)
		from = to
	}
	if len(result.Cues) > 0 {
		result.Source = "whisper"
		if language != "auto" {
			result.Language = language
		}
	}
	return result, nil
}

func writeTranscript(dir string, cues []captionCue) error {
	return os.WriteFile(filepath.Join(dir, batchTranscript), []byte(compactTranscript(cues)), 0600)
}

func compactTranscript(cues []captionCue) string {
	var text strings.Builder
	var groupStart float64
	for _, cue := range cues {
		if text.Len() == 0 || cue.Start >= groupStart+transcriptBlock {
			if text.Len() > 0 {
				text.WriteByte('\n')
			}
			groupStart = cue.Start
			seconds := int64(cue.Start)
			fmt.Fprintf(&text, "%02d:%02d:%02d", seconds/3600, seconds/60%60, seconds%60)
		}
		text.WriteByte(' ')
		text.WriteString(strings.Join(strings.Fields(cue.Text), " "))
	}
	if text.Len() > 0 {
		text.WriteByte('\n')
	}
	return text.String()
}
