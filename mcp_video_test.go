package main

import (
	"bytes"
	"encoding/json"
	"image/jpeg"
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
	content, err := runVideoMCPTool(t.Context(), filepath.Dir(input), "", videoFrameTool, args)
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
