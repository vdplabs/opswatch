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
swift run
```

Then:

1. Click `OpsWatch` in the menu bar.
2. Open `Settings...` and confirm the repo root, model, timing, and environment.
3. Click `Check Setup` to verify Go, Ollama, the model, and macOS capture tools.
4. Open `Windows`.
5. Select the window to watch.
6. Click `Start Watching`.

The log opens automatically and macOS notifications are sent for emitted alerts.

## Configuration

Use `Settings...` from the menu bar to configure and save values in macOS preferences.

Use `Check Setup` after changing settings. It runs:

```bash
go run ./cmd/opswatch doctor
```

and writes the result to `/tmp/opswatch-menubar.log`.

Recommended local performance defaults:

- Repo root: `/Users/vishal/go/src/github.com/vdplabs/opswatch`
- Vision provider: `ollama`
- Model: `llama3.2-vision`
- Interval: `10s`
- Max image dimension: `1000`
- Ollama num predict: `128`
- Alert cooldown: `2m`
- Min analysis interval: `30s`
- Environment: `prod`

Optional incident context:

```bash
export OPSWATCH_INTENT="Add a CNAME record for api.example.com"
export OPSWATCH_EXPECTED_ACTION="add CNAME record in existing hosted zone"
export OPSWATCH_PROTECTED_DOMAIN=example.com
```

You can also enter these optional fields in `Settings...`. If they are omitted, OpsWatch still watches for high-risk actions such as DNS zone creation and destructive terminal commands.

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
