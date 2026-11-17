# watch-video

Give a vision-capable model one contact sheet with timestamps, then inspect only the parts that matter. The Agent Skill works with a host that can execute a local command and open JPEGs. You can also run the CLI yourself and attach the resulting images and timestamped text to any model that accepts those inputs.

## Install

Give your agent the project URL and ask it to install Watch Video. The [installation guide](INSTALL.md) covers choosing the release for your platform, verifying SHA-256, installing media tools, and setting up the Skill and MCP server.

```text
Install and verify Watch Video in this agent. https://github.com/secretpot/watch-video
```

## Use in an agent

```text
@watch-video <video_url>
@watch-video <video_url> 这个视频说了什么
```

With just the link, the agent gives a timestamped overview from a storyboard and available captions. Follow-up questions reuse that evidence and inspect the relevant moment or interval. For example, in the same conversation ask `What appears on screen at 00:16?`

When the host passes `@watch-video` as text, the installed Skill can be selected from its name and description. For native explicit selection, use `$watch-video` in Codex or select `watch-video` from DeepSeek Harness's `/` menu, followed by the same URL and optional question. [Codex skill selection](https://developers.openai.com/blog/eval-skills#3-manually-trigger-the-skill-to-expose-hidden-assumptions) · [DeepSeek Harness skill selection](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/client/ui-skill/README.md#use-this-package)

## Benchmarks

The [three-site A/B results](BENCHMARK.md) report elapsed time, total tokens, and uncached input separately for a timestamped overview and a follow-up on Bilibili, YouTube, and Dailymotion.

## CLI reference

Use the executable path in place of `watch-video` below, or add its `bin` folder to `PATH`. Pass an unused path to `--out` and let the CLI create the directory.

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

`evidence` accepts a local file or a single public HTTP(S) recording. Native HLS and DASH downloads use up to four concurrent fragments. For a public URL, it downloads an authored subtitle when the extractor declares the source language and advertises a track with exactly the same tag. Pass `--subtitle-lang CODE` to select a different track. An explicit `--subtitle` file or `--no-asr` skips automatic subtitle downloads. The storyboard omits unused grid space while retaining the sampled frames at their extracted resolution. Storyboard and caption preparation run independently. Read the returned index for the absolute local video path, the JPEG path and tile times, and available caption artifacts. Captions use an explicit or adjacent SRT/VTT, then an embedded text track, then local Whisper. If captions are unavailable, the index reports why and retains the storyboard and any downloaded video. The compact `captions.txt` is for an overview and `captions.json` keeps individual cue boundaries.

`overview` produces at most nine tiles in one 960×540 JPEG. Scene-based sampling prefers footage between detected cuts and scans at up to two frames per second for ranges over two minutes. `focus` writes JPEGs with a long edge up to 1280 pixels. `--at` takes the first source frame at or after the requested moment, before any `--end` cutoff. The index reports the actual frame time. Use seconds, `MM:SS`, or `HH:MM:SS` for all command time arguments. Ranges are on the original video timeline with an exclusive end.

For long ranges covered by two to nine consecutive source chapters, the overview uses a frame from each chapter and fills the remaining tiles from the longest intervals. The resulting index includes chapter ranges. For other ranges, the overview uses the scene scan.

`captions` can read a same-name or explicitly supplied subtitle and can transcribe with `--whisper-model FILE`. An individual Whisper window is limited to 120 seconds. `evidence` and `captions --batch` split longer requests locally. They process two windows at once when Go can schedule at least 12 CPU threads and the model file is at most 160 MiB, returning cues in source order. Other configurations use one worker. `fetch` downloads separately when only a reusable local source is needed. Some providers require a local copy. Sampled images can miss brief actions, and image and text token usage depends on the host model.

## Optional MCP server

Start the bundled CLI with `watch-video mcp` as a local stdio server. A host that forwards images from MCP tool results can receive the JPEG and timestamped text in the same response. `video_evidence` returns one storyboard for a local file or a public recording. `video_frame` returns one detailed frame for a known time. When the overview has captions, `video_captions` uses its caption reference to return exact cue boundaries in a requested interval without decoding the video or transcribing it again. An optional `quote` aligns matching words to the start of their first caption and the end of their last caption. For long transcripts, the server sends timestamped excerpts and indicates when to request a narrower interval. The local video path and caption reference can be reused while that server keeps its temporary evidence.

See the [MCP configuration examples](INSTALL.md#configure-mcp) for Codex and DeepSeek Harness. The Skill uses the CLI when MCP is unavailable.

## Build and contribute

Use the Go toolchain version in `go.mod` and install the platform's FFmpeg tools. Run `go test ./...`.

```sh
mkdir -p bin
CGO_ENABLED=0 go build -trimpath -o bin/watch-video .
```

```powershell
New-Item -ItemType Directory -Force bin | Out-Null
$env:CGO_ENABLED = '0'
go build -trimpath -o bin/watch-video.exe .
```

Use [Issues](https://github.com/secretpot/watch-video/issues) to report a bug or propose a change. Include your OS, the relevant `doctor` output, and a public or synthetic reproduction.

## Credits and license

The contact-sheet and zoom workflow draws on [dsh-video-storyboard](https://github.com/SAOS-3624/dsh-video-storyboard). Time-window delivery draws on [dsh-watch-together](https://github.com/SAOS-3624/dsh-watch-together). Watch Video is an independent implementation released under the [MIT License](LICENSE).
