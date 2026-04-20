# Releases

OpsWatch can publish downloadable macOS artifacts from GitHub Releases.

## Artifacts

Each tagged release builds:

- `opswatch-darwin-arm64`: CLI for Apple Silicon Macs
- `opswatch-darwin-amd64`: CLI for Intel Macs
- `OpsWatchBar-macos.zip`: unsigned macOS menu bar app
- `checksums.txt`: SHA-256 checksums for CLI binaries

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
dist/OpsWatchBar-macos.zip
```

Build the CLI locally:

```bash
GOOS=darwin GOARCH=arm64 go build -o dist/opswatch-darwin-arm64 ./cmd/opswatch
GOOS=darwin GOARCH=amd64 go build -o dist/opswatch-darwin-amd64 ./cmd/opswatch
```

## Installing The Menu Bar App

Download `OpsWatchBar-macos.zip`, unzip it, and move `OpsWatchBar.app` to `/Applications`.

The app is currently unsigned and not notarized. macOS may block it on first open. To allow it:

1. Open System Settings.
2. Go to Privacy & Security.
3. Allow `OpsWatchBar.app` under the security warning.

Future releases should add Developer ID signing and notarization.
