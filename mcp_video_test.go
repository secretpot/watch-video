package main

import (
	"bytes"
	"encoding/json"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestMCPVideoFrameReturnsTimestampAndJPEG(t *testing.T) {
	videoTools(t)
	input := writeVideoFixture(t, "小片.mp4", "-f", "lavfi", "-i", "color=c=blue:s=64x64:r=1:d=18")
	args, err := json.Marshal(map[string]string{"input": input, "at": "00:16", "end": "00:17"})
	if err != nil {
		t.Fatal(err)
	}
	content, err := runVideoMCPTool(t.Context(), filepath.Dir(input), "", videoFrameTool, args, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) != 2 || content[0].Type != "text" || !strings.Contains(content[0].Text, "16.000000 seconds") ||
		content[1].Type != "image" || content[1].MimeType != "image/jpeg" {
		t.Fatalf("unexpected MCP content parts: %d", len(content))
	}
	if config, err := jpeg.DecodeConfig(bytes.NewReader(content[1].Data)); err != nil || config.Width != 64 {
		t.Fatalf("unexpected tool JPEG: %+v, %v", config, err)
	}
}

func TestMCPVideoEvidenceListsSourceChaptersBeforeCaptions(t *testing.T) {
	videoTools(t)
	metadata := filepath.Join(t.TempDir(), "chapters.txt")
	const chapterData = `;FFMETADATA1
[CHAPTER]
TIMEBASE=1/1000
START=0
END=40000
title=开头
[CHAPTER]
TIMEBASE=1/1000
START=40000
END=132000
title=结尾
`
	if err := os.WriteFile(metadata, []byte(chapterData), 0o600); err != nil {
		t.Fatal(err)
	}
	input := writeVideoFixture(t, "chapters.mp4",
		"-f", "lavfi", "-i", "color=c=blue:s=64x64:r=1:d=132",
		"-f", "ffmetadata", "-i", metadata, "-map_chapters", "1")
	subtitle := filepath.Join(filepath.Dir(input), "chapters.srt")
	if err := os.WriteFile(subtitle, []byte("1\n00:00:01,000 --> 00:00:02,000\n开头台词\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	args, err := json.Marshal(map[string]string{"input": input})
	if err != nil {
		t.Fatal(err)
	}
	content, err := runVideoMCPTool(t.Context(), t.TempDir(), "", videoEvidenceTool, args, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) != 2 || content[0].Type != "text" || content[1].Type != "image" {
		t.Fatalf("expected one text and one image, got %+v", content)
	}
	chapters, captions, hasCaptionRef := strings.Cut(content[0].Text, "\nCaption reference:")
	const want = "Source chapter labels and time ranges (publisher metadata)\n" +
		"- 00:00:00.000000–00:00:40.000000  开头\n" +
		"- 00:00:40.000000–00:02:12.000000  结尾"
	if !hasCaptionRef || !strings.HasSuffix(chapters, want) || !strings.Contains(captions, "开头台词") {
		t.Fatalf("unexpected chapter and caption evidence: %q", content[0].Text)
	}
}

func TestChapterLabelIsOneBoundedLine(t *testing.T) {
	const title = "  南\n北\t段\x1b\u202e 落  "
	if got := chapterLabel(title); got != "南 北 段 落" {
		t.Fatalf("unexpected chapter label %q", got)
	}
	long := chapterLabel(strings.Repeat("章", 121))
	if len([]rune(long)) != 120 || !strings.HasSuffix(long, "…") {
		t.Fatalf("chapter label was not bounded: %q", long)
	}
}

func TestMCPVideoCaptionsReadsCompleteCuesFromEarlierEvidence(t *testing.T) {
	videoTools(t)
	input := writeVideoFixture(t, "clip.mp4", "-f", "lavfi", "-i", "color=c=blue:s=64x64:r=2:d=12")
	subtitles := `1
00:00:03,500 --> 00:00:04,100
Crosses start

2
00:00:04,120 --> 00:00:04,500
2022 H100

3
00:00:05,000 --> 00:00:06,000
2023 H200

4
00:00:06,010 --> 00:00:07,000
2024 B200

5
00:00:07,100 --> 00:00:08,000
2025 B300

6
00:00:07,950 --> 00:00:08,500
Crosses end
`
	if err := os.WriteFile(filepath.Join(filepath.Dir(input), "clip.srt"), []byte(subtitles), 0o600); err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	args, err := json.Marshal(map[string]string{"input": input, "start": "00:02", "end": "00:10"})
	if err != nil {
		t.Fatal(err)
	}
	overview, err := runVideoMCPTool(t.Context(), scratch, "", videoEvidenceTool, args, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, referenceLine, found := strings.Cut(overview[0].Text, "\nCaption reference: ")
	reference, _, _ := strings.Cut(referenceLine, "\n")
	if !found || !strings.HasPrefix(reference, "request-") {
		t.Fatalf("overview did not return a caption reference: %q", overview[0].Text)
	}
	call := func(scratch, reference, start, end string) ([]mcpContent, error) {
		t.Helper()
		args, err := json.Marshal(map[string]string{"evidence": reference, "start": start, "end": end})
		if err != nil {
			t.Fatal(err)
		}
		return runVideoMCPTool(t.Context(), scratch, filepath.Join(scratch, "missing-tools"), videoCaptionsTool, args, nil)
	}
	content, err := call(scratch, reference, "00:04", "00:08")
	if err != nil {
		t.Fatal(err)
	}
	if len(content) != 1 || content[0].Type != "text" || len(content[0].Data) != 0 {
		t.Fatalf("caption lookup should only return text: %+v", content)
	}
	const want = "Captions (sidecar), 00:00:04.000000 to 00:00:08.000000:\n" +
		"00:00:04.120000 --> 00:00:04.500000 2022 H100\n" +
		"00:00:05.000000 --> 00:00:06.000000 2023 H200\n" +
		"00:00:06.010000 --> 00:00:07.000000 2024 B200\n" +
		"00:00:07.100000 --> 00:00:08.000000 2025 B300\n"
	if content[0].Text != want {
		t.Fatalf("unexpected caption cues: %q", content[0].Text)
	}
	quoteArgs, err := json.Marshal(map[string]string{"evidence": reference, "start": "00:04", "end": "00:08", "quote": "H100, 2023 H200"})
	if err != nil {
		t.Fatal(err)
	}
	quoteResult, err := runVideoMCPTool(t.Context(), scratch, "", videoCaptionsTool, quoteArgs, nil)
	prefix := "Matched quote: H100 2023 H200\nQuote span: 00:00:04.120000 --> 00:00:06.000000\n"
	if err != nil || len(quoteResult) != 1 || quoteResult[0].Type != "text" || quoteResult[0].Text != prefix+want {
		t.Fatalf("unexpected quote result: %+v, %v", quoteResult, err)
	}
	for _, invalid := range []struct {
		name, scratch, reference, start, end, reason string
	}{
		{"not a reference", scratch, reference + "/../", "00:04", "00:08", "caption reference from video_evidence"},
		{"expired", t.TempDir(), reference, "00:04", "00:08", "expired or has no captions"},
		{"before evidence", scratch, reference, "00:01", "00:08", "caption evidence starts at"},
		{"past cutoff", scratch, reference, "00:04", "00:10.001", "require 0 <= start < end <= 10"},
		{"missing cutoff", scratch, reference, "00:04", "", "start and end are required"},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			_, err := call(invalid.scratch, invalid.reference, invalid.start, invalid.end)
			if err == nil || !strings.Contains(err.Error(), invalid.reason) {
				t.Fatalf("expected %q, got %v", invalid.reason, err)
			}
		})
	}
}

func TestBoundedTranscriptPreservesBothEnds(t *testing.T) {
	start := "00:00:00 开头 "
	end := "01:00:00 结尾 "
	line := strings.Repeat("语", 127)
	input := start + line + "\n" + strings.Repeat("00:40:00 "+line+"\n", 300) + end + line + "\n"
	content, sampled := boundedTranscript(input)
	if !sampled || utf8.RuneCountInString(content) > transcriptBudget || !strings.HasPrefix(content, start) || !strings.Contains(content, end) {
		t.Fatalf("bounded transcript length %d, sampled %v", utf8.RuneCountInString(content), sampled)
	}
}
