package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvidenceIncludesStoryboardAndWindowedCaptions(t *testing.T) {
	videoTools(t)
	input := writeVideoFixture(t, "示例 视频.mp4", "-f", "lavfi", "-i", "color=c=blue:s=64x64:r=1:d=19")
	parent := filepath.Dir(input)
	subtitle := "1\n00:00:15,250 --> 00:00:16,000\n可见\n\n" +
		"2\n00:00:17,000 --> 00:00:18,000\n超出截止\n"
	if err := os.WriteFile(filepath.Join(parent, "示例 视频.srt"), []byte(subtitle), 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(parent, "evidence")
	var stdout bytes.Buffer
	if err := run(t.Context(), []string{"evidence", input, "--start", "00:15", "--end", "00:17", "--out", out}, &stdout); err != nil {
		t.Fatal(err)
	}
	var result evidenceResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Video != input || result.Start != 15 || result.End != 17 || len(result.Images) != 1 || result.Captions.Source != "sidecar" {
		t.Fatalf("unexpected evidence: %+v", result)
	}
	if len(result.Frames) != 2 || result.Images[0].Width != 128 || result.Images[0].Height != 64 {
		t.Fatalf("expected two source-size tiles without the empty rows: %+v", result.renderedVideo)
	}
	for _, frame := range result.Frames {
		if frame.Time < result.Start || frame.Time >= result.End {
			t.Fatalf("frame outside requested window: %+v", frame)
		}
	}
	for _, name := range []string{result.Images[0].Path, result.Captions.Index, "evidence.json"} {
		if info, err := os.Stat(filepath.Join(out, name)); err != nil || info.Size() == 0 {
			t.Fatalf("missing artifact %q: %v", name, err)
		}
	}
	transcript, err := os.ReadFile(filepath.Join(out, result.Captions.Text))
	if err != nil || string(transcript) != "00:00:15 可见\n" {
		t.Fatalf("transcript = %q, %v", transcript, err)
	}
}

func TestEvidenceKeepsImageWhenCaptionsFail(t *testing.T) {
	videoTools(t)
	input := writeVideoFixture(t, "clip.mp4", "-f", "lavfi", "-i", "color=c=blue:s=64x64:r=2:d=1")
	parent := filepath.Dir(input)
	if err := os.WriteFile(filepath.Join(parent, "clip.srt"), []byte("1\n00:00:00,700 --> 00:00:00,200\nBad cue\n"), 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(parent, "evidence")
	var stdout bytes.Buffer
	if err := run(t.Context(), []string{"evidence", input, "--out", out}, &stdout); err != nil {
		t.Fatal(err)
	}
	var result evidenceResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Captions.Status != "error" || !strings.Contains(result.Captions.Error, "invalid subtitle timing") || result.Captions.Text != "" {
		t.Fatalf("unexpected captions: %+v", result.Captions)
	}
	if _, err := os.Stat(filepath.Join(out, "overview.jpg")); err != nil {
		t.Fatal(err)
	}
}

func TestFocusAtReturnsOneFrameAtTheClockTime(t *testing.T) {
	videoTools(t)
	input := writeVideoFixture(t, "clip.mp4", "-f", "lavfi", "-i", "color=c=blue:s=64x64:r=2:d=18")
	var stdout bytes.Buffer
	out := filepath.Join(filepath.Dir(input), "focus")
	if err := run(t.Context(), []string{"focus", input, "--at", "00:16", "--end", "00:00:16.5", "--out", out}, &stdout); err != nil {
		t.Fatal(err)
	}
	var result renderedVideo
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Frames) != 1 || len(result.Images) != 1 || result.Frames[0].Time != 16 || result.End != 16.5 {
		t.Fatalf("unexpected focus: %+v", result)
	}
}
