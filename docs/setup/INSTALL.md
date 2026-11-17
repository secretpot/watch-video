# Install watch-video v0.4.0

[简体中文](INSTALL.zh-CN.md) · [Project home](../../README.md)

Give the release archive to an agent with local shell access and say **“Install Watch Video.”** The agent can also start from the project URL. Install on the machine where the agent runs. [Release downloads](https://github.com/secretpot/watch-video/releases/tag/v0.4.0)

## Default installation

| Host OS | Release archive |
| --- | --- |
| macOS 15+ on Apple Silicon | `watch-video-macos-arm64.tar.gz` |
| Windows 11 x64 preview | `watch-video-windows-amd64-preview.zip` |

On Apple Silicon, `sysctl -n hw.optional.arm64` returns `1`. On Windows, `[System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture` should report `X64`. The Windows archive is tested on Windows Server 2025 x64 in CI. Windows 11 desktop testing is pending.

1. Use the archive the user supplied. Get its matching `.sha256` from the same release if it was omitted. With only the project URL, download both files. Verify the checksum and extract the archive. Install the whole `watch-video` directory in the active host’s user Skill location. Both Codex and DeepSeek Harness support `~/.agents/skills`. Reuse an existing host-specific Skill location if the host already has one. For an upgrade, move the old bundle outside scanned Skill directories first. [Codex Skill directories](https://learn.chatgpt.com/docs/build-skills#where-codex-loads-local-skills) · [Harness Skill directories](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/skill/skill-filesystem/README.md)
2. Run the installed executable with `setup`. It prepares FFmpeg and ffprobe 8.0 or later, whisper.cpp, and the multilingual Whisper Base q5_1 model. On macOS it uses Homebrew. If Homebrew is missing, follow the bootstrap steps below, then rerun `setup`. On Windows it installs the required executables and adjacent DLLs in the current user’s data directory.
3. Check the `setup` result. The last JSON report must show `"ready": true`, `video.status: "verified"`, and `speech.status: "verified"`. The check decodes a frame and recognizes a fixed sentence from the bundled video, which contains audio and no subtitles. Use the reported `tools_dir` when configuring MCP below. Reload the host configuration and check that `video_evidence`, `video_frame` and `video_captions` are available in a new session.

### Verify and extract the archive

Run the matching command in the directory containing both downloaded files. Use an empty extraction directory to avoid mixing releases.

**macOS**

```sh
shasum -a 256 -c watch-video-macos-arm64.tar.gz.sha256 &&
tar -xzf watch-video-macos-arm64.tar.gz
```

Once the bundle is in the active Skill directory, run its executable.

```sh
"$HOME/.agents/skills/watch-video/bin/watch-video" setup
```

**Windows PowerShell**

```powershell
$asset = 'watch-video-windows-amd64-preview.zip'
$expected = ((Get-Content "$asset.sha256" -Raw) -split '\s+')[0]
if ((Get-FileHash $asset -Algorithm SHA256).Hash -ne $expected) { throw 'SHA-256 mismatch' }
Expand-Archive -LiteralPath $asset -DestinationPath . -ErrorAction Stop
```

Once the bundle is in the active Skill directory, run its executable.

```powershell
& (Join-Path $HOME '.agents\skills\watch-video\bin\watch-video.exe') setup
```

If macOS Gatekeeper or Windows SmartScreen flags the downloaded executable, you can build from the release tag. [Gatekeeper](https://support.apple.com/en-us/102445) · [SmartScreen](https://learn.microsoft.com/windows/apps/package-and-deploy/smartscreen-reputation) · [Source build](../development/README.md)

### Mainland China downloads and retries

Run `setup --mirror cn` when installing in mainland China or after an upstream connection failure. The default `setup` tries upstreams first, then regional fallbacks. `setup --mirror official` restricts setup to upstream routes.

- **macOS tools** use the TUNA and USTC Homebrew API and bottle mirrors on the regional route. The installer uses these settings for the install command. [TUNA](https://mirrors.tuna.tsinghua.edu.cn/help/homebrew-bottles/) · [USTC](https://mirrors.ustc.edu.cn/help/homebrew-bottles.html)
- **Windows tools** can use the Gyan publisher’s own server for FFmpeg and a GitHub release proxy for the pinned archives if the primary route fails. The GitHub proxy is a third party. The same pinned upstream SHA-256 applies to every route. [FFmpeg Windows builds](https://ffmpeg.org/download.html#build-windows) · [Gyan archives](https://github.com/GyanD/codexffmpeg/releases/expanded_assets/9.0.2) · [Whisper CPU archive](https://github.com/ggml-org/whisper.cpp/releases/expanded_assets/b5454) · [GH-Proxy](https://gh-proxy.com/en/docs/github-accelerator/)
- **The model** uses a pinned Hugging Face revision, with the public, unauthenticated HF-Mirror as a regional route. The downloader verifies SHA‑256 before accepting the file. [Upstream model](https://huggingface.co/ggerganov/whisper.cpp/blob/f281eb45af861ab5e5297d23694b7d46e090c02c/ggml-base-q5_1.bin) · [HF-Mirror](https://hf-mirror.com/)

If Homebrew is missing, use the [Homebrew installer](https://brew.sh/). In mainland China, follow the TUNA bootstrap in a temporary working directory, then retry `setup --mirror cn`. If Apple requests a password or Command Line Tools confirmation, complete that native prompt and resume. [TUNA Homebrew bootstrap](https://mirrors.tuna.tsinghua.edu.cn/help/homebrew/)

```sh
git clone --depth=1 https://mirrors.tuna.tsinghua.edu.cn/git/homebrew/install.git brew-install
HOMEBREW_BREW_GIT_REMOTE=https://mirrors.tuna.tsinghua.edu.cn/git/homebrew/brew.git \
HOMEBREW_CORE_GIT_REMOTE=https://mirrors.tuna.tsinghua.edu.cn/git/homebrew/homebrew-core.git \
HOMEBREW_API_DOMAIN=https://mirrors.tuna.tsinghua.edu.cn/homebrew-bottles/api \
HOMEBREW_BOTTLE_DOMAIN=https://mirrors.tuna.tsinghua.edu.cn/homebrew-bottles \
/bin/bash brew-install/install.sh
```

If a download or the sample check fails, tell the user **local transcription is not ready**, identify the failing component, and give the retry command with the installed executable. Retry `setup --mirror cn` after a failed upstream download or `setup --mirror official` to avoid mirrors. The bundle can stay installed while you retry. If you already have the tools and a local model, run `verify-install --tools-dir DIR --whisper-model FILE` to test them directly. MCP uses the default model at `~/Library/Application Support/watch-video/ggml-base-q5_1.bin` on macOS and `%LOCALAPPDATA%\watch-video\ggml-base-q5_1.bin` on Windows. If both model sources are unreachable, fetch the same pinned file on another network, copy it to that path, and rerun `setup` to verify and reuse it.

On Windows, if Whisper reports a missing runtime DLL, install the Microsoft-signed Visual C++ v14 x64 Redistributable, then rerun `setup`. The installer can request elevation. If you are testing a custom model that Whisper cannot open from a path containing non-ASCII characters, retry `verify-install` with an ASCII-only path to that file. [Microsoft runtime instructions](https://learn.microsoft.com/en-us/cpp/windows/latest-supported-vc-redist) · [x64 installer](https://aka.ms/vc14/vc_redist.x64.exe) · [Whisper path handling](https://github.com/ggml-org/whisper.cpp/issues/4018)

For public URLs, install the official `yt-dlp` separately and make its command available on the agent’s `PATH`. YouTube can require a supported JavaScript runtime. [yt-dlp releases](https://github.com/yt-dlp/yt-dlp/releases/latest) · [YouTube dependencies](https://github.com/yt-dlp/yt-dlp/wiki/EJS)

## Configure MCP

Use the absolute installed executable path as the local stdio command. Pass `mcp`, `--tools-dir` and the absolute `tools_dir` reported by `setup` as three separate arguments. This lets the desktop host find the same tools even with a different `PATH`. On hosts without MCP, use the same `--tools-dir` in CLI commands.

For cache locations and retention, see [Public recording cache](../usage/README.md#public-recording-cache).

### Codex

Add this table to `~/.codex/config.toml` (or the `config.toml` in your configured `CODEX_HOME`). Update the existing table if `watch_video` is already configured. Replace both example paths with your installed executable and the reported tools directory. For example, a Windows TOML literal is `'C:\Users\you\.agents\skills\watch-video\bin\watch-video.exe'`. [Codex MCP configuration](https://learn.chatgpt.com/docs/extend/mcp?surface=cli#configure-with-config-toml)

```toml
[mcp_servers.watch_video]
command = '/Users/you/.agents/skills/watch-video/bin/watch-video'
args = ['mcp', '--tools-dir', '/opt/homebrew/bin']
tool_timeout_sec = 300
```

Use `codex mcp list` to check the configuration or `/mcp` in the Codex terminal UI to see active servers. Start a new session and select `$watch-video` to invoke the Skill explicitly.

### DeepSeek Harness

Add this top-level list item to `~/.dsh/profiles/desktop/cordis.patch.yml`. Use the active profile and the `DSH_HOME` directory if configured. Keep existing entries and update the existing server row if present. Replace both example paths with your installed executable and the reported tools directory. A Windows YAML literal is `'C:\Users\you\.agents\skills\watch-video\bin\watch-video.exe'`. Use a vision-enabled model with attachments enabled. [Harness MCP configuration](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/mcp/mcp-client/README.md)

```yaml
- insert:
    - id: watch-video-mcp
      name: '@deepseek-ai/dsh-mcp-client'
      config:
        serverName: watch_video
        transport: stdio
        command: '/Users/you/.agents/skills/watch-video/bin/watch-video'
        args: ['mcp', '--tools-dir', '/opt/homebrew/bin']
        toolCallTimeoutMs: 300000
```

Start a new session after the profile reloads. Use the `/` menu to select `watch-video` explicitly. [Harness skill selector](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/client/ui-skill/README.md#use-this-package)

## Check the agent

In a new agent session, send `@watch-video <video_url>`, or use a local video path. The overview should use `video_evidence`. In the same chat, ask what is visible at a particular timestamp to check `video_frame`, or ask for a line of dialogue to check `video_captions`.
