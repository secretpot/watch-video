package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var languageTag = regexp.MustCompile(`^[A-Za-z0-9]+(-[A-Za-z0-9]+)*$`)

type fetchedVideo struct {
	Video              string         `json:"video"`
	Subtitle           string         `json:"subtitle,omitempty"`
	Chapters           []videoChapter `json:"chapters,omitempty"`
	subtitleIncomplete bool
}

type fetchOptions struct {
	subtitleLanguage string
	autoSubtitle     bool
}

type fetchMetadata struct {
	Type       string                       `json:"_type"`
	IsLive     bool                         `json:"is_live"`
	LiveStatus string                       `json:"live_status"`
	Chapters   []videoChapter               `json:"chapters"`
	Language   string                       `json:"language"`
	Subtitles  map[string][]json.RawMessage `json:"subtitles"`
}

func fetchSubtitleLanguage(options fetchOptions, item fetchMetadata) (language string, optional bool) {
	if options.subtitleLanguage != "" {
		return options.subtitleLanguage, false
	}
	if options.autoSubtitle && languageTag.MatchString(item.Language) && len(item.Subtitles[item.Language]) != 0 {
		return item.Language, true
	}
	return "", false
}

func fetchVideo(ctx context.Context, tools toolset, input, out string, options fetchOptions) (fetchedVideo, error) {
	parsed, err := url.Parse(input)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fetchedVideo{}, errors.New("fetch requires a public HTTP(S) URL without credentials")
	}
	if options.subtitleLanguage != "" && !languageTag.MatchString(options.subtitleLanguage) {
		return fetchedVideo{}, errors.New("--subtitle-lang requires one exact language tag, such as en or zh-Hans")
	}
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := tools.version(ctx, tool); err != nil {
			return fetchedVideo{}, err
		}
	}
	ffmpeg, err := tools.path("ffmpeg")
	if err != nil {
		return fetchedVideo{}, err
	}
	common := []string{
		"--ignore-config", "--no-plugin-dirs", "--no-playlist", "--playlist-items", "1",
		"--socket-timeout", "20", "--retries", "2", "--fragment-retries", "2",
		"--ffmpeg-location", filepath.Dir(ffmpeg),
	}
	preflight := append(append([]string{}, common...), "--skip-download", "--flat-playlist", "--dump-single-json", "--", input)
	metadata, _, err := tools.run(ctx, "yt-dlp", preflight...)
	if err != nil {
		return fetchedVideo{}, err
	}
	var item fetchMetadata
	if err := json.Unmarshal(metadata, &item); err != nil {
		return fetchedVideo{}, fmt.Errorf("read yt-dlp metadata: %w", err)
	}
	if (item.Type != "" && item.Type != "video") || item.IsLive ||
		item.LiveStatus == "is_live" || item.LiveStatus == "is_upcoming" || item.LiveStatus == "post_live" {
		return fetchedVideo{}, errors.New("URL must resolve to a single completed recording")
	}

	language, optionalSubtitle := fetchSubtitleLanguage(options, item)
	args := append(common,
		"--match-filter", "!is_live & !is_upcoming",
		"--format", "bv[height<=720]+ba/b[height<=720]/b[height<=?720]/bv[height<=?720]",
		"--concurrent-fragments", "4",
		"--paths", out, "--output", "source.%(ext)s",
		"--print", "after_move:%(filepath)j", "--no-progress",
	)
	if optionalSubtitle {
		// yt-dlp skips an unavailable subtitle and continues downloading the video.
		// A nonzero exit or missing final media path still fails the request.
		args = append(args, "--ignore-errors")
	}
	if language != "" {
		args = append(args, "--write-subs", "--sub-langs", "^"+regexp.QuoteMeta(language)+"$",
			"--sub-format", "vtt/srt/best", "--convert-subs", "srt")
	}
	args = append(args, "--", input)
	stdout, _, err := tools.run(ctx, "yt-dlp", args...)
	if err != nil {
		return fetchedVideo{}, err
	}
	var downloaded string
	if err := json.Unmarshal(stdout, &downloaded); err != nil {
		return fetchedVideo{}, fmt.Errorf("read yt-dlp output path: %w", err)
	}
	path, err := regularFile(downloaded)
	if err != nil {
		return fetchedVideo{}, err
	}
	if filepath.Dir(path) != out || !strings.HasPrefix(filepath.Base(path), "source.") {
		return fetchedVideo{}, errors.New("yt-dlp returned a path outside the expected output")
	}
	subtitleStem := strings.TrimSuffix(path, filepath.Ext(path))
	info, err := probeVideo(ctx, tools, path)
	if err != nil {
		return fetchedVideo{}, err
	}
	height := info.Video.Height
	if info.Video.Rotation%180 != 0 {
		height = info.Video.Width
	}
	if height > 720 {
		// Direct files can lack a format height in yt-dlp's metadata.
		// Keep their audio and normalized timeline when reducing the size.
		reduced := filepath.Join(out, "video.mkv")
		_, _, err := tools.run(ctx, "ffmpeg", "-hide_banner", "-nostdin", "-v", "error",
			"-protocol_whitelist", "file", "-copyts", "-start_at_zero", "-i", path,
			"-map", "0:"+strconv.Itoa(info.Video.Index), "-map", "0:a?", "-map", "0:s?",
			"-vf", "scale=-2:720:reset_sar=1", "-c", "copy", "-c:v", "libx264",
			"-preset", "veryfast", reduced)
		if err != nil {
			return fetchedVideo{}, err
		}
		if err := os.Remove(path); err != nil {
			return fetchedVideo{}, err
		}
		path = reduced
	}
	result := fetchedVideo{Video: filepath.Base(path), Chapters: item.Chapters}
	if language != "" {
		downloadedSubtitle := subtitleStem + "." + language + ".srt"
		if _, err := regularFile(downloadedSubtitle); !errors.Is(err, os.ErrNotExist) {
			if err != nil {
				return fetchedVideo{}, err
			}
			subtitle := strings.TrimSuffix(path, filepath.Ext(path)) + ".srt"
			if err := os.Rename(downloadedSubtitle, subtitle); err != nil {
				return fetchedVideo{}, err
			}
			result.Subtitle = filepath.Base(subtitle)
		}
		result.subtitleIncomplete = result.Subtitle == ""
	}
	return result, nil
}
