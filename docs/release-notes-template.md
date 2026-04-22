## What is OpsWatch

OpsWatch is an incident change witness for live operations: it compares observed changes with operator intent, local context, and safety policy. Start with the [README](../README.md).

## This release

- Packaged macOS menu bar app and CLI artifacts are attached below.
- See the generated changelog for the tag-specific code changes in this release.

## Quick start

1. Download `OpsWatchBar-macos-arm64.zip` for Apple Silicon Macs, or `opswatch-cli-darwin-amd64` / `opswatch-cli-darwin-arm64` for CLI-only installs.
2. Unzip `OpsWatchBar.app` and move it to `/Applications`.
3. Start Ollama and pull a vision model:
   `ollama serve`
   `ollama pull qwen2.5vl:3b-q4_K_M`
4. Open OpsWatch from the menu bar, choose a window, and click `Start Watching`.
5. Keep the log window visible for the first session while tuning model, interval, and context.
