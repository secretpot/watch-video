# Use watch-video

[Project README](../../README.md) · [简体中文](README.zh-CN.md)

## Agent usage

Send `@watch-video <video_url>` for a timestamped overview. Use a local video path in place of the URL, or append a question to focus the first answer. Ask follow-ups in the same conversation so the agent can reuse the local video and inspect the relevant moment or interval.

When a host passes `@watch-video` as text, the agent can select the installed Skill by name and description. To select it explicitly, use `$watch-video` in Codex or select `watch-video` from DeepSeek Harness's `/` menu. [Codex skill selection](https://developers.openai.com/blog/eval-skills#3-manually-trigger-the-skill-to-expose-hidden-assumptions) · [DeepSeek Harness skill selection](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/client/ui-skill/README.md#use-this-package)

## CLI commands

Use the executable path in place of `watch-video` below, or add its `bin` folder to `PATH`. Pass an unused path inside an existing parent directory to `--out` and let the CLI create it.

```text
watch-video evidence "clip.mp4" --out evidence
watch-video focus "clip.mp4" --at 00:16 --out detail
watch-video probe "clip.mp4"
watch-video overview "clip.mp4" --out overview
watch-video focus "clip.mp4" --start 1 --end 3 --frames 3 --out focus
watch-video captions "clip.mp4" --start 1 --end 3 --subtitle "clip.srt" --out captions
watch-video captions "clip.mp4" --batch --whisper-model "model.bin" --out transcript
watch-video fetch "https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4" --out download
```

Time arguments accept seconds, `MM:SS`, or `HH:MM:SS`. Ranges use the original video timeline with an exclusive end.

### Prepare an overview

`probe` reports a local video's duration and its video, audio and subtitle tracks.

`evidence` accepts a local file or a single public HTTP(S) recording. It prepares a compact storyboard and captions independently. The index gives the absolute local video path, the JPEG path and tile times, and available caption artifacts. The storyboard crops unused grid space while keeping the sampled frames at their extracted resolution. Use `captions.txt` for an overview and `captions.json` for individual cue boundaries. If captions are unavailable, the index reports why and retains the storyboard and any downloaded video.

`overview` writes at most nine tiles in one 960×540 JPEG. When two to nine consecutive source chapters cover a requested range longer than two minutes, it samples each chapter, fills remaining tiles from the longest intervals, and includes chapter ranges in the index. Otherwise it prefers footage between detected cuts, scanning at up to two frames per second for ranges over two minutes.

### Inspect a frame or interval

`focus` writes up to six JPEGs with a long edge up to 1280 pixels. `--at` takes the first source frame at or after the requested moment, before any `--end` cutoff. The index reports the actual frame time. Use `--start`, `--end` and `--frames` to inspect a short interval. Sampled images can miss brief actions, and image and text token usage depends on the host model.

### Captions and downloads

`evidence` and `captions` look for an explicit or adjacent SRT/VTT, then an embedded text track. `evidence` can use a configured local Whisper model when subtitles are unavailable. For `captions`, pass `--whisper-model FILE` to transcribe. A single Whisper window is limited to 120 seconds. `evidence` and `captions --batch` split longer requests locally. They process two windows at once when Go can schedule at least 12 CPU threads and the model file is at most 160 MiB, returning cues in source order. Other configurations use one worker.

For public URLs, `evidence` automatically downloads an authored subtitle when the extractor advertises a track whose language tag exactly matches the declared source language. An explicit `--subtitle` file or `--no-asr` skips automatic subtitle downloads. Pass `--subtitle-lang CODE` to request a particular track explicitly.

`fetch` downloads a reusable local source separately. Native HLS and DASH downloads use up to four concurrent fragments.

## Optional MCP server

Start the bundled CLI with `watch-video mcp` as a local stdio server. A host that forwards images from MCP tool results can receive the JPEG and timestamped text in the same response.

- `video_evidence` returns one storyboard for a local file or public recording along with available captions. For long transcripts, it sends timestamped excerpts and indicates when to request a narrower interval.
- `video_frame` returns one detailed frame for a known time.
- `video_captions` uses the overview's caption reference to return exact cue boundaries for a requested interval without decoding or transcribing the video again. An optional `quote` aligns matching words to the start of their first caption and the end of their last caption.

Reuse the temporary video path and caption reference within the same MCP session. In a new chat or after the server restarts, call `video_evidence` with the original URL to get fresh references. The server can reuse the cached download.

The Skill uses the CLI when MCP is unavailable. [MCP configuration for Codex and DeepSeek Harness](../setup/INSTALL.md#configure-mcp)

### Public recording cache

The MCP server keeps downloads of public HTTP(S) recordings that require no authentication, along with downloaded subtitles and chapter data, for seven days from the time the download is stored successfully. Cache lookup uses the exact URL, including its query string, and the subtitle download policy. Passing `subtitle_lang` or `no_asr` can select a different download.

Whisper captions are reused for the same cached recording, normalized time window, language and model. A different time window can reuse the download and transcribe that window as needed.

The server prunes expired cache entries on startup and hourly while running. The default directory is `~/Library/Caches/watch-video/public-v1` on macOS and `%LOCALAPPDATA%\watch-video\public-v1` on Windows. Use `watch-video mcp --cache-dir DIR` to select a different cache directory.
