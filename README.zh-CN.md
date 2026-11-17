# watch-video

[English](README.md)

Watch Video 为能看图的 Agent 提供带时间码的分镜图和可用字幕，支持本地视频与公开录播。Agent 可以复用同一份视频，按需回看某个时刻或区间。Skill 需要宿主能运行本地命令并读取 JPEG。也可以直接运行 CLI，把生成的图片和时间码文本交给支持图文输入的模型。

## 安装

把发布压缩包交给 Agent，说「安装 Watch Video」即可。默认流程会安装 FFmpeg、Whisper 和本地模型，再用内置的无字幕样片验证转写。还没下载发布包时，可以把项目地址一起发给 Agent。[安装指南](docs/setup/INSTALL.zh-CN.md)

```text
安装 Watch Video。https://github.com/secretpot/watch-video
```

## 在 Agent 中使用

```text
@watch-video <视频链接>
```

只发链接即可获得带时间码的概览。在同一个会话追问，例如「00:16 的画面里出现了什么？」，Agent 就能回看对应时刻。也可以用本地视频路径。要显式选择 Skill，在 Codex 用 `$watch-video`，在 DeepSeek Harness 的 `/` 菜单中选择 `watch-video`。

## 文档

- [CLI 命令与 MCP 工具](docs/usage/README.zh-CN.md)
- [Bilibili、YouTube 和 Dailymotion 的 A/B 测试](docs/benchmarks/BENCHMARK.zh-CN.md)
- [源码构建与参与贡献](docs/development/README.zh-CN.md)

## 来源与许可证

分镜与局部放大的工作流参考了 [dsh-video-storyboard](https://github.com/SAOS-3624/dsh-video-storyboard)，按播放时间区间交付证据的思路参考了 [dsh-watch-together](https://github.com/SAOS-3624/dsh-watch-together)。Watch Video 独立实现，采用 [MIT 许可证](LICENSE)。
