# Context Packs

OpsWatch uses local context packs to understand company-specific infrastructure during an incident. Vision describes what appears on screen; context packs tell OpsWatch which domains, accounts, services, and runbook actions are in scope.

By default, OpsWatch reads YAML or JSON files from:

```text
~/.opswatch/context
```

You can override this path with `--context-dir` or `OPSWATCH_CONTEXT_DIR`. The menu bar app exposes the same value as `Context directory` in `Settings...`.

## Quickstart

Create a starter pack:

```bash
opswatch context init
opswatch context inspect
```

You can also sync the active AWS CLI identity into a local pack:

```bash
opswatch context sync aws \
  --profile prod \
  --environment prod \
  --account-name prod \
  --owner platform \
  --risk critical
```

To also import Route 53 hosted zones as protected domains, add `--include-route53`. This uses read-only AWS CLI calls: `sts get-caller-identity` and, when requested, `route53 list-hosted-zones`.

Then run with context:

```bash
go run ./cmd/opswatch analyze-image \
  --vision-provider ollama \
  --model qwen2.5vl:3b-q4_K_M \
  --image /path/to/screenshot.png \
  --context-dir ~/.opswatch/context
```

The context pack can provide intent and expected action, so those CLI flags become optional when the pack contains active incident context.

## Schema

```yaml
incident:
  id: inc-demo
  title: Demo service incident
  intent: Restart one unhealthy service instance
  expected_action: restart one instance without changing shared infrastructure
  environment: prod
  service: api

protected_domains:
  - name: example.com
    environment: prod
    owner: platform
    authoritative_zone_id: Z123456789
    risk: critical

aws_accounts:
  - id: "123456789012"
    name: prod
    environment: prod
    owner: platform
    risk: critical

services:
  - name: api
    environment: prod
    owner: application-platform
    tier: tier-0
    risk: critical

runbooks:
  - id: service-restart
    title: Restart one service instance
    service: api
    environment: prod
    expected_action: restart one instance without changing shared infrastructure
    allowed_actions:
      - kubernetes.restart_pod
```

## How Context Is Used

OpsWatch converts context pack entries into normal incident events before analyzing a screenshot or event file.

- `incident` fills current intent, expected action, service, and environment when CLI flags are not provided.
- `protected_domains` enrich DNS policies with owner, environment, risk, and authoritative zone ID.
- `aws_accounts` lets policies recognize production account mutations when vision extracts an `account_id`.
- `services` records ownership and criticality for future service-aware policies.
- `runbooks` provides expected action hints and allowed action names for policy evolution.

Context packs are local files. They are not uploaded by OpsWatch.

## AWS Sync

`opswatch context sync aws` writes a pack named `aws-<account-id>.yaml` in the context directory. It intentionally uses the AWS CLI instead of storing long-lived credentials in OpsWatch.

Useful flags:

- `--profile`: AWS CLI profile to use
- `--region`: AWS CLI region for commands that need one
- `--environment`: environment label such as `prod` or `staging`
- `--account-name`: friendly account name
- `--owner`: owning team
- `--risk`: risk label such as `critical`, `high`, or `medium`
- `--include-route53`: import hosted zones as protected domains
