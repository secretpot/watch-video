package main

import (
	"image/jpeg"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func videoTools(t *testing.T) toolset {
	t.Helper()
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("%s is required for media integration tests", name)
		}
	}
	return toolset{}
}

func writeVideoFixture(t *testing.T, filename string, args ...string) string {
	t.Helper()
	input := filepath.Join(t.TempDir(), filename)
	cmd := exec.Command("ffmpeg", append([]string{"-nostdin", "-hide_banner", "-loglevel", "error"}, append(args, "-c:v", "mpeg4", "-threads:v", "1", input)...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("prepare fixture: %v\n%s", err, output)
	}
	return input
}

func TestFocusUsesSourceTimestampsForOffsetVFR(t *testing.T) {
	tools := videoTools(t)
	ctx := t.Context()
	input := writeVideoFixture(t, "直式 片段.mp4", "-f", "lavfi", "-i", "testsrc2=size=96x160:rate=10:duration=1",
		"-vf", "select='not(mod(n,3))+eq(n,1)',setpts=PTS+8/TB", "-fps_mode", "passthrough", "-video_track_timescale", "10")
	info, err := probeVideo(ctx, tools, input)
	if err != nil {
		t.Fatal(err)
	}
	if info.StartTime != 8 || len(info.Subtitles) != 0 {
		t.Fatalf("unexpected probe: %+v", info)
	}
	out := t.TempDir()
	result, err := renderVideo(ctx, tools, info, 0.08, 0.6, out, true, 6)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Frames) != 2 || math.Abs(result.Frames[0].Time-0.1) > 1e-6 || math.Abs(result.Frames[1].Time-0.3) > 1e-6 {
		t.Fatalf("got %+v, want source frames at .1 and .3 with end .6 excluded", result.Frames)
	}
	if len(result.Images) != 2 {
		t.Fatalf("got %+v, want one JPEG per frame", result.Images)
	}
	file, err := os.Open(filepath.Join(out, result.Images[0].Path))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	config, err := jpeg.DecodeConfig(file)
	if err != nil || config.Width != 96 || config.Height != 160 {
		t.Fatalf("portrait JPEG was not preserved: %+v %v", config, err)
	}
	result, err = renderVideo(ctx, tools, info, 0.59, 0.61, t.TempDir(), true, 1)
	if err != nil {
		t.Fatal(err)
	}
	frames := result.Frames
	if len(frames) != 1 || math.Abs(frames[0].Time-0.6) > 1e-6 {
		t.Fatalf("a frame just inside a fractional cutoff was lost: %+v", frames)
	}
}

func TestProbeNormalizesOffsetMatroskaDuration(t *testing.T) {
	tools := videoTools(t)
	input := writeVideoFixture(t, "offset.mkv", "-f", "lavfi", "-i", "testsrc2=size=64x64:rate=10:duration=2",
		"-output_ts_offset", "7")
	info, err := probeVideo(t.Context(), tools, input)
	if err != nil {
		t.Fatal(err)
	}
	if info.StartTime != 7 || info.Duration != 2 {
		t.Fatalf("got origin %g and duration %g, want origin 7 and duration 2", info.StartTime, info.Duration)
	}
}

func TestOverviewAcceptsSingleFrame(t *testing.T) {
	tools := videoTools(t)
	ctx := t.Context()
	input := writeVideoFixture(t, "one.mp4", "-f", "lavfi", "-i", "color=c=blue:s=64x64:r=25", "-frames:v", "1")
	info, err := probeVideo(ctx, tools, input)
	if err != nil {
		t.Fatal(err)
	}
	result, err := renderVideo(ctx, tools, info, 0, info.Duration, t.TempDir(), false, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Frames) != 1 || result.Frames[0].Time != 0 || result.Frames[0].Row != 1 || result.Frames[0].Column != 1 {
		t.Fatalf("unexpected single frame index: %+v", result.Frames)
	}
	if len(result.Images) != 1 || result.Images[0].Width != 960 || result.Images[0].Height != 540 {
		t.Fatalf("unexpected overview size: %+v", result.Images)
	}
}

func TestOverviewPrefersCutWithinBin(t *testing.T) {
	tools := videoTools(t)
	ctx := t.Context()
	input := writeVideoFixture(t, "cut.mp4", "-f", "lavfi", "-i", "color=c=black:s=64x64:r=10:d=3.1",
		"-f", "lavfi", "-i", "color=c=white:s=64x64:r=10:d=5.9", "-filter_complex", "[0:v][1:v]concat=n=2:v=1:a=0")
	info, err := probeVideo(ctx, tools, input)
	if err != nil {
		t.Fatal(err)
	}
	result, err := renderVideo(ctx, tools, info, 0, info.Duration, t.TempDir(), false, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Frames) != 9 || math.Abs(result.Frames[3].Time-3.1) > 1e-6 {
		t.Fatalf("expected 9 frames with a cut at 3.1 seconds in bin four: %+v", result.Frames)
	}
}

func TestLongOverviewShowsBriefInsertBeforeFirstMidpoint(t *testing.T) {
	tools := videoTools(t)
	ctx := t.Context()
	input := writeVideoFixture(t, "long.mp4",
		"-f", "lavfi", "-i", "color=black:s=64x64:r=2:d=2",
		"-f", "lavfi", "-i", "color=white:s=64x64:r=2:d=4",
		"-f", "lavfi", "-i", "color=black:s=64x64:r=2:d=130",
		"-filter_complex", "[0:v][1:v][2:v]concat=n=3:v=1:a=0")
	info, err := probeVideo(ctx, tools, input)
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	result, err := renderVideo(ctx, tools, info, 0, info.Duration, out, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Frames) != 9 || len(result.Images) != 1 || result.Frames[0].Time != 4 {
		t.Fatalf("expected one scene-aware overview with the insert at 4 s: %+v", result)
	}
	file, err := os.Open(filepath.Join(out, result.Images[0].Path))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	sheet, err := jpeg.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	red, green, blue, _ := sheet.At(160, 90).RGBA()
	if red < 60000 || green < 60000 || blue < 60000 {
		t.Fatal("first tile does not show the white insert between the two cuts")
	}
}
