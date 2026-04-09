# Changelog

## 2026-04-09
- Move the report prompt filename into a config variable so it's easier to customize.
- Integrate the transcribe stage into the pipeline so transcription runs as part of processing.
- Add a --submit-only flag to fire-and-forget birdnet jobs without waiting for results.
- Add a --recurse flag to walk subdirectories when collecting inputs.
- Improve reporting for large sessions with top-N species and hourly bucketing.

## 2026-04-08
- framore CLI: initial v0.1 with pipeline stages
- BirdNet: switch to Ductile API with sync polling and session-unified detections
- Config: interactive wizard plus code GPS support and platform-neutral paths
- Reporting and transcription: Ollama narrative generation with editable prompts and one-shot transcribe command with Whisper config
- Maintenance and docs: initialize beads issue tracking, add README and MIT license, and fix various lint issues

