# OpsWatch

OpsWatch is an incident change witness: it compares what operators intend to do during an incident with what is actually being changed on screen, in terminals, and through infrastructure APIs.

The current prototype is intentionally focused. It turns what is visible on screen into structured operational events, compares them with local context and policy, and emits alerts when a risky action does not match stated intent or safety guardrails.

![Landscape](docs/images/landscape.png)

## Quick Start

The fastest path is the macOS menu bar app:

1. Download `OpsWatchBar-macos-arm64.zip` from [GitHub Releases](https://github.com/vdplabs/opswatch/releases).
2. Move `OpsWatchBar.app` to `/Applications`.
3. Start Ollama and pull `qwen2.5vl:3b-q4_K_M`.
4. Click the OpsWatch menu bar icon, select a window, and click `Start Watching`.

CLI-only installs can download `opswatch-cli-darwin-arm64` for Apple Silicon or `opswatch-cli-darwin-amd64` for Intel Macs.

## Why

During incident bridges, screen share gives visibility but not verification. People can see a console or terminal, yet still miss the exact account, object type, region, command flag, or blast radius.

OpsWatch is built around the delta between:

- spoken or written intent
- observed operational action
- known infrastructure state
- incident policy

Example:

> Intent: restart one unhealthy service instance
> Observed: run a broad delete command against all production service instances
> Alert: broad destructive action with production blast radius

![Flow](docs/images/flow.png)

## Current Prototype

This repo currently includes:

- a Go CLI: `opswatch analyze`
- JSON event ingestion for speech, screen, terminal, API, and runbook observations
- OCR-first screenshot and window analysis with local VLM fallback
- a macOS watcher for selected windows and screen regions using `screencapture`
- a native macOS menu bar app for `Verify Current` and `Start Watching`
- DNS and terminal safety policies
- local YAML/JSON context packs for protected domains, AWS accounts, services, and incident runbooks
- high-signal alert output
- sample incident streams for validating high-risk change detection

## Try It

```bash
go test ./...
go run ./cmd/opswatch doctor --vision-provider ollama --model qwen2.5vl:3b-q4_K_M --repo-root .
go run ./cmd/opswatch analyze --events examples/infra_change.jsonl
```

Expected output includes critical alerts when a broad production-impacting command is observed.

## Analyze A Screenshot

Pass a screenshot into the same analyzer pipeline. For local-only analysis, use Ollama with a vision model:

```bash
ollama serve
ollama pull qwen2.5vl:3b-q4_K_M

go run ./cmd/opswatch analyze-image \
  --vision-provider ollama \
  --model qwen2.5vl:3b-q4_K_M \
  --image /path/to/screenshot.png \
  --context-dir ~/.opswatch/context \
  --max-image-dimension 1200 \
  --ollama-num-predict 128 \
  --environment prod
```

You can also use OpenAI vision:

```bash
export OPENAI_API_KEY=...

go run ./cmd/opswatch analyze-image \
  --vision-provider openai \
  --image /path/to/screenshot.png \
  --intent "Restart one unhealthy service instance" \
  --expected-action "restart one instance without changing shared infrastructure" \
  --environment prod
```

The vision step converts the image into a normalized `screen` event, then the regular OpsWatch policies decide whether to alert.

## Performance Tuning

For Apple Silicon Macs, the current best local default is:

- model: `qwen2.5vl:3b-q4_K_M`
- `num_ctx`: `4096`
- `--max-image-dimension`: `1000` for browser windows, `768-1000` for terminal-heavy windows
- `--ollama-num-predict`: `128`

Optional Ollama runtime tweak to test:

```bash
export OLLAMA_FLASH_ATTENTION=1
ollama serve
```

This may help on some setups, but the biggest wins so far have come from:

- smaller quantized models
- native Apple OCR before VLM fallback
- warmed local runtime after the first request

Expect the first run after a restart to be slower than steady-state runs.

## Benchmark Vision Models

Compare local vision models against the same screenshot and context:

```bash
go run ./cmd/opswatch bench vision \
  --image /path/to/screenshot.png \
  --models qwen2.5vl:3b-q4_K_M,qwen2.5vl,granite3.2-vision,llama3.2-vision \
  --context-dir examples/context \
  --runs 3
```

Use `go run ./cmd/opswatch`, not `go run cmd/opswatch/main.go`. The latter compiles only `main.go` and skips sibling files that contain subcommands.

Useful candidates:

- `qwen2.5vl:3b-q4_K_M`: balanced local default for UI, browser, console, and terminal screenshots
- `qwen2.5vl`: higher-cost alternative when the 3B quantized model misses details
- `granite3.2-vision`: smaller and faster document/OCR-oriented model
- `llama3.2-vision`: slower fallback

## Local Context

OpsWatch can read local context packs from `~/.opswatch/context` or a path passed with `--context-dir`. These packs provide incident intent, expected action, protected domains, AWS accounts, service ownership, and runbook hints without sending internal inventory anywhere.

```bash
go run ./cmd/opswatch context init
go run ./cmd/opswatch context inspect
```

Sync the current AWS CLI account into context:

```bash
go run ./cmd/opswatch context sync aws \
  --profile prod \
  --environment prod \
  --account-name prod \
  --owner platform \
  --risk critical
```

See [docs/context-packs.md](docs/context-packs.md) for the schema.

## Start Watching

On macOS, the prototype can capture the full screen repeatedly and analyze each frame:

```bash
ollama serve

go run ./cmd/opswatch watch \
  --vision-provider ollama \
  --model qwen2.5vl:3b-q4_K_M \
  --interval 10s \
  --capture-rect 900,0,1150,1000 \
  --max-image-dimension 1200 \
  --ollama-num-predict 128 \
  --skip-unchanged \
  --min-analysis-interval 30s \
  --alert-cooldown 2m \
  --notify \
  --verbose \
  --environment prod
```

This is the lower-level CLI watch mode. For most testing on macOS, the menu bar app is the better path because it targets a selected window and keeps unrelated desktop pixels out of the analysis path.

Local vision models can briefly make the laptop feel busy, especially on the first request or with large Retina screenshots. Use `--max-image-dimension 1200`, `--ollama-num-predict 128`, `--min-analysis-interval 30s`, and a slower watch interval while testing.

Watch mode now skips frames that look visually unchanged, suppresses duplicate alerts during a cooldown window, and deletes temporary frames by default. Use `--keep-frames` only when debugging what the watcher captured.

Use `--notify` on macOS to show a local notification whenever OpsWatch emits an alert.

Use `--capture-rect x,y,width,height` to watch only the operational part of the screen. On macOS this uses `screencapture -R`. In a layout with Terminal on the left and AWS Console on the right, a rectangle like `900,0,1150,1000` avoids sending Terminal and browser chrome to the vision model. Add `--verbose` to see capture, resize, hash, and vision timings for each frame.

You can also target a specific macOS window when you know its window id:

```bash
go run ./cmd/opswatch watch \
  --vision-provider ollama \
  --model qwen2.5vl:3b-q4_K_M \
  --window-id 12345 \
  --interval 10s \
  --max-image-dimension 1000 \
  --ollama-num-predict 128 \
  --min-analysis-interval 30s \
  --environment prod
```

Intent, expected action, and protected domains are optional. Without them, OpsWatch still emits generic high-risk action warnings. Set these only when incident context is available:

```bash
export OPSWATCH_INTENT="Restart one unhealthy service instance"
export OPSWATCH_EXPECTED_ACTION="restart one instance without changing shared infrastructure"
export OPSWATCH_CONTEXT_DIR="$HOME/.opswatch/context"
```

## Menu Bar App

The macOS companion lives in `macos/OpsWatchBar`. It lists visible windows, lets you select one, and starts/stops OpsWatch from the menu bar.

Downloadable builds are published from GitHub Releases. See [docs/releases.md](docs/releases.md) for release artifacts and packaging.

### Menu Bar Quickstart

Start Ollama and pull the local vision model:

```bash
ollama serve
ollama pull qwen2.5vl:3b-q4_K_M
```

For the easiest path, download `OpsWatchBar-macos-arm64.zip` from GitHub Releases, unzip it, and move `OpsWatchBar.app` to `/Applications`. The app bundle includes the `opswatch` CLI, so you do not need a Go checkout for the menu bar app.

For local development, launch the menu bar app with Swift:

```bash
cd /Users/vishal/go/src/github.com/vdplabs/opswatch/macos/OpsWatchBar
swift build
swift run OpsWatchBar
```

Then use the menu bar:

1. Click `OpsWatch`.
2. Open `Settings...` and confirm the model, timing, environment, and context directory. The repo root is only used by local `swift run` development builds.
3. Click `Check Setup` to verify Ollama, the model, and macOS capture tools. Local development builds also verify Go and the repo root.
4. Open `Windows`.
5. Select the browser, terminal, Zoom, or console window to watch.
6. Click `Verify Current` for a one-shot check, or `Start Watching` for continuous monitoring.
7. Keep the automatically opened log window visible.

The menu bar status indicators are:

- shield/eye icon plus `OpsWatch` means idle
- shield/eye icon plus `OpsWatch ◦` means a window is selected
- shield/eye icon plus `OpsWatch …` means watcher is starting
- shield/eye icon plus `OpsWatch ●` means watching
- shield/eye icon plus `OpsWatch !` means attention needed

Optional incident context makes alerts more specific. You can set these in `Settings...` or put them in local context packs:

```bash
export OPSWATCH_INTENT="Restart one unhealthy service instance"
export OPSWATCH_EXPECTED_ACTION="restart one instance without changing shared infrastructure"
export OPSWATCH_CONTEXT_DIR="$HOME/.opswatch/context"
```

Without these optional values, OpsWatch still emits baseline high-risk warnings such as DNS zone creation, destructive terminal commands, IAM changes, network edge changes, infra apply/deploy actions, and broad-scope operations.

The intended direction is low-friction intent capture, in this order:

1. one-line intent entered in the menu bar before watching
2. local context packs synced from incident tooling or cloud inventory
3. inferred context from visible ticket, chat, or console metadata
4. future live speech/transcript adapters for declared operator intent

The menu bar Settings window includes three model profiles:

- `Fast`: `granite3.2-vision`, `768` max dimension, `96` predict tokens
- `Balanced`: `qwen2.5vl:3b-q4_K_M`, `1000` max dimension, `128` predict tokens
- `Accurate`: `llama3.2-vision`, `1200` max dimension, `192` predict tokens

Logs are written to `/tmp/opswatch-menubar.log`. macOS may require Screen Recording permission for Terminal, Swift, or the packaged app.

When you click `Start Watching`, the menu bar app opens the log file immediately and passes `--notify` to the watcher so alerts also appear through macOS notifications.

`Verify Current` uses the same selected-window pipeline, but runs only one capture and analysis pass. It is a good way to test the current screen, model, and context before leaving OpsWatch running in the background.

If `swift run` fails on another Mac with `Invalid manifest` or `undefined symbols for architecture arm64`, see [macos/OpsWatchBar/README.md](macos/OpsWatchBar/README.md#troubleshooting-swift) for Xcode/SwiftPM cleanup steps.

## Event Model

OpsWatch consumes JSON Lines events. Each line is one observation:

```json
{"ts":"2026-04-20T20:42:10Z","source":"speech","actor":"incident-commander","text":"Restart one unhealthy service instance"}
```

Important event sources:

- `speech`: transcript snippets from Zoom or the bridge
- `screen`: OCR or vision summaries from shared screen frames
- `terminal`: commands and output extracted from terminals
- `api`: read-only infrastructure state
- `runbook`: expected action from runbook or ticket context

## Product Direction

The near-term wedge is high-risk console and terminal verification:

- Route53, Cloudflare, Azure DNS, and GCP DNS console flows
- `aws route53`, `gcloud dns`, `az network dns`, and common shell commands
- environment/account mismatch
- protected domain mutations
- destructive command patterns

Later adapters can feed the same analyzer from Zoom, Slack, OCR, browser automation, read-only cloud APIs, and incident management systems.
