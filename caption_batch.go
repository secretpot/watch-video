package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	batchTranscript        = "captions.txt"
	transcriptBlock        = 15
	whisperParallelMinCPUs = 12
	whisperParallelMaxSize = 160 * 1024 * 1024
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
	var windows []captionWindow
	for from := result.Start; from < result.End; {
		to := min(from+whisperWindowSeconds, result.End)
		windows = append(windows, captionWindow{start: from, end: to})
		from = to
	}
	workers := 1
	if len(windows) > 1 && runtime.GOMAXPROCS(0) >= whisperParallelMinCPUs {
		weights, err := os.Stat(model)
		if err != nil {
			return captionsResult{}, fmt.Errorf("inspect Whisper weights: %w", err)
		}
		// Two four-thread Whisper processes can overlap on this CPU budget.
		// Reserve the extra process for base-size or smaller weights.
		if weights.Size() <= whisperParallelMaxSize {
			workers = 2
		}
	}
	cues, err := transcribeCaptionWindows(ctx, windows, workers, func(ctx context.Context, window captionWindow) ([]captionCue, error) {
		return transcribeWindow(ctx, tools, info, window.start, window.end, dir, model, language)
	})
	if err != nil {
		return captionsResult{}, err
	}
	result.Cues = append(result.Cues, cues...)
	if len(result.Cues) > 0 {
		result.Source = "whisper"
		if language != "auto" {
			result.Language = language
		}
	}
	return result, nil
}

type captionWindow struct {
	start float64
	end   float64
}

func transcribeCaptionWindows(ctx context.Context, windows []captionWindow, workers int, transcribe func(context.Context, captionWindow) ([]captionCue, error)) ([]captionCue, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	jobs := make(chan int)
	batches := make([][]captionCue, len(windows))
	var group sync.WaitGroup
	for range workers {
		group.Go(func() {
			for i := range jobs {
				if ctx.Err() != nil {
					return
				}
				window := windows[i]
				cues, err := transcribe(ctx, window)
				if err != nil {
					cancel(fmt.Errorf("transcribe %.3f–%.3f s: %w", window.start, window.end, err))
					return
				}
				batches[i] = cues
			}
		})
	}
dispatch:
	for i := range windows {
		select {
		case jobs <- i:
		case <-ctx.Done():
			break dispatch
		}
	}
	close(jobs)
	group.Wait()
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	var cues []captionCue
	for _, batch := range batches {
		cues = append(cues, batch...)
	}
	return cues, nil
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
