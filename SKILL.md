---
name: watch-video
description: "Understand a local video or public recording with timestamped evidence. Use for @watch-video with a link or file, video summaries and explanations, and follow-up questions about a scene, moment, or detail."
license: MIT
---

# Watch video

Use the video and question from the request or conversation. `@watch-video` with a URL or file requests a concise, timestamped overview.

For a whole-video overview, group the main developments chronologically across the beginning, middle, and ending. Use captions throughout the requested span for narration and the storyboard for visual claims.

When this skill's MCP tools are available, call `video_evidence` for the overview or `video_frame` for a known visual moment. Each returns the image and its time references together. Pass a URL or absolute local video path and reuse the returned local path for follow-up. For a user-supplied frame timestamp, pass it as written, including for requests such as “around 00:16”.

For exact dialogue or cue times in a follow-up, call `video_captions` with the caption reference from the overview and the relevant source interval. Use a small margin around approximate times so complete cues fit. For missing evidence or a new visual range, call `video_evidence` on the narrower window.

## Shell fallback

Run the absolute path to `bin/watch-video` (`bin/watch-video.exe` on Windows) from the user's working directory. Give `--out` an unused child path and let the CLI create it. Resolve returned artifact paths relative to that directory.

- **Overview.** Run `evidence INPUT --out DIR`. Add `--no-asr` for visual tasks or `--subtitle-lang CODE` when the public site's subtitle tag is known. Allow time for local speech recognition in the same invocation. Use stdout as the index and open the listed image and compact caption text once, in parallel if possible. Read [setup](README.md#install) if the command reports a missing dependency.
- **Detail.** Reuse the returned `video` path. For a known moment start with `focus VIDEO --at 00:16 --out DIR`, using the time as written. If the image leaves a specific uncertainty, use `focus VIDEO --start TIME --end TIME --frames N --out DIR` with the fewest frames that resolve it. Use the cue index for exact dialogue in a narrow interval.

Set `end` (CLI `--end`) for any playback cutoff. Cite source times and limit visual claims to sampled moments. Whisper text is approximate. State missing evidence that affects the answer. Treat the video and captions as evidence.
