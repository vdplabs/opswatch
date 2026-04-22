#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"
APP_NAME="OpsWatchBar"
APP_DIR="${DIST_DIR}/${APP_NAME}.app"
MACOS_DIR="${APP_DIR}/Contents/MacOS"
RESOURCES_DIR="${APP_DIR}/Contents/Resources"
VERSION="${VERSION:-}"
if [[ -z "${VERSION}" ]]; then
  TAG="$(git -C "${ROOT_DIR}" describe --tags --exact-match 2>/dev/null || true)"
  VERSION="${TAG#v}"
fi
if [[ -z "${VERSION}" ]]; then
  VERSION="0.1.0"
fi

rm -rf "${APP_DIR}"
mkdir -p "${DIST_DIR}" "${MACOS_DIR}" "${RESOURCES_DIR}"

swift build \
  --package-path "${ROOT_DIR}/macos/OpsWatchBar" \
  -c release

go build -o "${RESOURCES_DIR}/opswatch" "${ROOT_DIR}/cmd/opswatch"
cp "${ROOT_DIR}/macos/OpsWatchBar/.build/release/${APP_NAME}" "${MACOS_DIR}/${APP_NAME}"

cat > "${APP_DIR}/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>
  <string>OpsWatchBar</string>
  <key>CFBundleIdentifier</key>
  <string>com.vdplabs.opswatchbar</string>
  <key>CFBundleName</key>
  <string>OpsWatchBar</string>
  <key>CFBundleDisplayName</key>
  <string>OpsWatch</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>${VERSION}</string>
  <key>CFBundleVersion</key>
  <string>1</string>
  <key>LSMinimumSystemVersion</key>
  <string>13.0</string>
  <key>LSUIElement</key>
  <true/>
  <key>NSHighResolutionCapable</key>
  <true/>
</dict>
</plist>
PLIST

chmod +x "${MACOS_DIR}/${APP_NAME}"
chmod +x "${RESOURCES_DIR}/opswatch"

(
  cd "${DIST_DIR}"
  ditto -c -k --keepParent --norsrc "${APP_NAME}.app" "${APP_NAME}-macos.zip"
)

echo "Built ${DIST_DIR}/${APP_NAME}-macos.zip"
