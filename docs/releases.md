# Releases

OpsWatch can publish downloadable macOS artifacts from GitHub Releases.

## Artifacts

Each tagged release builds:

- `opswatch-cli-darwin-arm64`: CLI for Apple Silicon Macs
- `opswatch-cli-darwin-amd64`: CLI for Intel Macs
- `OpsWatchBar-macos-arm64.zip`: unsigned macOS menu bar app with the `opswatch` CLI bundled inside the app
- `checksums.txt`: SHA-256 checksums for CLI binaries

Every GitHub release should include:

- `What is OpsWatch`: one sentence plus a README link
- `This release`: a short summary of what changed in the tag
- `Quick start`: download, unzip, pull a model, select a window, start watching

## Create A Release

Create and push a version tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

GitHub Actions will build the artifacts and attach them to the release.

You can also run the workflow manually from GitHub Actions using `workflow_dispatch`.

## Local Packaging

Build the menu bar app zip locally:

```bash
bash scripts/package_macos_app.sh
```

The artifact is written to:

```text
dist/OpsWatchBar-macos-arm64.zip
```

The package script builds the Swift menu bar app and copies a native `opswatch` CLI into `OpsWatchBar.app/Contents/Resources/opswatch`. Release installs can start watching without a Go checkout.

Build the CLI locally:

```bash
GOOS=darwin GOARCH=arm64 go build -o dist/opswatch-cli-darwin-arm64 ./cmd/opswatch
GOOS=darwin GOARCH=amd64 go build -o dist/opswatch-cli-darwin-amd64 ./cmd/opswatch
```

## Installing The Menu Bar App

Download `OpsWatchBar-macos-arm64.zip`, unzip it, and move `OpsWatchBar.app` to `/Applications`.

Start Ollama and pull the local vision model before watching:

```bash
ollama serve
ollama pull qwen2.5vl:3b-q4_K_M
```

The app is currently unsigned and not notarized. macOS may block it on first open. To allow it:

1. Open System Settings.
2. Go to Privacy & Security.
3. Allow `OpsWatchBar.app` under the security warning.

Future releases should add Developer ID signing and notarization.
