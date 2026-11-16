# watch-video

Give a vision-capable model one contact sheet with timestamps, then inspect only the parts that matter. The Agent Skill works with a host that can execute a local command and open JPEGs. You can also run the CLI yourself and attach the resulting images and timestamped text to any model that accepts those inputs.

## Use in an agent

```text
@watch-video <video_url>
@watch-video <video_url> 这个视频说了什么
```

With just the link, the agent gives a timestamped overview. A single CLI call prepares the local video, a compact storyboard, and timestamped text when available. Follow-up questions reuse that video and inspect only the relevant moment or interval.

When the host passes `@watch-video` as text, the installed Skill can be selected from its name and description. For native explicit selection, use `$watch-video` in Codex or select `watch-video` from DeepSeek Harness's `/` menu, followed by the same URL and optional question. [Codex skill selection](https://developers.openai.com/blog/eval-skills#3-manually-trigger-the-skill-to-expose-hidden-assumptions) · [DeepSeek Harness skill selection](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/client/ui-skill/README.md#use-this-package)

## Install

Download the package and its matching `.sha256` file from [GitHub Releases](https://github.com/secretpot/watch-video/releases/latest). The Go executable is included. Install FFmpeg and ffprobe **8.0 or later** for your platform.

| Platform | Package | FFmpeg setup |
| --- | --- | --- |
| macOS 15+ on Apple Silicon | `watch-video-macos-arm64.tar.gz` | `brew install ffmpeg` |
| Windows 11 x86_64 preview | `watch-video-windows-amd64-preview.zip` | `winget install --id Gyan.FFmpeg --exact --source winget` |

On Windows, you can also use a [Gyan portable build](https://www.gyan.dev/ffmpeg/builds/) and add its `bin` directory to `PATH`. Open a new terminal or restart your agent after changing its environment. Windows CI runs on Windows Server 2025 x64. Windows 11 x64 desktop testing is pending.

**macOS**

```sh
shasum -a 256 -c watch-video-macos-arm64.tar.gz.sha256
tar -xzf watch-video-macos-arm64.tar.gz
./watch-video/bin/watch-video doctor
```

**Windows PowerShell**

```powershell
$archive = 'watch-video-windows-amd64-preview.zip'
$expected = ((Get-Content "$archive.sha256" -Raw) -split '\s+')[0]
if ((Get-FileHash $archive -Algorithm SHA256).Hash -ne $expected) { throw 'SHA-256 mismatch' }
Expand-Archive $archive -DestinationPath .
& .\watch-video\bin\watch-video.exe doctor
```

macOS Gatekeeper or Windows SmartScreen may flag a new GitHub download. You can build the executable from the tagged source using the instructions below. [macOS Gatekeeper](https://support.apple.com/en-us/102445) · [Windows SmartScreen](https://learn.microsoft.com/windows/apps/package-and-deploy/smartscreen-reputation)

For a skill-enabled agent, place the extracted `watch-video` folder in its skills directory. If a tool is installed outside the agent's `PATH`, pass `--tools-dir DIR` to the relevant command.

### Optional tools

- **Public recordings.** Put the official `yt-dlp_macos` or Windows `yt-dlp.exe` on `PATH` under the command name `yt-dlp`. Some sites need an additional JavaScript runtime. Support depends on the upstream extractor and the site's ability to deliver an anonymous recording. [yt-dlp release files](https://github.com/yt-dlp/yt-dlp#release-files) · [YouTube setup](https://github.com/yt-dlp/yt-dlp/wiki/EJS)

### Local speech recognition

On macOS, run `brew install whisper.cpp`. On Windows, extract `whisper-bin-x64.zip` from the linked upstream build, keep its DLLs next to `whisper-cli.exe`, and add the executable directory to `PATH`. [whisper.cpp setup](https://github.com/ggml-org/whisper.cpp#quick-start) · [Windows x64 binaries](https://github.com/ggml-org/whisper.cpp/releases/expanded_assets/b5454)

Download the multilingual [ggml-base-q5_1.bin model](https://huggingface.co/ggerganov/whisper.cpp/blob/main/ggml-base-q5_1.bin) once and place it at the default path for your system:

| Platform | Default model path |
| --- | --- |
| macOS | `~/Library/Application Support/watch-video/ggml-base-q5_1.bin` |
| Windows | `%LOCALAPPDATA%\watch-video\ggml-base-q5_1.bin` |

`evidence` checks this path automatically when usable subtitles are unavailable. Pass `--whisper-model FILE` to select a different compatible model or `--no-asr` for visual tasks. Whisper runs on the CPU and estimates when each phrase was spoken. If Windows Whisper rejects a model path containing non-ASCII characters, use an ASCII-only model path with the CLI option. [Whisper Windows path issue](https://github.com/ggml-org/whisper.cpp/issues/4018)

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

`evidence` accepts a local file or a single public HTTP(S) recording. For a known subtitle language on a video platform, pass `--subtitle-lang CODE`. Read the returned index for the absolute local video path, the JPEG path and tile times, and available caption artifacts. Captions use an explicit or adjacent SRT/VTT, then an embedded text track, then local Whisper. If captions are unavailable, the index reports why and retains the storyboard and any downloaded video. The compact `captions.txt` is for an overview and `captions.json` keeps individual cue boundaries.

`overview` produces at most nine tiles in one 960×540 JPEG. `focus` writes JPEGs with a long edge up to 1280 pixels. `--at` takes the first source frame at or after the requested moment, before any `--end` cutoff. The index reports the actual frame time. Use seconds, `MM:SS`, or `HH:MM:SS` for all command time arguments. Ranges are on the original video timeline with an exclusive end.

`captions` can read a same-name or explicitly supplied subtitle and can transcribe with `--whisper-model FILE`. An individual Whisper window is limited to 120 seconds. `evidence` and `captions --batch` split longer requests locally into windows of at most 120 seconds. `fetch` downloads separately when only a reusable local source is needed. Some providers require a local copy. Sampled images can miss brief actions, and image and text token usage depends on the host model.

## Optional MCP server

Start a build from this source with `watch-video mcp` as a local stdio server. A host that forwards images from MCP tool results can receive the JPEG and timestamped text in the same response. `video_evidence` returns one storyboard for a local file or a public recording. `video_frame` returns one detailed frame for a known time. For long transcripts, the tool sends timestamped excerpts across the window and indicates when to request a narrower interval. Reuse the returned local video path within the same server session. The server keeps its temporary downloads until it exits.

In Codex, add the following to your configuration and replace the command with the absolute path to the binary.

```toml
[mcp_servers.watch_video]
command = "/path/to/watch-video"
args = ["mcp"]
tool_timeout_sec = 300
```

In DeepSeek Harness, add this entry to the profile overlay and use a vision-enabled model with attachments enabled.

```yaml
- insert:
    - id: watch-video-mcp
      name: '@deepseek-ai/dsh-mcp-client'
      config:
        serverName: watch_video
        transport: stdio
        command: /path/to/watch-video
        args: [mcp]
        toolCallTimeoutMs: 300000
```

Allow enough tool time for local transcription. Add `--tools-dir` and its directory to the server arguments if the host's PATH does not include FFmpeg. The Skill uses the CLI when the server tools are unavailable. [Codex MCP configuration](https://learn.chatgpt.com/docs/extend/mcp?surface=cli) · [DeepSeek Harness MCP configuration](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/mcp/mcp-client/README.md)

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
