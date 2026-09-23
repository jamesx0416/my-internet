#!/usr/bin/env bash
set -euo pipefail

APK="${1:-}"
if [[ -z "$APK" || ! -f "$APK" ]]; then
  echo "Usage: $0 /path/to/app-debug.apk" >&2
  exit 2
fi

if ! command -v adb >/dev/null 2>&1; then
  echo "adb is not installed." >&2
  echo "On macOS, the small option is: brew install --cask android-platform-tools" >&2
  exit 3
fi

echo "Connected Android devices:"
adb devices

echo
echo "Installing Split Router..."
adb install -r "$APK"

echo
echo "Installed. Start Shizuku, open Split Router, grant Shizuku access, then press Start."
