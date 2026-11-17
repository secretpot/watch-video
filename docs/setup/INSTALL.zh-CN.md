# 安装 watch-video v0.4.1

[English](INSTALL.md) · [项目首页](../../README.zh-CN.md)

把发布压缩包交给能运行本地命令的 Agent，说 **「安装 Watch Video」**。只有项目地址时也可让 Agent 下载发布包。请在运行 Agent 的机器上执行安装。[发布包下载](https://github.com/secretpot/watch-video/releases/tag/v0.4.1)

## 默认安装流程

| 宿主系统 | 发布包 |
| --- | --- |
| macOS 15+ Apple Silicon | `watch-video-macos-arm64.tar.gz` |
| Windows 11 x64 预览版 | `watch-video-windows-amd64-preview.zip` |

Apple Silicon 上 `sysctl -n hw.optional.arm64` 返回 `1`。Windows 上 `[System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture` 应返回 `X64`。Windows 发布包已在 Windows Server 2025 x64 上通过 CI，Windows 11 桌面仍待实测。

1. 优先使用用户提供的发布包。如果未附 `.sha256`，从同一发布页下载对应校验文件。只有项目地址时，下载对应平台的包和校验文件。校验后解压，将整个 `watch-video` 目录安装到当前宿主的用户 Skill 目录。Codex 和 DeepSeek Harness 都支持 `~/.agents/skills`。已有宿主专用目录时沿用原路径，升级前先把旧包移出 Skill 的扫描目录。[Codex Skill 目录](https://learn.chatgpt.com/docs/build-skills#where-codex-loads-local-skills) · [Harness Skill 目录](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/skill/skill-filesystem/README.md)
2. 用安装后的可执行文件运行 `setup`。它会准备 FFmpeg 和 ffprobe 8.0 或更高版本、whisper.cpp，以及多语言 Whisper Base q5_1 模型。macOS 使用 Homebrew，缺少 Homebrew 时按下文安装后重试。Windows 将所需程序和配套 DLL 装到当前用户的数据目录。
3. 核对 `setup` 的 JSON 结果。必须同时看到 `"ready": true`、`video.status: "verified"` 和 `speech.status: "verified"`。该验证会在内置样片中解码一帧画面，并从无字幕音轨识别预期句子。按下文用返回的 `tools_dir` 配置 MCP，重载宿主后在新会话核对 `video_evidence`、`video_frame` 和 `video_captions` 可用。

### 校验并解压发布包

在存放发布包和校验文件的目录运行对应命令。请在空目录解压，避免混入旧版本文件。

**macOS**

```sh
shasum -a 256 -c watch-video-macos-arm64.tar.gz.sha256 &&
tar -xzf watch-video-macos-arm64.tar.gz
```

把包放入当前 Skill 目录后，用安装路径运行。

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

把包放入当前 Skill 目录后，用安装路径运行。

```powershell
& (Join-Path $HOME '.agents\skills\watch-video\bin\watch-video.exe') setup
```

如果 macOS Gatekeeper 或 Windows SmartScreen 拦截了下载的程序，可以从发布标签构建。[Gatekeeper](https://support.apple.com/en-us/102445) · [SmartScreen](https://learn.microsoft.com/windows/apps/package-and-deploy/smartscreen-reputation) · [从源码构建](../development/README.zh-CN.md)

### 中国大陆下载与重试

在中国大陆安装，或遇到上游访问失败时，用 `setup --mirror cn` 优先试区域镜像。默认 `setup` 先试上游，再试区域备用源。只想使用上游时，运行 `setup --mirror official`。

- **macOS 工具**可用清华 TUNA 和中科大 USTC 的 Homebrew API 与二进制包镜像。镜像设置只用于安装命令。[清华 TUNA](https://mirrors.tuna.tsinghua.edu.cn/help/homebrew-bottles/) · [中科大 USTC](https://mirrors.ustc.edu.cn/help/homebrew-bottles.html)
- **Windows 工具**可从 FFmpeg 构建发布方 Gyan 的站点下载，也可经第三方 GitHub 发布文件代理获取固定版本。各源的压缩包均按上游固定的 SHA‑256 校验。[FFmpeg Windows 构建](https://ffmpeg.org/download.html#build-windows) · [Gyan 安装包](https://github.com/GyanD/codexffmpeg/releases/expanded_assets/9.0.2) · [Whisper CPU 安装包](https://github.com/ggml-org/whisper.cpp/releases/expanded_assets/b5454) · [GH-Proxy](https://gh-proxy.com/en/docs/github-accelerator/)
- **Whisper 模型**固定使用 Hugging Face 的指定版本，区域备用源为公开、无需登录的 HF-Mirror。文件通过 SHA‑256 校验后才会启用。[上游模型](https://huggingface.co/ggerganov/whisper.cpp/blob/f281eb45af861ab5e5297d23694b7d46e090c02c/ggml-base-q5_1.bin) · [HF-Mirror](https://hf-mirror.com/)

如果没有 Homebrew，按 [Homebrew 官方说明](https://brew.sh/)安装。在中国大陆可在临时目录运行下列清华镜像命令，再重试 `setup --mirror cn`。如系统请求密码或确认安装 Apple 命令行工具，请在系统窗口完成后继续。[清华 Homebrew 镜像说明](https://mirrors.tuna.tsinghua.edu.cn/help/homebrew/)

```sh
git clone --depth=1 https://mirrors.tuna.tsinghua.edu.cn/git/homebrew/install.git brew-install
HOMEBREW_BREW_GIT_REMOTE=https://mirrors.tuna.tsinghua.edu.cn/git/homebrew/brew.git \
HOMEBREW_CORE_GIT_REMOTE=https://mirrors.tuna.tsinghua.edu.cn/git/homebrew/homebrew-core.git \
HOMEBREW_API_DOMAIN=https://mirrors.tuna.tsinghua.edu.cn/homebrew-bottles/api \
HOMEBREW_BOTTLE_DOMAIN=https://mirrors.tuna.tsinghua.edu.cn/homebrew-bottles \
/bin/bash brew-install/install.sh
```

下载或样片验证失败时，明确告诉用户 **本地转写尚未就绪**，注明失败的依赖，并给出带安装路径的重试命令。上游失败时重试 `setup --mirror cn`，只用上游时重试 `setup --mirror official`。已装好的发布包可保留。已有工具和本地模型时，用 `verify-install --tools-dir DIR --whisper-model FILE` 单独验证。MCP 使用默认模型路径，macOS 为 `~/Library/Application Support/watch-video/ggml-base-q5_1.bin`，Windows 为 `%LOCALAPPDATA%\watch-video\ggml-base-q5_1.bin`。 两个模型源都不可达时，可在其他网络下载同一个固定版本，复制到上述路径再运行 `setup` 校验和复用。

Windows 上如果 Whisper 提示缺少运行库 DLL，安装微软签名的 Visual C++ v14 x64 运行库后，再运行 `setup`。系统可能要求管理员确认。如果 Whisper 无法打开非 ASCII 路径里的自定义模型，将该模型放到纯 ASCII 路径，再运行 `verify-install`。[微软运行库说明](https://learn.microsoft.com/zh-cn/cpp/windows/latest-supported-vc-redist) · [x64 安装程序](https://aka.ms/vc14/vc_redist.x64.exe) · [Whisper 路径问题](https://github.com/ggml-org/whisper.cpp/issues/4018)

查看公开链接还需要单独安装官方 `yt-dlp`，并让 Agent 能从 `PATH` 找到它。YouTube 可能还需要兼容的 JavaScript 运行时。[yt-dlp 发布](https://github.com/yt-dlp/yt-dlp/releases/latest) · [YouTube 附加依赖](https://github.com/yt-dlp/yt-dlp/wiki/EJS)

## 配置 MCP

把已安装的可执行文件绝对路径设为本地 stdio 命令，依次传入 `mcp`、`--tools-dir` 和 `setup` 返回的绝对 `tools_dir`。即使桌面宿主采用不同的 `PATH`，也能找到同一套工具。宿主不支持 MCP 时，请在 CLI 命令中使用相同的 `--tools-dir`。

缓存位置和保留期限见[公开录播缓存](../usage/README.zh-CN.md#公开录播缓存)。

### Codex

将下列 TOML 配置表加入 `~/.codex/config.toml`，或加入已配置的 `CODEX_HOME` 下的 `config.toml`。如果已有 `watch_video` 配置，请更新现有条目。将示例中的两处绝对路径换成已安装的可执行文件和工具目录。Windows 上可以使用 TOML 字面量字符串 `'C:\Users\you\.agents\skills\watch-video\bin\watch-video.exe'`。 [Codex MCP 配置](https://learn.chatgpt.com/docs/extend/mcp?surface=cli#configure-with-config-toml)

```toml
[mcp_servers.watch_video]
command = '/Users/you/.agents/skills/watch-video/bin/watch-video'
args = ['mcp', '--tools-dir', '/opt/homebrew/bin']
tool_timeout_sec = 300
```

用 `codex mcp list` 检查配置，也可以在 Codex 终端界面里用 `/mcp` 查看已启用的服务器。打开新会话，通过 `$watch-video` 显式调用 Skill。

### DeepSeek Harness

将下列 YAML 列表项加入 `~/.dsh/profiles/desktop/cordis.patch.yml` 的顶层。使用当前启用的配置档，如果设置了 `DSH_HOME`，请使用对应目录。保留其他条目，已有该服务器时直接更新对应条目。将示例中的两处绝对路径换成已安装的可执行文件和工具目录。Windows 上可以使用 YAML 字面量字符串 `'C:\Users\you\.agents\skills\watch-video\bin\watch-video.exe'`。使用支持视觉输入的模型，并启用附件。 [Harness MCP 配置](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/mcp/mcp-client/README.md)

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

配置档重新加载后，打开新会话，从 `/` 菜单中显式选择 `watch-video`。 [Harness Skill 选择器](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/client/ui-skill/README.md#use-this-package)

## 验证 Agent

在新会话发 `@watch-video <视频链接>`，也可以换成本地视频路径。概览应调用 `video_evidence`。收到概览后，在同一会话问某个时间点的画面可验证 `video_frame`，追问一段台词可验证 `video_captions`。

## 卸载

先移除 MCP 注册并重启宿主，让原先运行的 Watch Video 服务退出。再移除已安装的 `watch-video` Skill 目录，目录里的 `bin/` 也包含 MCP 可执行文件。如果多个宿主共用同一个 Skill 目录，等最后一个宿主停用后再移除这份安装。

### Codex

运行 `codex mcp remove watch_video`，再用 `codex mcp list` 核对。手动编辑时，从生效的 `config.toml` 删除 `[mcp_servers.watch_video]` 及其所有子表，包括工具级设置。默认配置是 `~/.codex/config.toml`。如设置了 `CODEX_HOME`，以其中的配置为准。

移除已安装的 Skill 目录 `~/.agents/skills/watch-video`。如果装在 Codex 专用目录，则移除 `~/.codex/skills/watch-video`。其他安装位置以实际路径为准。

### DeepSeek Harness

从 `~/.dsh/profiles/desktop/cordis.patch.yml` 删除带有 `id: watch-video-mcp` 和 `serverName: watch_video` 的条目。检查曾启用它的其他配置档，例如 `headless`。如设置了 `DSH_HOME`，以其中的配置为准。如果该条目是对应 `insert:` 块里的唯一一项，可一并删除空块。

移除已安装的 Skill 目录 `~/.agents/skills/watch-video`。如果装在 Harness 专用目录，则移除 `~/.dsh/skills/watch-video`。其他安装位置以实际路径为准。

### 其他 Agent 宿主

查看宿主生效的 MCP 配置，找到 `command` 指向 Watch Video 可执行文件且带有 `mcp` 参数的注册条目。确认它属于哪个配置档，以及宿主从哪里读取 `watch-video/SKILL.md`。按宿主的配置方式移除注册，等该 Skill 目录无人共用时，再移除安装目录。

### 清理共享模型和缓存

Codex、DeepSeek Harness 和其他使用这份安装的宿主都停用 Watch Video 后，可以清理共享数据。

- **macOS**。在访达用「前往文件夹」打开 `~/Library/Application Support/watch-video` 和 `~/Library/Caches/watch-video`，分别对应下载的 Whisper 模型和录播缓存。将这两个目录移入废纸篓。
- **Windows**。在文件资源管理器中打开 `%LOCALAPPDATA%\watch-video`，将目录移入回收站。该目录包含下载的模型、缓存和 Watch Video 安装的媒体工具。

如果设置了 `--cache-dir`，也检查该位置的 Watch Video 缓存。其他软件还在使用的 Homebrew 或单独安装的媒体工具可继续保留。
