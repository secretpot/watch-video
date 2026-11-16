package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCaptionsFilterWebVTTByRequestedWindow(t *testing.T) {
	input := "\ufeffWEBVTT\r\n\r\nNOTE metadata\r\nignore me\r\n\r\n" +
		"00:01.000 --> 00:02.100\r\nbefore the window\r\n\r\n" +
		"spoken-line\r\n00:02.100 --> 00:04.000 align:start\r\n<v Mei>这句 &amp; <b>the second line</b>\r\n多行。\r\n\r\n" +
		"00:03.900 --> 00:04.500\r\nfuture dialogue\r\n"
	got, err := captionsInWindow([]byte(input), 2, 4, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []captionCue{{Start: 2.1, End: 4, Text: "这句 & the second line\n多行。"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cues = %#v; want %#v", got, want)
	}
}

func TestCaptionsShiftSpeechToVideoTimeline(t *testing.T) {
	input := []byte("1\n00:00:00,100 --> 00:00:00,400\nA line\n\n2\n00:00:00,400 --> 00:00:01,100\nOut of scope\n")
	got, err := captionsInWindow(input, 21.1, 22.1, 21.1)
	if err != nil {
		t.Fatal(err)
	}
	want := []captionCue{{Start: 21.2, End: 21.5, Text: "A line"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cues = %#v; want %#v", got, want)
	}
}

func TestCaptionsPreferSidecarToEmbeddedAndSpeech(t *testing.T) {
	dir := t.TempDir()
	subtitle := filepath.Join(dir, "已有 字幕.srt")
	if err := os.WriteFile(subtitle, []byte("1\n00:00:01,000 --> 00:00:02,000\nExisting text\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Neither external tool could read this input. A sidecar must still produce the requested evidence.
	info := mediaInfo{Input: filepath.Join(dir, "missing.mkv"), Audio: []streamInfo{{Index: 1}},
		Subtitles: []streamInfo{{Index: 2, Codec: "subrip"}}}
	value, err := collectCaptions(context.Background(), toolset{}, info, 0, 3, dir, subtitle, "unused-model.bin", "en")
	if err != nil {
		t.Fatal(err)
	}
	got := value.(captionsResult)
	if got.Source != "sidecar" || len(got.Cues) != 1 || got.Cues[0].Text != "Existing text" {
		t.Fatalf("result = %#v", got)
	}
}

func TestCaptionsEmbeddedOnNonzeroContainerTimeline(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is unavailable")
	}
	dir := t.TempDir()
	subtitle := filepath.Join(dir, "内嵌 字幕.srt")
	input := filepath.Join(dir, "带时间偏移 视频.mkv")
	if err := os.WriteFile(subtitle, []byte("1\n00:00:01,250 --> 00:00:02,750\nInside\n\n2\n00:00:02,750 --> 00:00:04,000\nFuture dialogue\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=red:s=64x64:r=5:d=5", "-i", subtitle,
		"-map", "0:v:0", "-map", "1:s:0", "-c:v", "ffv1", "-c:s", "srt", "-output_ts_offset", "7", input)
	if data, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create subtitle fixture: %v\n%s", err, data)
	}
	info := mediaInfo{Input: input, StartTime: 7, Subtitles: []streamInfo{{Index: 1, Codec: "subrip"}}}
	value, err := collectCaptions(context.Background(), toolset{}, info, 1, 3, dir, "", "unused-model.bin", "en")
	if err != nil {
		t.Fatal(err)
	}
	got := value.(captionsResult)
	want := []captionCue{{Start: 1.25, End: 2.75, Text: "Inside"}}
	if got.Source != "embedded" || !reflect.DeepEqual(got.Cues, want) {
		t.Fatalf("result = %#v; want embedded cues %#v", got, want)
	}
}

func TestCaptionsLongASRRequiresFocusedWindow(t *testing.T) {
	info := mediaInfo{Audio: []streamInfo{{Index: 1}}}
	_, err := collectCaptions(context.Background(), toolset{}, info, 21, 142, t.TempDir(), "", "model.bin", "")
	if err == nil || !strings.Contains(err.Error(), "120 seconds") {
		t.Fatalf("error = %v; want ASR duration error", err)
	}
}
