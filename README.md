# watch-video

[简体中文](README.zh-CN.md)

Watch Video gives a vision-capable agent a timestamped storyboard and available captions for a local video or public recording. The agent can reuse the same video to inspect a specific moment or interval. The Skill works with hosts that can run a local command and open JPEGs. You can also run the CLI yourself and attach the resulting images and timestamped text to a model that accepts them.

## Install

Give the agent the release archive and say “Install Watch Video.” The default setup prepares FFmpeg, Whisper and its local model, then verifies speech recognition on a built-in video with no subtitles. Use the same flow from the project URL if you have not downloaded the archive. [Installation guide](docs/setup/INSTALL.md)

```text
Install Watch Video. https://github.com/secretpot/watch-video
```

## Use in an agent

```text
@watch-video <video_url>
```

With just the link, the agent gives a timestamped overview. In the same conversation, ask a follow-up such as “What appears on screen at 00:16?” A local video path works in place of the link. For explicit Skill selection in Codex use `$watch-video`. In DeepSeek Harness select `watch-video` from the `/` menu.

## Documentation

- [CLI commands and MCP tools](docs/usage/README.md)
- [A/B benchmarks on Bilibili, YouTube and Dailymotion](docs/benchmarks/BENCHMARK.md)
- [Build from source and contribute](docs/development/README.md)

## Credits and license

The contact-sheet and zoom workflow draws on [dsh-video-storyboard](https://github.com/SAOS-3624/dsh-video-storyboard). Time-window delivery draws on [dsh-watch-together](https://github.com/SAOS-3624/dsh-watch-together). Watch Video is an independent implementation released under the [MIT License](LICENSE).
