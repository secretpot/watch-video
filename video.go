package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	sheetWidth  = 960
	sheetHeight = 540
	sheetSide   = 3
)

type mediaInfo struct {
	Input         string       `json:"input"`
	Duration      float64      `json:"duration"`
	StartTime     float64      `json:"start_time"`
	Video         streamInfo   `json:"video"`
	Audio         []streamInfo `json:"audio"`
	Subtitles     []streamInfo `json:"subtitles"`
	videoTimeBase timeBase
}

type streamInfo struct {
	Index     int    `json:"index"`
	Codec     string `json:"codec"`
	Language  string `json:"language,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	FrameRate string `json:"frame_rate,omitempty"`
	Rotation  int    `json:"rotation,omitempty"`
}

func probeVideo(ctx context.Context, tools toolset, input string) (mediaInfo, error) {
	data, _, err := tools.run(ctx, "ffprobe", "-v", "error", "-protocol_whitelist", "file", "-show_streams", "-show_format", "-of", "json", "-i", input)
	if err != nil {
		return mediaInfo{}, err
	}
	var probe struct {
		Format struct {
			Name     string `json:"format_name"`
			Start    string `json:"start_time"`
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			Index     int    `json:"index"`
			Codec     string `json:"codec_name"`
			Type      string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			FrameRate string `json:"avg_frame_rate"`
			Start     string `json:"start_time"`
			Duration  string `json:"duration"`
			TimeBase  string `json:"time_base"`
			Tags      struct {
				Language string `json:"language"`
			} `json:"tags"`
			Disposition struct {
				AttachedPic int `json:"attached_pic"`
			} `json:"disposition"`
			SideData []struct {
				Rotation int `json:"rotation"`
			} `json:"side_data_list"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return mediaInfo{}, fmt.Errorf("read ffprobe output: %w", err)
	}
	info := mediaInfo{Input: input, Audio: []streamInfo{}, Subtitles: []streamInfo{}}
	info.StartTime, _ = mediaNumber(probe.Format.Start)
	info.Duration, _ = mediaNumber(probe.Format.Duration)
	var videoFound bool
	var knownEnd, allEndsKnown = 0.0, true
	for _, stream := range probe.Streams {
		metadata := streamInfo{
			Index: stream.Index, Codec: stream.Codec, Language: stream.Tags.Language,
			Width: stream.Width, Height: stream.Height, FrameRate: stream.FrameRate,
		}
		switch stream.Type {
		case "video":
			if videoFound || stream.Disposition.AttachedPic != 0 || stream.Width <= 0 || stream.Height <= 0 {
				continue
			}
			top, bottom, found := strings.Cut(stream.TimeBase, "/")
			numerator, numErr := strconv.Atoi(top)
			denominator, denErr := strconv.Atoi(bottom)
			if !found || numErr != nil || denErr != nil || numerator <= 0 || denominator <= 0 {
				return mediaInfo{}, fmt.Errorf("input video has no usable presentation time base")
			}
			info.videoTimeBase = timeBase{numerator, denominator}
			for _, sideData := range stream.SideData {
				if sideData.Rotation != 0 {
					metadata.Rotation = sideData.Rotation
					break
				}
			}
			info.Video, videoFound = metadata, true
		case "audio":
			info.Audio = append(info.Audio, metadata)
		case "subtitle":
			info.Subtitles = append(info.Subtitles, metadata)
			continue
		default:
			continue
		}
		start, startKnown := mediaNumber(stream.Start)
		duration, durationKnown := mediaNumber(stream.Duration)
		if startKnown && durationKnown && duration > 0 {
			knownEnd = math.Max(knownEnd, start-info.StartTime+duration)
		} else {
			allEndsKnown = false
		}
	}
	if !videoFound {
		return mediaInfo{}, fmt.Errorf("input has no video stream")
	}
	// A muxer can include a leading timestamp offset in its reported duration.
	if (allEndsKnown && knownEnd > 0) || info.Duration <= 0 {
		info.Duration = knownEnd
	} else if probe.Format.Name == "matroska,webm" && info.StartTime > 0 && info.Duration > info.StartTime {
		// Matroska stores the segment endpoint from timestamp zero. Frame and
		// subtitle extraction use the first media timestamp as their origin.
		info.Duration -= info.StartTime
	}
	if info.Duration <= 0 {
		return mediaInfo{}, fmt.Errorf("input has no known positive duration")
	}
	return info, nil
}

func mediaNumber(value string) (float64, bool) {
	n, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsInf(n, 0) || math.IsNaN(n) {
		return 0, false
	}
	return n, true
}

type renderedVideo struct {
	Start  float64      `json:"start"`
	End    float64      `json:"end"`
	Images []imageFile  `json:"images"`
	Frames []frameIndex `json:"frames"`
}

type imageFile struct {
	Path   string `json:"path"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type frameIndex struct {
	Time   float64 `json:"time"`
	Image  string  `json:"image"`
	Row    int     `json:"row,omitempty"`
	Column int     `json:"column,omitempty"`
}

const focusSize = 1280

func renderFocusAt(ctx context.Context, tools toolset, info mediaInfo, start, end float64, out string) (renderedVideo, error) {
	result := renderedVideo{Start: start, End: end}
	frame, pts, err := extractFrame(ctx, tools, info, start, end, focusSize, focusSize)
	if err != nil {
		return renderedVideo{}, err
	}
	if frame == nil || pts < start || pts >= end {
		return renderedVideo{}, fmt.Errorf("no video frame at or after %.3f s before cutoff %.3f s", start, end)
	}
	if err := result.addFocusFrame(out, frame, pts); err != nil {
		return renderedVideo{}, err
	}
	return result, nil
}

func (result *renderedVideo) addFocusFrame(out string, frame image.Image, pts float64) error {
	name := fmt.Sprintf("frame-%02d.jpg", len(result.Frames)+1)
	if err := writeJPEG(filepath.Join(out, name), frame); err != nil {
		return err
	}
	result.Frames = append(result.Frames, frameIndex{Time: pts, Image: name})
	result.Images = append(result.Images, imageFile{Path: name, Width: frame.Bounds().Dx(), Height: frame.Bounds().Dy()})
	return nil
}

func renderVideo(ctx context.Context, tools toolset, info mediaInfo, start, end float64, out string, focus bool, frames int) (renderedVideo, error) {
	width, height := sheetWidth/sheetSide, sheetHeight/sheetSide
	var cuts []float64
	if focus {
		width, height = focusSize, focusSize
	} else {
		frames = sheetSide * sheetSide
		if end-start <= 120 {
			var err error
			cuts, err = sceneCuts(ctx, tools, info, start, end)
			if err != nil {
				return renderedVideo{}, err
			}
		}
	}
	result := renderedVideo{Start: start, End: end}
	var sheet *image.RGBA
	if !focus {
		sheet = image.NewRGBA(image.Rect(0, 0, sheetWidth, sheetHeight))
	}
	lastTime := math.Inf(-1)
	for i := range frames {
		binStart := start + (end-start)*float64(i)/float64(frames)
		binEnd := start + (end-start)*float64(i+1)/float64(frames)
		if i == frames-1 {
			binEnd = end
		}
		target := sampleTarget(cuts, binStart, binEnd)
		frame, pts, err := extractFrame(ctx, tools, info, target, binEnd, width, height)
		if err != nil {
			return renderedVideo{}, err
		}
		if frame == nil && target > binStart {
			// A short or variable-rate bin can contain a frame only before its midpoint.
			frame, pts, err = extractFrame(ctx, tools, info, binStart, binEnd, width, height)
			if err != nil {
				return renderedVideo{}, err
			}
		}
		if frame == nil || pts < start || pts >= end || pts <= lastTime {
			continue
		}
		lastTime = pts
		if focus {
			if err := result.addFocusFrame(out, frame, pts); err != nil {
				return renderedVideo{}, err
			}
			continue
		}
		index := len(result.Frames)
		row, column := index/sheetSide, index%sheetSide
		x := column*width + (width-frame.Bounds().Dx())/2
		y := row*height + (height-frame.Bounds().Dy())/2
		dst := image.Rect(x, y, x+frame.Bounds().Dx(), y+frame.Bounds().Dy())
		draw.Draw(sheet, dst, frame, frame.Bounds().Min, draw.Src)
		result.Frames = append(result.Frames, frameIndex{Time: pts, Image: "overview.jpg", Row: row + 1, Column: column + 1})
	}
	if len(result.Frames) == 0 {
		return renderedVideo{}, fmt.Errorf("no video frames have presentation times in [%.3f, %.3f)", start, end)
	}
	if !focus {
		if err := writeJPEG(filepath.Join(out, "overview.jpg"), sheet); err != nil {
			return renderedVideo{}, err
		}
		result.Images = []imageFile{{Path: "overview.jpg", Width: sheetWidth, Height: sheetHeight}}
	}
	return result, nil
}

func sampleTarget(cuts []float64, start, end float64) float64 {
	mid := start + (end-start)/2
	best, distance := mid, math.Inf(1)
	for _, cut := range cuts {
		if cut >= start && cut < end && math.Abs(cut-mid) < distance {
			best, distance = cut, math.Abs(cut-mid)
		}
	}
	return best
}

func videoDecodeArgs(info mediaInfo, start float64) []string {
	// Seek to the preceding microsecond so that fractional time-base ticks stay eligible.
	seek := strconv.FormatFloat(math.Floor(start*1e6)/1e6, 'f', 6, 64)
	return []string{
		"-hide_banner", "-nostdin", "-loglevel", "info", "-filter_threads", "1",
		"-copyts", "-start_at_zero", "-ss", seek, "-noaccurate_seek", "-protocol_whitelist", "file", "-i", info.Input,
		"-map", fmt.Sprintf("0:%d", info.Video.Index), "-an", "-sn", "-dn",
	}
}

type timeBase struct {
	numerator, denominator int
}

func (base timeBase) tickAtOrAfter(seconds float64) int64 {
	tick := math.Floor(seconds * float64(base.denominator) / float64(base.numerator))
	if tick*float64(base.numerator)/float64(base.denominator) < seconds {
		tick++
	}
	return int64(tick)
}

func trimVideo(base timeBase, start, end float64) string {
	// Duration arguments to trim round to the nearest tick. Use ceiling ticks to
	// preserve the half-open window even when the source has a coarse time base.
	return fmt.Sprintf("settb=%d/%d,trim=start_pts=%d:end_pts=%d", base.numerator, base.denominator,
		base.tickAtOrAfter(start), base.tickAtOrAfter(end))
}

func sceneCuts(ctx context.Context, tools toolset, info mediaInfo, start, end float64) ([]float64, error) {
	args := videoDecodeArgs(info, start)
	filter := trimVideo(info.videoTimeBase, start, end) + ",scale=320:180:force_original_aspect_ratio=decrease:reset_sar=1,scdet=sc_pass=1,showinfo=checksum=0"
	_, stderr, err := tools.run(ctx, "ffmpeg", append(args, "-vf", filter, "-fps_mode", "passthrough", "-f", "null", "-")...)
	if err != nil {
		return nil, err
	}
	return frameTimes(stderr)
}

func extractFrame(ctx context.Context, tools toolset, info mediaInfo, start, end float64, width, height int) (image.Image, float64, error) {
	args := videoDecodeArgs(info, start)
	// Fit by display aspect and avoid enlarging source pixels for a focus frame.
	scale := fmt.Sprintf("scale=w='max(1,min(iw*sar,%d))':h='min(ih,%d)':force_original_aspect_ratio=decrease:reset_sar=1", width, height)
	filter := trimVideo(info.videoTimeBase, start, end) + ",showinfo=checksum=0," + scale
	data, stderr, err := tools.run(ctx, "ffmpeg", append(args,
		"-vf", filter, "-frames:v", "1", "-fps_mode", "passthrough",
		"-c:v", "png", "-pix_fmt", "rgb24", "-threads:v", "1", "-f", "image2pipe", "pipe:1")...)
	if err != nil {
		return nil, 0, err
	}
	if len(data) == 0 {
		return nil, 0, nil
	}
	times, err := frameTimes(stderr)
	if err != nil {
		return nil, 0, err
	}
	if len(times) == 0 {
		return nil, 0, fmt.Errorf("ffmpeg did not report the decoded frame timestamp")
	}
	frame, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, fmt.Errorf("decode extracted frame: %w", err)
	}
	return frame, times[0], nil
}

var (
	frameTimeBase = regexp.MustCompile(`config in time_base:\s*(\d+)/(\d+)`)
	framePTS      = regexp.MustCompile(`\bpts:\s*(-?\d+)\s+pts_time:`)
)

func frameTimes(stderr []byte) ([]float64, error) {
	var times []float64
	var numerator, denominator float64
	for line := range strings.SplitSeq(string(stderr), "\n") {
		if !strings.HasPrefix(line, "[Parsed_showinfo_") {
			continue
		}
		if fields := frameTimeBase.FindStringSubmatch(line); fields != nil {
			numerator, _ = strconv.ParseFloat(fields[1], 64)
			denominator, _ = strconv.ParseFloat(fields[2], 64)
		}
		if fields := framePTS.FindStringSubmatch(line); fields != nil {
			pts, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil || numerator == 0 || denominator == 0 {
				return nil, fmt.Errorf("ffmpeg reported an invalid presentation timestamp or time base")
			}
			times = append(times, float64(pts)*numerator/denominator)
		}
	}
	return times, nil
}

func writeJPEG(path string, frame image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	encodeErr := jpeg.Encode(file, frame, &jpeg.Options{Quality: 87})
	closeErr := file.Close()
	if encodeErr != nil {
		return encodeErr
	}
	return closeErr
}
