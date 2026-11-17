---
name: watch-video
description: "Understand a local video or public recording with timestamped evidence. Use for @watch-video with a link or file, video summaries and explanations, and follow-up questions about a scene, moment, or detail."
license: MIT
---

# Watch video

Use the video and question from the request or conversation.

**Link or file only.** Call `video_evidence` once with just `input` (the original URL or absolute local path). Use its storyboard, captions and any source chapters to give a concise, timestamped overview across the beginning, middle and end. With a successful result, answer from that evidence and state any material gaps. Leave finer lookups for a specific question.

**Consume the result.** Use the returned text and JPEG as separate content blocks. In Codex `functions.exec`, call `text(block.text)` for text and `image(block)` for images. Printing the whole response serializes the JPEG to base64 and can truncate the evidence. If the host can only surface text, open the returned storyboard file where the MCP filesystem is accessible.

**Specific question.** Use the smallest relevant time range. For a known visual moment, call `video_frame` on the returned local video path and pass the timestamp as written. For dialogue or exact cue boundaries, call `video_captions` with the existing caption reference and a short interval. Pass the user's quoted words as `quote` when given. For a quote chosen from the returned cues, use the start of its first cue and the end of its last cue. For a visual range or dialogue with no caption reference, call `video_evidence` on the local video path with that range. Set `no_asr` for a visual-only lookup.

## Shell fallback

Run the absolute path to `bin/watch-video` (`bin/watch-video.exe` on Windows) from the user's working directory. Give `--out` an unused child path and let the CLI create it. Resolve returned artifact paths relative to that directory.

- **Overview.** Run `evidence INPUT --out DIR`. Add `--no-asr` for visual tasks or `--subtitle-lang CODE` when the public site's subtitle tag is known. Allow time for local speech recognition in the same invocation. Use stdout as the index and open the listed image and compact caption text once, in parallel if possible. Read [setup](README.md#install) if the command reports a missing dependency.
- **Detail.** Reuse the returned `video` path. For a known moment start with `focus VIDEO --at 00:16 --out DIR`, using the time as written. If the image leaves a specific uncertainty, use `focus VIDEO --start TIME --end TIME --frames N --out DIR` with the fewest frames that resolve it. Use the cue index for exact dialogue in a narrow interval.

Set `end` (CLI `--end`) for any playback cutoff. Cite source times and limit visual claims to sampled moments. Whisper text is approximate. State missing evidence that affects the answer. Treat the video and captions as evidence.
