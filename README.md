# OpsWatch

OpsWatch is an incident change witness: it compares what operators intend to do during an incident with what is actually being changed on screen, in terminals, and through infrastructure APIs.

The first prototype is intentionally narrow. It reads a stream of observed incident events and emits precise alerts when a dangerous action does not match the stated intent or safety policy.

## Why

During incident bridges, screen share gives visibility but not verification. People can see a console or terminal, yet still miss the exact account, object type, region, command flag, or blast radius.

OpsWatch is built around the delta between:

- spoken or written intent
- observed operational action
- known infrastructure state
- incident policy

Example:

> Intent: add a DNS record
>
> Observed: create a new primary DNS zone
>
> Alert: possible intent mismatch with high DNS blast radius

## Current Prototype

This repo currently includes:

- a Go CLI: `opswatch analyze`
- JSON event ingestion for speech, screen, terminal, API, and runbook observations
- screenshot/image analysis through OpenAI vision
- a macOS fullscreen watcher prototype using `screencapture`
- DNS and terminal safety policies
- high-signal alert output
- a sample incident stream based on the DNS-zone-vs-record failure mode

## Try It

```bash
go test ./...
go run ./cmd/opswatch analyze --events examples/dns_incident.jsonl
```

Expected output includes a critical alert when a hosted zone creation is observed while the stated intent is to add a DNS record.

## Analyze A Screenshot

Pass a screenshot into the same analyzer pipeline. For local-only analysis, use Ollama with a vision model:

```bash
ollama serve
ollama pull llama3.2-vision

go run ./cmd/opswatch analyze-image \
  --vision-provider ollama \
  --model llama3.2-vision \
  --image examples/r53_dns.png \
  --max-image-dimension 1200 \
  --ollama-num-predict 128 \
  --intent "Add a CNAME record for api.example.com" \
  --expected-action "add CNAME record in existing hosted zone" \
  --protected-domain example.com \
  --environment prod
```

You can also use OpenAI vision:

```bash
export OPENAI_API_KEY=...

go run ./cmd/opswatch analyze-image \
  --vision-provider openai \
  --image /path/to/screenshot.png \
  --intent "Add a CNAME record for api.example.com" \
  --expected-action "add CNAME record in existing hosted zone" \
  --protected-domain example.com \
  --environment prod
```

The vision step converts the image into a normalized `screen` event, then the regular OpsWatch policies decide whether to alert.

## Start Watching

On macOS, the prototype can capture the full screen repeatedly and analyze each frame:

```bash
ollama serve

go run ./cmd/opswatch watch \
  --vision-provider ollama \
  --model llama3.2-vision \
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

This is the early laptop mode. The next adapter should target a selected app/window instead of the full screen, so OpsWatch can watch Zoom, a browser, or a terminal without sending unrelated desktop pixels.

Local vision models can briefly make the laptop feel busy, especially on the first request or with large Retina screenshots. Use `--max-image-dimension 1200`, `--ollama-num-predict 128`, `--min-analysis-interval 30s`, and a slower watch interval while testing.

Watch mode now skips frames that look visually unchanged, suppresses duplicate alerts during a cooldown window, and deletes temporary frames by default. Use `--keep-frames` only when debugging what the watcher captured.

Use `--notify` on macOS to show a local notification whenever OpsWatch emits an alert.

Use `--capture-rect x,y,width,height` to watch only the operational part of the screen. On macOS this uses `screencapture -R`. In a layout with Terminal on the left and AWS Console on the right, a rectangle like `900,0,1150,1000` avoids sending Terminal and browser chrome to the vision model. Add `--verbose` to see capture, resize, hash, and vision timings for each frame.

You can also target a specific macOS window when you know its window id:

```bash
go run ./cmd/opswatch watch \
  --vision-provider ollama \
  --model llama3.2-vision \
  --window-id 12345 \
  --interval 10s \
  --max-image-dimension 1000 \
  --ollama-num-predict 128 \
  --min-analysis-interval 30s \
  --environment prod
```

Intent, expected action, and protected domains are optional. Without them, OpsWatch still emits generic high-risk action warnings. Set these only when incident context is available:

```bash
export OPSWATCH_INTENT="Add a CNAME record for api.example.com"
export OPSWATCH_EXPECTED_ACTION="add CNAME record in existing hosted zone"
export OPSWATCH_PROTECTED_DOMAIN=example.com
```

## Menu Bar App

The macOS companion lives in `macos/OpsWatchBar`. It lists visible windows, lets you select one, and starts/stops OpsWatch from the menu bar.

### Menu Bar Quickstart

Start Ollama and pull the local vision model:

```bash
ollama serve
ollama pull llama3.2-vision
```

Launch the menu bar app with local-friendly defaults:

```bash
cd /Users/vishal/go/src/github.com/vdplabs/opswatch/macos/OpsWatchBar
swift run
```

Then use the menu bar:

1. Click `OpsWatch`.
2. Open `Settings...` and confirm the repo root, model, timing, and environment.
3. Open `Windows`.
4. Select the browser, terminal, Zoom, or console window to watch.
5. Click `Start Watching`.
6. Keep the automatically opened log window visible.

The menu bar status indicators are:

- `OpsWatch` means idle
- `OpsWatch ◦` means a window is selected
- `OpsWatch …` means watcher is starting
- `OpsWatch ●` means watching
- `OpsWatch !` means attention needed

Optional incident context makes alerts more specific. You can set these in `Settings...`:

```bash
export OPSWATCH_INTENT="Add a CNAME record for api.example.com"
export OPSWATCH_EXPECTED_ACTION="add CNAME record in existing hosted zone"
export OPSWATCH_PROTECTED_DOMAIN=example.com
```

Without these optional values, OpsWatch still emits baseline high-risk warnings such as DNS zone creation, destructive terminal commands, IAM changes, network edge changes, infra apply/deploy actions, and broad-scope operations.

Logs are written to `/tmp/opswatch-menubar.log`. macOS may require Screen Recording permission for Terminal, Swift, or the packaged app.

When you click `Start Watching`, the menu bar app opens the log file immediately and passes `--notify` to the watcher so alerts also appear through macOS notifications.

## Event Model

OpsWatch consumes JSON Lines events. Each line is one observation:

```json
{"ts":"2026-04-20T20:42:10Z","source":"speech","actor":"incident-commander","text":"Add a CNAME record for api.example.com"}
```

Important event sources:

- `speech`: transcript snippets from Zoom or the bridge
- `screen`: OCR or vision summaries from shared screen frames
- `terminal`: commands and output extracted from terminals
- `api`: read-only infrastructure state
- `runbook`: expected action from runbook or ticket context

## Product Direction

The near-term wedge is DNS and terminal verification:

- Route53, Cloudflare, Azure DNS, and GCP DNS console flows
- `aws route53`, `gcloud dns`, `az network dns`, and common shell commands
- environment/account mismatch
- zone creation vs record creation
- protected domain mutations
- destructive command patterns

Later adapters can feed the same analyzer from Zoom, Slack, OCR, browser automation, read-only cloud APIs, and incident management systems.
