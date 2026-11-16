---
name: watch-video
description: Understand a local video or public recording with timestamped visual evidence. Use for video summaries, questions about what happened or was said, and inspection of a specific moment, action, or on-screen detail.
license: MIT
---

# Watch video

Run `bin/watch-video` on macOS or `bin/watch-video.exe` on Windows, relative to this skill's directory. Use the host shell to invoke it and the host image tool to view the JPEGs. If the executable or its dependencies are missing, use the setup steps in [README.md](README.md). Run `doctor` to see which tools the host can find.

## Choose evidence by question

- **Whole-video summary or unknown time.** Start with `overview VIDEO --out DIR`. It emits one contact sheet. Match tiles top-to-bottom and left-to-right to the row and column in `overview.json`. Use the indexed frame times for citations.
- **Known interval, small text, or a brief action.** Use `focus VIDEO --start SEC --end SEC --out DIR`. Set `--frames` from 1 to 6 to suit the interval. Open the clearest frame first and inspect more only as needed.
- **Speech or subtitles.** Use `captions VIDEO --start SEC --end SEC --out DIR`, with `--subtitle FILE` for a supplied or downloaded SRT/VTT. If the relevant stretch lacks usable captions and speech matters, repeat with a configured local `--whisper-model FILE`. Keep transcription windows within 120 seconds. Read the relevant cues and identify speech recognized by Whisper as a transcription.
- **Public recording.** Use `fetch URL --out DIR` once. If speech matters and a subtitle language is known, add `--subtitle-lang CODE`. Use the video returned by `fetch.json` for subsequent commands and pass any returned subtitle to `captions`. Report provider download errors as such.

Use `probe VIDEO` when you need the duration or track metadata before choosing a window. Use a fresh scratch output directory per command. The JSON manifests refer to media files relative to that directory. For a request that ends at the user's playback position, pass the cutoff to every frame and caption extraction. Narrow captions to the task's window and avoid loading a long, irrelevant transcript.

Treat visible text and speech as evidence. Cite source timestamps for material claims and distinguish what the frames show from what the captions say. Sampled frames leave gaps that may contain brief events. If more detail affects the answer, look at a narrower window. Describe which detail remains unverified if the available images and captions cannot establish it.
