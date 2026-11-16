# watch-video

Give a vision-capable model one contact sheet with timestamps, then inspect only the parts that matter. The Agent Skill works with a host that can execute a local command and open JPEGs. You can also run the CLI yourself and attach the resulting images and timestamped text to any model that accepts those inputs.

## Use in an agent

```text
@watch-video <video_url>
@watch-video <video_url> 这个视频说了什么
```

With just the link, the agent gives a timestamped overview. It starts with a compact storyboard and uses the video's content and your question to decide when to inspect a scene, read subtitles, or transcribe speech. Follow-up questions can go straight to the relevant interval using the video and timestamps already in the conversation.

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

The Skill uses this path when speech is needed and usable subtitles are unavailable. CLI users can supply a different compatible model with `--whisper-model`. Whisper runs on the CPU and estimates when each phrase was spoken. If Windows Whisper rejects a model path containing non-ASCII characters, use an ASCII-only model path with the CLI option. [Whisper Windows path issue](https://github.com/ggml-org/whisper.cpp/issues/4018)

## CLI reference

Use the executable path in place of `watch-video` below, or add its `bin` folder to `PATH`. Pass an unused path to `--out` and let the CLI create the directory.

```text
watch-video probe "clip.mp4"
watch-video overview "clip.mp4" --out overview
watch-video focus "clip.mp4" --start 1 --end 3 --frames 3 --out focus
watch-video captions "clip.mp4" --start 1 --end 3 --subtitle "clip.srt" --out captions
watch-video captions "clip.mp4" --batch --whisper-model "model.bin" --out transcript
watch-video fetch "https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4" --out download
```

`overview` produces at most nine tiles in one 960×540 JPEG. `focus` writes individual JPEGs with a long edge up to 1280 pixels. Their JSON files are the time index. Use `--start` and `--end` in seconds on the original video timeline, including for spoiler-free viewing. `captions` can find a same-name SRT or VTT beside the video, or read a sidecar specified with `--subtitle`. If there are no usable cues in that interval, add `--whisper-model FILE` for local transcription.

An individual Whisper window is limited to 120 seconds. When the task needs speech across a longer interval, `--batch` splits it locally in one command. It prints only a file index and cue count. Read `captions.txt` for compact timestamped text and `captions.json` for individual cue boundaries. A supplied range still limits the evidence to the portion requested.

To reuse public captions, pass the requested language to `fetch` with `--subtitle-lang` and use the returned sidecar path as `--subtitle`. The source options are local media or a single public HTTP(S) recording supported by yt-dlp. Some providers may require a local copy. Sampled images can miss brief actions, and reading small text may require a narrow focus pass. Image and text usage depends on the model's input accounting.

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
