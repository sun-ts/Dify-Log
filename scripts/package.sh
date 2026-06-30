#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="$ROOT/dist"
APP_NAME="dify-log-excel"
GO_BIN="${GO_BIN:-go}"

rm -rf "$DIST"
mkdir -p "$DIST"

cd "$ROOT"
"$GO_BIN" test ./...

build_one() {
  local goos="$1"
  local goarch="$2"
  local label="$3"
  local ext="$4"
  local archive_ext="$5"
  local package_name="${APP_NAME}-${label}-${goarch}"
  local package_dir="$DIST/$package_name/$APP_NAME"
  mkdir -p "$package_dir/data" "$package_dir/logs"

  local binary="$APP_NAME$ext"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 "$GO_BIN" build -o "$package_dir/$binary" ./cmd/dify-log-excel

  cp "$ROOT/config.toml" "$package_dir/config.toml"
  cp "$ROOT/config.example.toml" "$package_dir/config.example.toml"
  cp "$ROOT/README.md" "$package_dir/README.md"
  cp "$ROOT/scripts/start.sh" "$package_dir/start.sh"
  cp "$ROOT/scripts/start.command" "$package_dir/start.command"
  cp "$ROOT/scripts/start.bat" "$package_dir/start.bat"
  chmod +x "$package_dir/start.sh" "$package_dir/start.command" || true
  touch "$package_dir/data/.gitkeep" "$package_dir/logs/.gitkeep"

  if [[ "$archive_ext" == "zip" ]]; then
    (cd "$DIST/$package_name" && zip -qr "../$package_name.zip" "$APP_NAME")
  else
    (cd "$DIST/$package_name" && tar -czf "../$package_name.tar.gz" "$APP_NAME")
  fi
}

build_one darwin arm64 macos "" zip
build_one darwin amd64 macos "" zip
build_one linux amd64 linux "" tar.gz
build_one windows amd64 windows ".exe" zip

cd "$DIST"
if command -v shasum >/dev/null 2>&1; then
  shasum -a 256 *.zip *.tar.gz > SHA256SUMS
elif command -v sha256sum >/dev/null 2>&1; then
  sha256sum *.zip *.tar.gz > SHA256SUMS
fi

find "$DIST" -maxdepth 1 -type f -name "${APP_NAME}-*" -print
