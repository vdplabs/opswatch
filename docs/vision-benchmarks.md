# Vision Benchmarks

OpsWatch can benchmark multiple local vision models against the same screenshot and context pack.

```bash
go run ./cmd/opswatch bench vision \
  --image examples/r53_dns.png \
  --models llama3.2-vision,qwen2.5vl,granite3.2-vision \
  --context-dir examples/context \
  --runs 3
```

Use the package form, `go run ./cmd/opswatch`. Do not run `go run cmd/opswatch/main.go`; that compiles only one file and skips sibling files that define subcommands.

## Candidate Models

- `qwen2.5vl`: good first choice for operational UI screenshots. Pull with `ollama pull qwen2.5vl`.
- `granite3.2-vision`: smaller and often faster for OCR/document-style extraction. Pull with `ollama pull granite3.2-vision`.
- `llama3.2-vision`: useful fallback, but often slower on laptop workloads.

## Output

The text table includes:

- `json`: successful normalized event extractions over requested runs
- `avg`, `p95`, `min`, `max`: model request duration
- `alerts`: alerts emitted from the final successful run
- `fail`: failed runs
- `last`: last extracted screen event or error

Use `--format json` for machine-readable output.
