# Watch Video v0.2.0 benchmark

In one two-turn run per video, DeepSeek Harness with Watch Video used **66.8–69.5% fewer uncached input tokens** and took **57.3–88.9% less time** across all three videos. Codex used fewer total tokens on all three, while uncached input increased on Dailymotion. The gains measure the agent workflows on these fixtures.

## Setup

- Both arms received the same locally staged video and the same two prompts in a fresh session. The follow-up stayed in the same session. SHA-256 verified identical inputs within each pair. Both YouTube arms also received the same adjacent English SRT. The other two fixtures had no supplied subtitles.
- **A** had the Watch Video Skill and stdio MCP server enabled at [`739a874`](https://github.com/secretpot/watch-video/commit/739a8748e471db8b40fb6af0ad39dd07ec56f818). **B** had neither. Both could use a workspace-write shell and ordinary image tools. B chose how to extract frames and read or transcribe speech with FFmpeg and Whisper. No frame count or processing strategy was imposed on B. This baseline represents an agent assembling its own evidence.
- The six Harness sessions ran serially on the same Apple Silicon Mac using Qwen3.8‑Flash high (`Qwen3.8-Flash-Next-oQ6e-mtp`). Web tools were disabled. The Bilibili control used shell networking to download Whisper weights during the run. The separate Codex sessions used GPT‑6 Luna high with tool networking disabled. The YouTube and Dailymotion Codex runs briefly overlapped. One Codex Bilibili control that opened an image from an earlier run was excluded. The reported pair was serial.
- **Total tokens** include cached and uncached model input and output. **Uncached input** excludes cache reads. Harness requests count metered assistant-generation events in the native session log. Codex requests count distinct token-usage updates. Totals are token counts, with no pricing assumptions. Elapsed time sums the two active turns, including tool work and excluding video staging and the interval between questions.

The videos were acquired from public URLs and staged locally before timing. Separate URL checks on the tested build produced a storyboard for each site. YouTube supplied an authored English track. Dailymotion used local Whisper. Bilibili returned nine chapter ranges with speech recognition disabled for that URL check. Public extraction depends on the upstream site and extractor.

| Site | Recording | Duration |
| --- | --- | ---: |
| YouTube | [Tears of Steel · `R6MlUcmOul8`](https://www.youtube.com/watch?v=R6MlUcmOul8) | 12:14 |
| Dailymotion | [Destination: Moon · `x328iw0`](https://www.dailymotion.com/video/x328iw0) | 08:33 |
| Bilibili | [Commentary on the people's view of history in games · `BV1waH56PE8W`](https://www.bilibili.com/video/BV1waH56PE8W/) | 27:31 |

## Measured results

Each cell shows **A / B**. Time is in seconds.

### DeepSeek Harness · Qwen3.8‑Flash high

| Video | Total tokens | Uncached input | Cached input | Output tokens | Requests | Time |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| YouTube | 130,778 / 2,112,820 | 26,613 / 87,255 | 93,691 / 1,987,359 | 10,474 / 38,206 | 7 / 43 | 135.1 / 1,220.7 |
| Dailymotion | 69,927 / 387,923 | 8,940 / 26,960 | 57,648 / 348,474 | 3,339 / 12,489 | 5 / 18 | 56.9 / 164.2 |
| Bilibili | 143,826 / 1,914,122 | 27,089 / 84,467 | 104,939 / 1,800,337 | 11,798 / 29,318 | 6 / 37 | 215.7 / 505.0 |

### Codex · GPT‑6 Luna high

| Video | Total tokens | Uncached input | Requests | Time |
| --- | ---: | ---: | ---: | ---: |
| YouTube | 82,397 / 280,404 | 18,044 / 49,290 | 5 / 15 | 28.9 / 88.2 |
| Dailymotion | 87,452 / 224,063 | 38,040 / 25,255 | 5 / 13 | 46.6 / 67.0 |
| Bilibili | 155,515 / 186,679 | 18,037 / 30,277 | 8 / 10 | 105.2 / 111.6 |

## Answer checks and limits

- **YouTube.** Both Harness arms cited the mechanical-hand line at the authored cue, 00:30.800–00:34.000. A's overview mislabeled the authored SRT as speech recognition. B also ran redundant ASR and waited on it, contributing to the large time gap. In Codex both arms gave the correct line. A matched the authored cue and B gave a wider interval.
- **Dailymotion.** Both arms in both hosts identified the blue fan of particles rising from a dark crater at 03:19.463 and connected it to the 2009 LCROSS detection of water and other chemicals in a permanently shadowed crater near the Moon's south pole. Narration intervals varied with the Whisper cue boundaries.
- **Bilibili.** Both Harness arms explained why the author says serious wrongdoing depends on an organized chain. A's strict quotation lookup failed after the agent supplied an inexact phrase. It answered from the cached fallback cues, with a broader quotation covering 00:14:30.320–00:14:49.920. B downloaded a larger Whisper model, transcribed the local audio again, and used on-screen text to correct recognition errors. It gave a narrower span of about 00:14:30.2–00:14:38.3. In Codex, A cited the requested opening line at 00:14:30.320–00:14:33.480. B quoted a later example around 00:16:32.
- This is one run per arm on three videos, with spot checks on the overviews and follow-ups. Shell strategy, cache reuse, model output length and speech recognition vary. In particular, the Harness Bilibili pair compares different ASR workflows, and the latency ratios include the model's chosen extra work.

## Exact prompts

### YouTube

1. 请看当前目录里的 clip.webm，给我一份简洁的中文概览，附关键时间码。请根据实际视频内容回答。
2. 回看开头 00:31 左右，Celia 质问 Thom 为什么不承认自己怕她的机械手。请给出这句的英文原文和起止时间码。

### Dailymotion

1. 请看当前目录里的 clip.mp4，给我一份简洁的中文概览，附关键时间码。请根据实际视频内容回答。
2. 回看 03:19.463。画面中央有什么颜色和形状的东西从哪里出现？附近旁白把它关联到哪次任务、在什么地点发现了什么？请分别给出画面时间和相关旁白时间。

### Bilibili

1. 请看当前目录里的 clip.mp4，给我一份简洁的中文概览，附关键时间码。请根据实际视频内容回答。
2. 回看「力量不可撤回」一章开头。作者为什么说现实里的恶通常是高度组织化的？请引用一句原话，并给出这句的起止时间码。
