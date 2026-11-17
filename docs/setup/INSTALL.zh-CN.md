# 安装 watch-video v0.3.0

[English](INSTALL.md) · [项目首页](../../README.zh-CN.md)

在运行 Agent 的机器上安装发布包。压缩包含 CLI 和 `watch-video` Skill。CLI 可以独立使用，能转发图片的本地 stdio MCP 宿主也可以将它作为服务器运行。

## 选择发布包并安装媒体工具

| 宿主系统 | 发布文件 | FFmpeg 安装命令 |
| --- | --- | --- |
| Apple Silicon，macOS 15 或更高版本 | `watch-video-macos-arm64.tar.gz` | `brew install ffmpeg` |
| Windows 11 x64 预览版 | `watch-video-windows-amd64-preview.zip` | `winget install --id Gyan.FFmpeg --exact --source winget` |

在 Apple Silicon Mac 上，`sysctl -n hw.optional.arm64` 返回 `1`。在 Windows 上，`[System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture` 应返回 `X64`。`ffmpeg` 和 `ffprobe` 均需 8.0 或更高版本。安装工具后如果 `PATH` 已更新，请打开新终端。Windows 预览版已在 CI 中通过 Windows Server 2025 x64 验证，Windows 11 桌面验证尚未完成。

若要处理公开视频链接，把官方的 `yt-dlp_macos` 或 `yt-dlp.exe` 放入 `PATH` 中的目录，并确保系统能够通过 `yt-dlp` 命令找到它。YouTube 可能还需要受支持的 JavaScript 运行时。 [yt-dlp 发布](https://github.com/yt-dlp/yt-dlp/releases/latest) · [YouTube 依赖](https://github.com/yt-dlp/yt-dlp/wiki/EJS)

## 校验并安装发布包

从同一个版本下载压缩包和配套的 `.sha256` 文件。在空目录中运行下面的命令。Codex 和 DeepSeek Harness 都能从默认的用户 Skill 目录 `~/.agents/skills` 发现安装好的 Skill。如果宿主使用其他 Skill 目录，请替换为该路径。 [v0.3.0 发布](https://github.com/secretpot/watch-video/releases/tag/v0.3.0) · [Codex Skill 目录](https://learn.chatgpt.com/docs/build-skills#where-codex-loads-local-skills) · [Harness Skill 目录](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/skill/skill-filesystem/README.md)

升级时，检查用户和项目的 Skill 目录中是否已有 `watch-video`，包括旧版 Codex 设置中的 `~/.codex/skills` 和 Harness 的 `~/.dsh/skills`。运行安装命令前，把旧版移出宿主扫描的 Skill 目录。如果继续使用宿主专用的 Skill 目录，请安装到该目录，并将 MCP 配置指向其中的可执行文件。

**macOS**

```sh
asset=watch-video-macos-arm64.tar.gz
release=https://github.com/secretpot/watch-video/releases/download/v0.3.0
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
$release = 'https://github.com/secretpot/watch-video/releases/download/v0.3.0'
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

`doctor` 应输出 `"ready": true`，并列出 `ffmpeg` 和 `ffprobe` 的路径与版本。如果 Agent 无法从当前环境找到媒体工具，用 `watch-video doctor --tools-dir DIR` 指定包含这两个可执行文件的目录绝对路径。下文配置 MCP 服务器参数时也用这一目录。

如果 macOS Gatekeeper 或 Windows SmartScreen 拦截了下载的可执行文件，可以从对应版本标签的源代码构建。 [Gatekeeper](https://support.apple.com/en-us/102445) · [SmartScreen](https://learn.microsoft.com/windows/apps/package-and-deploy/smartscreen-reputation) · [从源码构建](../development/README.zh-CN.md)

## 可选的语音识别

安装 `whisper-cli` 和多语言模型 `ggml-base-q5_1.bin`，即可为缺少可用字幕的视频生成转写。下列命令将模型放到各平台的默认位置。CLI 也支持用 `--whisper-model FILE` 选择其他模型，或者在 Windows 上的 Whisper 无法打开默认路径时指定只含 ASCII 字符的路径。 [模型下载](https://huggingface.co/ggerganov/whisper.cpp/blob/main/ggml-base-q5_1.bin)

**macOS**

```sh
brew install whisper.cpp
model_dir="$HOME/Library/Application Support/watch-video"
mkdir -p "$model_dir"
curl -fL 'https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base-q5_1.bin' \
  -o "$model_dir/ggml-base-q5_1.bin"
```

**Windows PowerShell**

把随包提供的 DLL 和 `whisper-cli.exe` 保存在同一个目录，并将该目录加入 `PATH`。 [Windows x64 可执行文件](https://github.com/ggml-org/whisper.cpp/releases/expanded_assets/b5454)

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

## 配置 MCP

MCP 不可用时，Skill 仍可通过 CLI 工作。要在兼容的宿主上启用 MCP，使用已安装的可执行文件绝对路径作为命令，第一项参数填 `mcp`。如果宿主找不到 FFmpeg，而媒体工具装在 Apple Silicon 上 Homebrew 的默认目录，可将 `args` 设为 `['mcp', '--tools-dir', '/opt/homebrew/bin']`。工具装在其他位置时，请替换为实际目录。

缓存位置和保留期限见[公开录播缓存](../usage/README.zh-CN.md#公开录播缓存)。

### Codex

将下列 TOML 配置表加入 `~/.codex/config.toml`，或加入已配置的 `CODEX_HOME` 下的 `config.toml`。如果已有 `watch_video` 配置，请更新现有条目。把示例命令替换成已安装的可执行文件绝对路径。Windows 上可以使用 TOML 字面量字符串 `'C:\Users\you\.agents\skills\watch-video\bin\watch-video.exe'`。 [Codex MCP 配置](https://learn.chatgpt.com/docs/extend/mcp?surface=cli#configure-with-config-toml)

```toml
[mcp_servers.watch_video]
command = '/Users/you/.agents/skills/watch-video/bin/watch-video'
args = ['mcp']
tool_timeout_sec = 300
```

用 `codex mcp list` 检查配置，也可以在 Codex 终端界面里用 `/mcp` 查看已启用的服务器。打开新会话，通过 `$watch-video` 显式调用 Skill。

### DeepSeek Harness

将下列 YAML 列表项加入 `~/.dsh/profiles/desktop/cordis.patch.yml` 的顶层。使用当前启用的配置档，如果设置了 `DSH_HOME`，请使用对应目录。保留其他条目，已有该服务器时直接更新对应条目。把示例命令替换成已安装的可执行文件绝对路径。Windows 上可以使用 YAML 字面量字符串 `'C:\Users\you\.agents\skills\watch-video\bin\watch-video.exe'`。使用支持视觉输入的模型，并启用附件。 [Harness MCP 配置](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/mcp/mcp-client/README.md)

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

配置档重新加载后，打开新会话，从 `/` 菜单中显式选择 `watch-video`。 [Harness Skill 选择器](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/client/ui-skill/README.md#use-this-package)

## 用视频验证安装

用已安装的可执行文件运行 `watch-video evidence "path/to/clip.mp4" --no-asr --out watch-video-check`，确保输出路径尚未使用。返回的 JSON 应包含分镜 JPEG 的路径。

在新的 Agent 会话中发送 `@watch-video <video_url>`，也可以把链接换成本地视频路径。收到带时间戳的概览后，在同一会话里追问，例如“00:01 时画面里有什么？给出画面的时间戳。”启用 MCP 时，宿主应显示概览调用 `video_evidence`，定位画面的追问调用 `video_frame`。如果概览返回了字幕引用，可用 `video_captions` 查询字幕。
