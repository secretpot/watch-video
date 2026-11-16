package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCaptionBatchWritesTextAndKeepsCueManifest(t *testing.T) {
	input := writeVideoFixture(t, "短片.mp4", "-f", "lavfi", "-i", "color=blue:s=64x64:r=1:d=20", "-c:v", "libx264")
	base := filepath.Dir(input)
	subtitle := filepath.Join(base, "短片.srt")
	srt := "1\n00:00:01,200 --> 00:00:02,100\nFirst\n\n" +
		"2\n00:00:05,000 --> 00:00:06,000\n同一段\n\n" +
		"3\n00:00:16,300 --> 00:00:17,500\nSecond\n"
	if err := os.WriteFile(subtitle, []byte(srt), 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(base, "batch")
	var stdout bytes.Buffer
	if err := run(context.Background(), []string{"captions", "--batch", input, "--out", out}, &stdout); err != nil {
		t.Fatal(err)
	}
	var files captionBatchFiles
	if err := json.Unmarshal(stdout.Bytes(), &files); err != nil {
		t.Fatal(err)
	}
	if files.Source != "sidecar" || files.Cues != 3 || files.Transcript != "captions.txt" || files.Manifest != "captions.json" {
		t.Fatalf("batch index = %+v", files)
	}
	if bytes.Contains(stdout.Bytes(), []byte("同一段")) {
		t.Fatal("batch printed caption text to stdout")
	}
	text, err := os.ReadFile(filepath.Join(out, files.Transcript))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(text), "00:00:01 First 同一段\n00:00:16 Second\n"; got != want {
		t.Fatalf("transcript = %q; want %q", got, want)
	}
	manifest, err := os.ReadFile(filepath.Join(out, files.Manifest))
	if err != nil {
		t.Fatal(err)
	}
	var captions captionsResult
	if err := json.Unmarshal(manifest, &captions); err != nil {
		t.Fatal(err)
	}
	if len(captions.Cues) != 3 || captions.Cues[2].Start != 16.3 || captions.Cues[2].End != 17.5 {
		t.Fatalf("cue manifest = %+v", captions)
	}
}

func TestParallelCaptionWindowsMergeInSourceOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	windows := []captionWindow{{0, 120}, {120, 240}, {240, 241}}
	thirdStarted := make(chan struct{})
	cues, err := transcribeCaptionWindows(ctx, windows, 2, func(ctx context.Context, window captionWindow) ([]captionCue, error) {
		switch window.start {
		case 0:
			// The first interval finishes only after a worker has completed
			// the second and advanced to the third.
			select {
			case <-thirdStarted:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		case 240:
			close(thirdStarted)
		}
		return []captionCue{{Start: window.start, End: window.end}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cues) != len(windows) {
		t.Fatalf("got %d cues, want %d", len(cues), len(windows))
	}
	for i, cue := range cues {
		if cue.Start != windows[i].start || cue.End != windows[i].end {
			t.Fatalf("cue %d = %+v, want window %+v", i, cue, windows[i])
		}
	}
}

func TestParallelCaptionWindowsCancelAndJoinAfterError(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	windows := []captionWindow{{0, 120}, {120, 240}}
	siblingStarted := make(chan struct{})
	siblingStopped := make(chan struct{})
	failure := errors.New("Whisper failed")
	_, err := transcribeCaptionWindows(ctx, windows, 2, func(ctx context.Context, window captionWindow) ([]captionCue, error) {
		if window.start == 0 {
			select {
			case <-siblingStarted:
				return nil, failure
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		close(siblingStarted)
		<-ctx.Done()
		close(siblingStopped)
		return nil, ctx.Err()
	})
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "transcribe 0.000–120.000 s") {
		t.Fatalf("got %v, want first interval error", err)
	}
	select {
	case <-siblingStopped:
	default:
		t.Fatal("sibling still running after return")
	}
}
