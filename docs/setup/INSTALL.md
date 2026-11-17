# Install watch-video v0.2.0

[简体中文](INSTALL.zh-CN.md) · [Project home](../../README.md)

Install the release on the machine that runs the agent. The archive includes the CLI and the `watch-video` Skill. The CLI also works on its own, and any local stdio MCP host that forwards images can use it as a server.

## Choose a package and install media tools

| Host OS | Release asset | Required FFmpeg tools |
| --- | --- | --- |
| macOS 15+ on Apple Silicon | `watch-video-macos-arm64.tar.gz` | `brew install ffmpeg` |
| Windows 11 x64 preview | `watch-video-windows-amd64-preview.zip` | `winget install --id Gyan.FFmpeg --exact --source winget` |

On macOS, `sysctl -n hw.optional.arm64` returns `1` on Apple Silicon. On Windows, `[System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture` should report `X64`. Both `ffmpeg` and `ffprobe` must be version 8.0 or later. Open a new terminal after installing tools if `PATH` changed. The Windows preview is exercised on Windows Server 2025 x64 in CI. Windows 11 desktop testing is pending.

For public recording URLs, put the official `yt-dlp_macos` or `yt-dlp.exe` on `PATH` under the command name `yt-dlp`. YouTube can require a supported JavaScript runtime. [yt-dlp releases](https://github.com/yt-dlp/yt-dlp/releases/latest) · [YouTube dependencies](https://github.com/yt-dlp/yt-dlp/wiki/EJS)

## Verify and install the release

Use the archive and its matching `.sha256` from the same release. Run the commands in an empty working directory. Codex and DeepSeek Harness can both discover the installed bundle in the default user skill directory, `~/.agents/skills`. If your host uses a different skills directory, substitute that path. [v0.2.0 release](https://github.com/secretpot/watch-video/releases/tag/v0.2.0) · [Codex skill directories](https://learn.chatgpt.com/docs/build-skills#where-codex-loads-local-skills) · [Harness skill directories](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/skill/skill-filesystem/README.md)

For an upgrade, check for an existing `watch-video` in user and project skill directories, including `~/.codex/skills` on older Codex setups and `~/.dsh/skills` in Harness. Move the prior bundle out of scanned skill directories before running the install commands. If you keep a host-specific skill directory, use it for the destination and point the MCP entry at that binary.

**macOS**

```sh
asset=watch-video-macos-arm64.tar.gz
release=https://github.com/secretpot/watch-video/releases/download/v0.2.0
skills_dir="$HOME/.agents/skills"

curl -fLO "$release/$asset" &&
curl -fLO "$release/$asset.sha256" &&
shasum -a 256 -c "$asset.sha256" &&
test ! -e "$skills_dir/watch-video" &&
tar -xzf "$asset" &&
mkdir -p "$skills_dir" &&
mv watch-video "$skills_dir/watch-video" &&
"$skills_dir/watch-video/bin/watch-video" doctor
```

**Windows PowerShell**

```powershell
$asset = 'watch-video-windows-amd64-preview.zip'
$release = 'https://github.com/secretpot/watch-video/releases/download/v0.2.0'
$skillsDir = Join-Path $HOME '.agents\skills'
$destination = Join-Path $skillsDir 'watch-video'

Invoke-WebRequest "$release/$asset" -OutFile $asset -UseBasicParsing -ErrorAction Stop
Invoke-WebRequest "$release/$asset.sha256" -OutFile "$asset.sha256" -UseBasicParsing -ErrorAction Stop
$expected = ((Get-Content "$asset.sha256" -Raw) -split '\s+')[0]
if ((Get-FileHash $asset -Algorithm SHA256).Hash -ne $expected) { throw 'SHA-256 mismatch' }
if (Test-Path -LiteralPath $destination) { throw "An installation already exists at $destination" }
Expand-Archive -LiteralPath $asset -DestinationPath . -ErrorAction Stop
New-Item -ItemType Directory -Force -Path $skillsDir -ErrorAction Stop | Out-Null
Move-Item -LiteralPath '.\watch-video' -Destination $destination -ErrorAction Stop
& (Join-Path $destination 'bin\watch-video.exe') doctor
```

`doctor` should report `"ready": true` and paths and versions for `ffmpeg` and `ffprobe`. If the agent cannot find installed tools through its environment, run `watch-video doctor --tools-dir DIR` with the absolute directory containing the media executables. Use that same directory in the server arguments below.

If macOS Gatekeeper or Windows SmartScreen flags the downloaded executable, you can build it from the tagged source. [Gatekeeper](https://support.apple.com/en-us/102445) · [SmartScreen](https://learn.microsoft.com/windows/apps/package-and-deploy/smartscreen-reputation) · [Source build](../development/README.md)

## Optional speech recognition

Install `whisper-cli` and the multilingual `ggml-base-q5_1.bin` model to transcribe videos that have no usable subtitles. The commands below place the model at the default location for each OS. The CLI also accepts `--whisper-model FILE` to select another model or an ASCII-only path if Windows Whisper cannot open the default path. [Model download](https://huggingface.co/ggerganov/whisper.cpp/blob/main/ggml-base-q5_1.bin)

**macOS**

```sh
brew install whisper.cpp
model_dir="$HOME/Library/Application Support/watch-video"
mkdir -p "$model_dir"
curl -fL 'https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base-q5_1.bin' \
  -o "$model_dir/ggml-base-q5_1.bin"
```

**Windows PowerShell**

Keep the supplied DLLs next to `whisper-cli.exe` and add its directory to your `PATH`. [Windows x64 binaries](https://github.com/ggml-org/whisper.cpp/releases/expanded_assets/b5454)

```powershell
$whisperDir = Join-Path $HOME 'whisper.cpp-b5454'
Invoke-WebRequest 'https://github.com/ggml-org/whisper.cpp/releases/download/b5454/whisper-bin-x64.zip' `
  -OutFile 'whisper-bin-x64.zip' -UseBasicParsing -ErrorAction Stop
Expand-Archive -LiteralPath 'whisper-bin-x64.zip' -DestinationPath $whisperDir -ErrorAction Stop
Get-ChildItem -LiteralPath $whisperDir -Recurse -File -Filter 'whisper-cli.exe'

$modelDir = Join-Path $env:LOCALAPPDATA 'watch-video'
New-Item -ItemType Directory -Force -Path $modelDir -ErrorAction Stop | Out-Null
Invoke-WebRequest 'https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base-q5_1.bin' `
  -OutFile (Join-Path $modelDir 'ggml-base-q5_1.bin') -UseBasicParsing -ErrorAction Stop
```

## Configure MCP

The Skill works through the CLI when MCP is unavailable. To enable MCP in any compatible host, use the absolute installed executable path as the command and `mcp` as its first argument. If the host cannot find FFmpeg, set `args` to `['mcp', '--tools-dir', '/opt/homebrew/bin']` for a default Apple Silicon Homebrew installation, or substitute the actual media tool directory.

### Codex

Add this table to `~/.codex/config.toml` (or the `config.toml` in your configured `CODEX_HOME`). Update the existing table if `watch_video` is already configured. Replace the example command with your absolute installed executable path. For example, a Windows TOML literal is `'C:\Users\you\.agents\skills\watch-video\bin\watch-video.exe'`. [Codex MCP configuration](https://learn.chatgpt.com/docs/extend/mcp?surface=cli#configure-with-config-toml)

```toml
[mcp_servers.watch_video]
command = '/Users/you/.agents/skills/watch-video/bin/watch-video'
args = ['mcp']
tool_timeout_sec = 300
```

Use `codex mcp list` to check the configuration or `/mcp` in the Codex terminal UI to see active servers. Start a new session and select `$watch-video` to invoke the Skill explicitly.

### DeepSeek Harness

Add this top-level list item to `~/.dsh/profiles/desktop/cordis.patch.yml`. Use the active profile and the `DSH_HOME` directory if configured. Keep existing entries and update the existing server row if present. Replace the example command with your absolute installed executable path. A Windows YAML literal is `'C:\Users\you\.agents\skills\watch-video\bin\watch-video.exe'`. Use a vision-enabled model with attachments enabled. [Harness MCP configuration](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/mcp/mcp-client/README.md)

```yaml
- insert:
    - id: watch-video-mcp
      name: '@deepseek-ai/dsh-mcp-client'
      config:
        serverName: watch_video
        transport: stdio
        command: '/Users/you/.agents/skills/watch-video/bin/watch-video'
        args: ['mcp']
        toolCallTimeoutMs: 300000
```

Start a new session after the profile reloads. Use the `/` menu to select `watch-video` explicitly. [Harness skill selector](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/client/ui-skill/README.md#use-this-package)

## Check a video

Run `watch-video evidence "path/to/clip.mp4" --no-asr --out watch-video-check` with the installed executable and an unused output path. The JSON should point to a storyboard JPEG.

In a new agent session, send `@watch-video <video_url>` or replace the URL with a local video path. After the timestamped overview, send a follow-up in the same chat, such as `What is visible at 00:01? Give the frame timestamp.` With MCP enabled, the host should show `video_evidence` for the overview and `video_frame` for the timed visual follow-up. Use `video_captions` when the overview returns a caption reference.
