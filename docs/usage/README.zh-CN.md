# 使用 watch-video

[English](README.md) · [项目首页](../../README.zh-CN.md) · [安装指南](../setup/INSTALL.zh-CN.md)

## 在 Agent 中使用

用 `@watch-video <视频链接>` 获取带时间码的概览。启用 MCP 时，Agent 调用一次 `video_evidence`，根据分镜和已有字幕回答。也可以使用本地视频路径，或在链接后加一个问题，指定首轮回答的重点。在同一会话追问细节时，Agent 可复用已有证据，回看相关时刻或区间。

当宿主把 `@watch-video` 当作文本传给 Agent 时，Agent 可按名称和说明选择已安装的 Skill。要显式选择 Skill，在 Codex 用 `$watch-video`，在 DeepSeek Harness 的 `/` 菜单中选择 `watch-video`。[Codex Skill 选择](https://developers.openai.com/blog/eval-skills#3-manually-trigger-the-skill-to-expose-hidden-assumptions) · [DeepSeek Harness Skill 选择](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/client/ui-skill/README.md#use-this-package)

## CLI

将下列命令中的 `watch-video` 替换为可执行文件的路径，或把它所在的 `bin` 目录加入 `PATH`。给 `--out` 指定一个尚不存在的目录，CLI 会创建它。目录的上一级路径须已存在。

```text
watch-video setup
watch-video verify-install
watch-video evidence "clip.mp4" --out evidence
watch-video focus "clip.mp4" --at 00:16 --out detail
watch-video probe "clip.mp4"
watch-video overview "clip.mp4" --out overview
watch-video focus "clip.mp4" --start 1 --end 3 --frames 3 --out focus
watch-video captions "clip.mp4" --start 1 --end 3 --subtitle "clip.srt" --out captions
watch-video captions "clip.mp4" --batch --whisper-model "model.bin" --out transcript
watch-video fetch "https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4" --out download
```

### 生成概览

`probe` 查看本地视频的时长、画面、音轨和字幕。对于只需下载为本地文件的公开视频，可用 `fetch`。下载单条公开 HTTP(S) 录播时，原生 HLS 和 DASH 最多并行下载四个分片。

`evidence` 接受本地文件或单条公开录播链接，分镜和字幕并行准备。对于公开链接，当提取器声明了源语言，且提供完全匹配该语言标签的发布方字幕时，会下载这条字幕。用 `--subtitle-lang CODE` 可指定字幕语言标签。提供 `--subtitle` 或使用 `--no-asr` 时，跳过自动字幕下载。分镜省去未使用的网格空位，并保留各采样帧提取时的分辨率。

返回的索引列出本地视频的绝对路径、JPEG 路径、各格时间码和可用的字幕文件。字幕依次使用指定或同目录同名的 SRT/VTT、内嵌文本字幕和本地 Whisper。字幕缺失时，索引给出原因，分镜和已下载的视频仍可使用。`captions.txt` 是适合通读的紧凑版字幕，`captions.json` 保留每条字幕的起止时间。

`overview` 生成一张 960×540 的 JPEG，最多九格。超过两分钟的区间有二到九个首尾相接的源视频章节完整覆盖时，每章取一帧，其余格位从最长的区间补齐，索引同时记录章节范围。其他区间优先从检测到的切点之间取帧，超过两分钟时以每秒最多两帧扫描。

### 回看片段

`focus` 可为一段时间提取 1–6 张 JPEG，每张长边最多 1280 像素。`--at` 返回时间戳大于或等于指定时刻的第一帧。如果提供 `--end`，该帧须早于截止时间。索引记录实际帧时间。所有命令的时间参数支持秒数、`MM:SS` 或 `HH:MM:SS`。区间使用原视频时间轴，包含起点，截止时间不计入。

`captions` 可读取同名或指定的字幕文件，也可用 `--whisper-model FILE` 本地转写。每段 Whisper 转写最多 120 秒。`evidence` 和 `captions --batch` 会在本地切分较长区间。当 Go 可并行使用至少 12 个逻辑 CPU 且模型文件不超过 160 MiB 时，同时处理两个区间，其余情况逐段处理。字幕始终按原视频时间排序。

抽帧可能漏掉短暂动作，可缩小时间区间回看。图片和文本的 token 用量取决于宿主模型。

## MCP 服务

用 `watch-video mcp` 启动本地 stdio 服务。支持转发 MCP 图片结果的宿主，可以从同一次调用收到 JPEG 和带时间码的文本。

- `video_evidence` 为本地文件或公开录播返回一张分镜图和可用字幕，也可指定时间区间。文本中包含分镜路径，能访问该路径的宿主可以直接打开 JPEG。
- `video_frame` 根据已知时间返回单张清晰帧和实际帧时间。
- `video_captions` 用概览返回的字幕引用查询短区间内各条字幕的起止时间，直接读取已有结果。可用 `quote` 匹配原话，时间范围从第一条匹配字幕的起点到最后一条匹配字幕的终点。

字幕过长时，工具会返回带时间码的摘录。在同一个 MCP 会话内可复用临时视频路径和字幕引用。开启新聊天或服务重启后，用原始 URL 调用 `video_evidence` 获取新的路径和引用，服务可复用缓存中的下载内容。

Skill 在 MCP 不可用时会使用 CLI。[Codex 和 DeepSeek Harness 的 MCP 配置](../setup/INSTALL.zh-CN.md#配置-mcp)

通过代码调用 MCP 时，分别转发文本和 JPEG 内容块。序列化整个响应会把 JPEG 变成 base64 文本，可能使证据被截断。Codex 是否要求工具审批由宿主配置决定。

### 公开录播缓存

MCP 服务缓存无需身份验证的公开 HTTP(S) 录播，以及一并下载的字幕和章节数据，从成功写入缓存起保留七天。查找缓存时使用完整 URL（含查询字符串）和字幕下载策略。传入 `subtitle_lang` 或 `no_asr` 可能会匹配到不同的下载缓存。

缓存中的录播、归一化后的时间区间、语言和模型一致时，可复用 Whisper 字幕。查看其他区间时可复用已下载的录播，按需转写该区间。

服务启动时和运行期间每小时清理过期缓存。macOS 默认目录为 `~/Library/Caches/watch-video/public-v1`，Windows 默认目录为 `%LOCALAPPDATA%\watch-video\public-v1`。用 `watch-video mcp --cache-dir DIR` 指定其他缓存目录。
