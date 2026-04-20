# OpsWatchBar

OpsWatchBar is the macOS menu bar companion for OpsWatch.

It lists visible windows, lets you pick one, and starts the Go watcher in the background with `--window-id`.

## Getting Started

Start Ollama and pull a local vision model:

```bash
ollama serve
ollama pull llama3.2-vision
```

Launch the menu bar app:

```bash
cd /Users/vishal/go/src/github.com/vdplabs/opswatch/macos/OpsWatchBar

OPSWATCH_ROOT=/Users/vishal/go/src/github.com/vdplabs/opswatch \
OPSWATCH_VISION_PROVIDER=ollama \
OPSWATCH_MODEL=llama3.2-vision \
OPSWATCH_INTERVAL=10s \
OPSWATCH_MAX_IMAGE_DIMENSION=1000 \
OPSWATCH_OLLAMA_NUM_PREDICT=128 \
OPSWATCH_ALERT_COOLDOWN=2m \
OPSWATCH_MIN_ANALYSIS_INTERVAL=30s \
OPSWATCH_ENVIRONMENT=prod \
swift run
```

Then:

1. Click `OpsWatch` in the menu bar.
2. Open `Windows`.
3. Select the window to watch.
4. Click `Start Watching`.

The log opens automatically and macOS notifications are sent for emitted alerts.

## Configuration

The app reads configuration from environment variables when it starts.

Required for normal local use:

```bash
export OPSWATCH_ROOT=/Users/vishal/go/src/github.com/vdplabs/opswatch
export OPSWATCH_VISION_PROVIDER=ollama
export OPSWATCH_MODEL=llama3.2-vision
```

Recommended local performance defaults:

```bash
export OPSWATCH_INTERVAL=10s
export OPSWATCH_MAX_IMAGE_DIMENSION=1000
export OPSWATCH_OLLAMA_NUM_PREDICT=128
export OPSWATCH_ALERT_COOLDOWN=2m
export OPSWATCH_MIN_ANALYSIS_INTERVAL=30s
export OPSWATCH_ENVIRONMENT=prod
```

Optional incident context:

```bash
export OPSWATCH_INTENT="Add a CNAME record for api.example.com"
export OPSWATCH_EXPECTED_ACTION="add CNAME record in existing hosted zone"
export OPSWATCH_PROTECTED_DOMAIN=example.com
```

If these are omitted, OpsWatch still watches for high-risk actions such as DNS zone creation and destructive terminal commands.

## Status Indicators

- `OpsWatch` means idle
- `OpsWatch ◦` means a window is selected
- `OpsWatch …` means watcher is starting
- `OpsWatch ●` means watching
- `OpsWatch !` means attention needed

Logs are written to:

```text
/tmp/opswatch-menubar.log
```

The log opens automatically when you click `Start Watching`. The watcher also sends macOS notifications for emitted alerts.

## Permissions

macOS may ask for Screen Recording permission for Terminal, Swift, or the built app. If the window list is incomplete or captures fail, grant permission in:

System Settings -> Privacy & Security -> Screen Recording
