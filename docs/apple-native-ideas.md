# Apple-Native Ideas

OpsWatch runs on macOS first, so Apple-native inference and OCR paths are worth treating as primary architecture, not just nice-to-have optimizations.

## Near-Term

- Apple Vision OCR as the default fast path for selected-window screenshots
- VLM fallback only when OCR confidence is weak or the screen is highly visual
- terminal-specific extraction from OCR text before invoking any general multimodal model

## Why

- terminal and cloud console screens are text-heavy
- OCR is dramatically cheaper than a full VLM pass
- local-only processing is easier to explain to security reviewers
- Apple Vision ships with the OS and avoids extra model setup

## Medium-Term

- Foundation Models as a local text-normalization layer after OCR
- browser-page heuristics for common operational consoles such as Route53, IAM, Kubernetes dashboards, and CI/CD pages
- Accessibility-based extraction for terminal-like apps when permissions allow it

## Product Shape

1. capture selected window
2. run Apple Vision OCR
3. normalize text into OpsWatch events
4. run Go policies
5. fall back to a VLM only when the cheaper path is insufficient

## Current Constraint

Apple Foundation Models are promising for local text reasoning, but they are not a direct replacement for screenshot vision input. OCR plus structured normalization is the cleaner bridge.
