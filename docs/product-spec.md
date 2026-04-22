# OpsWatch Product Spec

## One-Liner

OpsWatch is a live incident change witness that verifies whether observed operational changes match the stated intent, runbook, and safety policy.

## Problem

Incident bridges rely on screen sharing and human attention. That creates visibility, but not reliable verification. Operators are often stressed, moving fast, and working in unfamiliar consoles. Reviewers may see the screen without catching semantic mistakes like wrong account, wrong region, wrong object type, or destructive flags.

Example failure modes:

- intended action: restart one unhealthy service instance
- actual action: run a broad delete command against all production instances
- result: avoidable production impact during an already active incident

## Wedge

Start with high-precision alerts for high-risk console and terminal changes during declared incidents.

OpsWatch should detect:

- protected domain mutation
- high-risk DNS zone creation even when no intent is known
- production account or environment mismatch
- destructive terminal commands
- broad selectors like `--all`
- identity/access changes, network edge changes, data mutations, and infra apply/deploy actions

## User Experience

OpsWatch joins the incident bridge as an explicit participant, watches shared operational context, and posts short alerts to the incident channel.

Good alert:

> Broad destructive action: observed `kubectl delete pods --all --namespace prod`, but current intent is to restart one unhealthy service instance. Blast radius: production service fleet.

Bad alert:

> This might be risky. Please review.

Intent capture has to be lighter than change management. The product should support a ladder of intent sources:

- a one-line operator or IC statement entered at session start
- local context packs synced from tickets, cloud inventory, and runbooks
- visible metadata inferred from tabs, consoles, or channel names
- speech transcript snippets from the live incident bridge

If none of those are available, OpsWatch should still run in witness mode and emit only high-confidence scope and blast-radius alerts.

## MVP

The first implementation should support:

- Zoom or meeting frame ingestion through a pluggable adapter
- OCR/vision summaries normalized into `screen` events
- speech transcript snippets normalized into `speech` events
- read-only DNS inventory normalized into `api` events
- local context packs for protected domains, AWS accounts, services, and runbook expectations
- read-only context sync starting with AWS CLI account metadata
- policy-driven alerting
- Slack or text output
- post-incident timeline export

The incident surface should split into two views:

- notifications for high-confidence, immediately actionable alerts
- a chronological timeline for observed actions, confidence, policy fired, and incident scope

The current repo starts at the analyzer boundary, with JSONL events, screenshots, selected-window capture, and local context packs standing in for future adapters.

## Latency Standard

OpsWatch should optimize for warnings that arrive before or during the risky action, not long after it.

That means the product should prefer:

- OCR and structured extraction for text-heavy operational screens
- terminal-aware parsing for shell workflows
- VLM fallback only when cheaper extractors cannot confidently normalize the screen

## Non-Goals

- do not record full meeting video by default
- do not try to understand every possible UI
- do not block all changes in v1
- do not emit generic AI safety warnings
- do not become a covert employee surveillance tool

## Principles

- high precision over high coverage
- alerts must name the exact observed action
- alerts should name the observed action and the intent delta in one glance
- policy should explain why the action is risky
- raw video should be ephemeral by default
- stored artifacts should prefer structured events and alert summaries
- blameless operations culture matters; stored records should default to safety evidence, not ambient surveillance

## Trust And Ownership

OpsWatch should be positioned as a safety tool, not a disciplinary recorder.

- session activation must be explicit
- structured timeline retention should be configurable and auditable
- default storage should prefer redacted event summaries over raw recordings
- a future privacy audit mode should show exactly what a session would retain

## Agentic Future

OpsWatch should treat human and AI operators as first-class actors in the same event model.

- agent-declared plan should be ingested as intent
- agent-executed remediation should be normalized as observed action
- policy should compare declared plan, actual action, and protected production context the same way for both

## Connector Roadmap

Read-only adapters should prioritize the control planes engineers already trust during incidents:

- Kubernetes audit and deployment surfaces
- CloudTrail and AWS console context
- Vault audit devices and secret rotation workflows
- Terraform Cloud or plan/apply history
- GitHub Actions and deployment pipelines
- live meeting transcripts for spoken intent
