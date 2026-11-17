package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

const mcpCacheFixtureEnv = "WATCH_VIDEO_TEST_PUBLIC_FIXTURE"
const mcpCacheDisallowFetchEnv = "WATCH_VIDEO_TEST_DISALLOW_FETCH"

func TestMain(m *testing.M) {
	if fixture := os.Getenv(mcpCacheFixtureEnv); fixture != "" {
		if strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe") == "yt-dlp" {
			if err := servePublicVideoFixture(fixture, os.Args[1:]); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		}
		if len(os.Args) > 1 && os.Args[1] == "mcp" {
			main()
			return
		}
	}
	os.Exit(m.Run())
}

// Use the test executable as yt-dlp so the same fixture runs on both release platforms.
func servePublicVideoFixture(fixture string, args []string) error {
	if os.Getenv(mcpCacheDisallowFetchEnv) == "true" {
		return errors.New("unexpected yt-dlp invocation on a warm cache")
	}
	if slices.Contains(args, "--dump-single-json") {
		_, err := fmt.Fprintln(os.Stdout, `{"_type":"video","language":"en","subtitles":{"en":[{"ext":"srt"}],"de":[{"ext":"srt"}]}}`)
		return err
	}
	paths := slices.Index(args, "--paths")
	if paths < 0 || paths+1 == len(args) {
		return fmt.Errorf("expected yt-dlp --paths, got %q", args)
	}
	data, err := os.ReadFile(fixture)
	if err != nil {
		return err
	}
	stem := filepath.Join(args[paths+1], "source")
	if err := os.WriteFile(stem+".mp4", data, 0600); err != nil {
		return err
	}
	if index := slices.Index(args, "--sub-langs"); index >= 0 && index+1 < len(args) {
		track := strings.Trim(args[index+1], "^$")
		text := map[string]string{"en": "Cached English line", "de": "Deutsche Untertitel"}[track]
		if text == "" {
			return fmt.Errorf("unexpected fixture subtitle track %q", track)
		}
		srt := "1\n00:00:01,200 --> 00:00:01,800\n" + text + "\n"
		if err := os.WriteFile(stem+"."+track+".srt", []byte(srt), 0600); err != nil {
			return err
		}
	}
	return json.NewEncoder(os.Stdout).Encode(stem + ".mp4")
}

func TestMCPPublicCacheSurvivesRestart(t *testing.T) {
	videoTools(t)
	fixture := writeVideoFixture(t, "测试 clip.mp4", "-f", "lavfi", "-i", "color=c=blue:s=64x64:r=10:d=4")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	toolsDir := t.TempDir()
	shim := filepath.Join(toolsDir, "yt-dlp")
	if runtime.GOOS == "windows" {
		shim += ".exe"
	}
	if err := os.WriteFile(shim, data, 0700); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(t.TempDir(), "public cache")
	const url = "https://example.test/video"

	cold, stopCold := startMCPCacheProcess(t, executable, toolsDir, cache, fixture, false)
	coldEvidence := cold(videoEvidenceTool, map[string]string{"input": url})
	if len(coldEvidence) != 2 || !strings.Contains(coldEvidence[0].Text, "Cached English line") {
		t.Fatal("cold overview omitted its fixture captions or storyboard")
	}
	stopCold()

	warm, stopWarm := startMCPCacheProcess(t, executable, toolsDir, cache, fixture, true)
	warmEvidence := warm(videoEvidenceTool, map[string]string{"input": url})
	if len(warmEvidence) != 2 || !strings.Contains(warmEvidence[0].Text, "Cached English line") ||
		!bytes.Equal(coldEvidence[1].Data, warmEvidence[1].Data) {
		t.Fatal("restarted server did not reuse the same storyboard source and captions")
	}
	text := warmEvidence[0].Text
	_, captionLine, _ := strings.Cut(text, "\nCaption reference: ")
	captionRef, _, _ := strings.Cut(captionLine, "\n")
	caption := warm(videoCaptionsTool, map[string]string{
		"evidence": captionRef, "start": "00:01", "end": "00:02", "quote": "Cached English line",
	})
	if len(caption) != 1 || !strings.Contains(caption[0].Text, "Quote span: 00:00:01.200000 --> 00:00:01.800000") {
		t.Fatal("restarted server did not return the cached cue's exact boundaries")
	}
	videoLine, _, _ := strings.Cut(text, "\n")
	frame := warm(videoFrameTool, map[string]string{"input": strings.TrimPrefix(videoLine, "Local video: "), "at": "00:01"})
	if len(frame) != 2 || !strings.Contains(frame[0].Text, "1.000000 seconds") {
		t.Fatal("restarted server could not open an exact frame from the cached video")
	}
	if config, err := jpeg.DecodeConfig(bytes.NewReader(frame[1].Data)); err != nil || config.Width != 64 {
		t.Fatalf("invalid frame JPEG: %+v, %v", config, err)
	}
	stopWarm()

	variant, stopVariant := startMCPCacheProcess(t, executable, toolsDir, cache, fixture, false)
	translated := variant(videoEvidenceTool, map[string]string{"input": url, "subtitle_lang": "de"})
	if len(translated) != 2 || !strings.Contains(translated[0].Text, "Deutsche Untertitel") ||
		strings.Contains(translated[0].Text, "Cached English line") {
		t.Fatal("requesting another subtitle language reused the wrong cached track")
	}
	stopVariant()
}

func startMCPCacheProcess(t *testing.T, executable, toolsDir, cache, fixture string, disallowFetch bool) (func(string, map[string]string) []mcpContent, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, executable, "mcp", "--cache-dir", cache, "--tools-dir", toolsDir)
	cmd.Env = append(cmd.Environ(), mcpCacheFixtureEnv+"="+fixture, mcpCacheDisallowFetchEnv+"="+strconv.FormatBool(disallowFetch))
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			cancel()
			_ = cmd.Wait()
		}
	})
	encoder, decoder := json.NewEncoder(stdin), json.NewDecoder(stdout)
	requestID := 0
	request := func(method string, params any) mcpTestReply {
		t.Helper()
		requestID++
		body, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		id := json.RawMessage(strconv.Itoa(requestID))
		if err := encoder.Encode(mcpRequest{JSONRPC: "2.0", ID: id, Method: method, Params: body}); err != nil {
			t.Fatal(err)
		}
		var reply mcpTestReply
		if err := decoder.Decode(&reply); err != nil {
			t.Fatalf("read %s reply: %v", method, err)
		}
		if reply.Error != nil || !bytes.Equal(reply.ID, id) {
			t.Fatalf("unexpected %s reply: %+v", method, reply)
		}
		return reply
	}
	request("initialize", map[string]string{"protocolVersion": mcpProtocolVersion})
	if _, err := fmt.Fprintln(stdin, `{"jsonrpc":"2.0","method":"notifications/initialized"}`); err != nil {
		t.Fatal(err)
	}
	return func(name string, arguments map[string]string) []mcpContent {
			t.Helper()
			reply := request("tools/call", map[string]any{"name": name, "arguments": arguments})
			var result mcpToolResult
			if err := json.Unmarshal(reply.Result, &result); err != nil {
				t.Fatal(err)
			}
			if result.IsError {
				t.Fatalf("%s failed: %s", name, reply.Result)
			}
			return result.Content
		}, func() {
			t.Helper()
			err := errors.Join(stdin.Close(), cmd.Wait())
			stopped = true
			if err != nil {
				t.Fatal(err)
			}
		}
}
