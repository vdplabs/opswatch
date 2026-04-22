# OpsWatchBar

OpsWatchBar is the macOS menu bar companion for OpsWatch.

It lists visible windows, lets you pick one, and starts the Go watcher in the background with `--window-id`.

## Getting Started

Start Ollama and pull a local vision model:

```bash
ollama serve
ollama pull qwen2.5vl:3b-q4_K_M
```

For a release install, download `OpsWatchBar-macos-arm64.zip`, unzip it, and move `OpsWatchBar.app` to `/Applications`. The release app includes the `opswatch` CLI and does not require Go or a source checkout.

For local development, launch the menu bar app:

```bash
cd /Users/vishal/go/src/github.com/vdplabs/opswatch/macos/OpsWatchBar
swift build
swift run OpsWatchBar
```

`swift build` ensures the local `OpsWatchOCR` helper is available for the native OCR fast path.

Then:

1. Click `OpsWatch` in the menu bar.
2. Open `Settings...` and confirm the model, timing, environment, and context directory. The repo root is only used by local `swift run` development builds.
3. Click `Check Setup` to verify Ollama, the model, and macOS capture tools. Local development builds also verify Go and the repo root.
4. Open `Windows`.
5. Select the window to watch.
6. Click `Verify Current` to run a one-shot check against the current window contents, or `Start Watching` to keep monitoring.

The log opens automatically and macOS notifications are sent for emitted alerts.

## Configuration

Use `Settings...` from the menu bar to configure and save values in macOS preferences.

Use `Check Setup` after changing settings. It runs:

```bash
opswatch doctor
```

from the bundled CLI in release builds, or `go run ./cmd/opswatch doctor` in local development builds, then writes the result to `/tmp/opswatch-menubar.log`.

Recommended local performance defaults:

- Repo root: `/Users/vishal/go/src/github.com/vdplabs/opswatch` for local development only
- Model profile: `Balanced`
- Vision provider: `ollama`
- Model: `qwen2.5vl:3b-q4_K_M`
- Interval: `10s`
- Max image dimension: `1000`
- Ollama num predict: `128`
- Alert cooldown: `2m`
- Min analysis interval: `30s`
- Environment: `prod`
- Context directory: `~/.opswatch/context`

Optional incident context:

```bash
export OPSWATCH_INTENT="Restart one unhealthy service instance"
export OPSWATCH_EXPECTED_ACTION="restart one instance without changing shared infrastructure"
export OPSWATCH_CONTEXT_DIR="$HOME/.opswatch/context"
```

You can also enter these optional fields in `Settings...`. For richer incident context, put YAML or JSON context packs in the context directory. If they are omitted, OpsWatch still watches for high-risk actions such as DNS zone creation and destructive terminal commands.

The Settings window now includes three model profiles:

- `Fast`: `granite3.2-vision`, `768` max dimension, `96` predict tokens
- `Balanced`: `qwen2.5vl:3b-q4_K_M`, `1000` max dimension, `128` predict tokens
- `Accurate`: `llama3.2-vision`, `1200` max dimension, `192` predict tokens

Choosing a profile updates the main model and performance fields immediately, and you can still fine-tune them after that.

Useful local tuning notes:

- `qwen2.5vl:3b-q4_K_M` is the current best balanced local model on Apple Silicon for OpsWatch workloads
- warm runs are meaningfully faster than the first run after restarting Ollama
- `OLLAMA_FLASH_ATTENTION=1` is worth testing, but it is not the main performance lever
- native Apple OCR is used before falling back to the VLM when the helper is available

## Status Indicators

- shield/eye icon plus `OpsWatch` means idle
- shield/eye icon plus `OpsWatch ◦` means a window is selected
- shield/eye icon plus `OpsWatch …` means watcher is starting
- shield/eye icon plus `OpsWatch ●` means watching
- shield/eye icon plus `OpsWatch !` means attention needed

Logs are written to:

```text
/tmp/opswatch-menubar.log
```

The log opens automatically when you click `Start Watching`. The watcher also sends macOS notifications for emitted alerts.

`Verify Current` runs the same analyzer once against the selected window and writes the result to the same log file. It is useful for testing model quality, context, and current policy coverage before starting continuous watching.

## Permissions

macOS may ask for Screen Recording permission for Terminal, Swift, or the built app. If the window list is incomplete or captures fail, grant permission in:

System Settings -> Privacy & Security -> Screen Recording

## Troubleshooting Swift

If `swift run` fails with `Invalid manifest`, `undefined symbols for architecture arm64`, or SDK/compiler mismatch errors, check the local Apple toolchain:

```bash
cd /Users/vishal/go/src/github.com/vdplabs/opswatch/macos/OpsWatchBar

swift --version
xcode-select -p
xcrun --show-sdk-path
```

If Xcode is installed, prefer the full Xcode toolchain:

```bash
sudo xcode-select --switch /Applications/Xcode.app/Contents/Developer
```

If only Command Line Tools are installed, refresh them:

```bash
xcode-select --install
```

Then clean and rebuild:

```bash
rm -rf .build
swift package reset
swift build
swift run OpsWatchBar
```

The menu bar app requires macOS 13 or newer. If the Swift compiler and SDK versions do not match, update Xcode from the App Store or Apple Developer downloads, then rerun the build.
