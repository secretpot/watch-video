# watch-video

Give a vision-capable model one contact sheet with timestamps, then inspect only the parts that matter. The Agent Skill works with a host that can execute a local command and open JPEGs. You can also run the CLI yourself and attach the resulting images and timestamped text to any model that accepts those inputs.

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
- **Local speech recognition.** On macOS, run `brew install whisper.cpp`. On Windows, extract `whisper-bin-x64.zip` from the linked upstream build, keep the included DLLs next to `whisper-cli.exe`, and add the executable directory to `PATH`. Download a Whisper model separately, such as the multilingual `ggml-base.bin`, and pass its local path to `--whisper-model`. [whisper.cpp setup](https://github.com/ggml-org/whisper.cpp#quick-start) · [Windows x64 binaries](https://github.com/ggml-org/whisper.cpp/releases/expanded_assets/b5454) · [GGML models](https://huggingface.co/ggerganov/whisper.cpp)

## Use

Use the executable path in place of `watch-video` below, or add its `bin` folder to `PATH`. Use a fresh directory for each `--out`.

```text
watch-video probe "clip.mp4"
watch-video overview "clip.mp4" --out overview
watch-video focus "clip.mp4" --start 1 --end 3 --frames 3 --out focus
watch-video captions "clip.mp4" --start 1 --end 3 --subtitle "clip.srt" --out captions
watch-video fetch "https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4" --out download
```

`overview` produces at most nine tiles in one 960×540 JPEG. `focus` writes individual JPEGs with a long edge up to 1280 pixels. Their JSON files are the time index. Use `--start` and `--end` in seconds on the original video timeline, including for spoiler-free viewing. Quoted dialogue comes from complete cues inside the requested window. `captions` can find a same-name SRT or VTT beside the video, or read a sidecar specified with `--subtitle`. If there are no usable cues in that window, add `--whisper-model FILE` on a window of up to 120 seconds. Transcription runs on the same machine.

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
