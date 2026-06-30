#!/usr/bin/env bash
set -euo pipefail

APP_NAME="dify-log-excel"

usage() {
  echo "usage: verify-package-layout.sh <package-dir> <binary-name> [--run]" >&2
  echo "       verify-package-layout.sh --self-test" >&2
}

error() {
  echo "package layout error: $*" >&2
}

verify_package_dir() {
  local package_dir="$1"
  local binary_name="$2"

  [[ -d "$package_dir" ]] || { error "$package_dir is not a directory"; return 1; }
  [[ -f "$package_dir/$binary_name" ]] || { error "$package_dir/$binary_name must be a regular file"; return 1; }
  [[ -x "$package_dir/$binary_name" ]] || { error "$package_dir/$binary_name must be executable"; return 1; }

  [[ ! -d "$package_dir/$APP_NAME" ]] || { error "$package_dir must not contain nested $APP_NAME directory"; return 1; }
  [[ -f "$package_dir/config.toml" ]] || { error "config.toml is missing"; return 1; }
  [[ -f "$package_dir/config.example.toml" ]] || { error "config.example.toml is missing"; return 1; }
  [[ -f "$package_dir/README.md" ]] || { error "README.md is missing"; return 1; }
  [[ -d "$package_dir/data" ]] || { error "data directory is missing"; return 1; }
  [[ -d "$package_dir/logs" ]] || { error "logs directory is missing"; return 1; }
  [[ -f "$package_dir/start.bat" ]] || { error "start.bat is missing"; return 1; }
  [[ -f "$package_dir/start.sh" ]] || { error "start.sh is missing"; return 1; }
  [[ -x "$package_dir/start.sh" ]] || { error "start.sh must be executable"; return 1; }
  [[ -f "$package_dir/start.command" ]] || { error "start.command is missing"; return 1; }
  [[ -x "$package_dir/start.command" ]] || { error "start.command must be executable"; return 1; }
}

run_portability_smoke() {
  local package_dir="$1"
  local binary_name="$2"
  local abs_package_dir
  local package_base
  local package_parent
  local tmp
  local status_output

  abs_package_dir="$(cd "$package_dir" && pwd -P)"
  package_base="$(basename "$abs_package_dir")"
  package_parent="$(cd "$abs_package_dir/.." && pwd -P)"

  (cd "$abs_package_dir" && "./$binary_name" version >/dev/null)
  (cd "$package_parent" && "./$package_base/$binary_name" version >/dev/null)

  tmp="$(mktemp -d)"
  status_output="$(cd "$tmp" && "$abs_package_dir/$binary_name" status)"
  rm -rf "$tmp"

  if [[ "$status_output" != *"data_dir=$abs_package_dir/data"* ]]; then
    error "status from arbitrary cwd did not use executable directory as data_dir"
    echo "$status_output" >&2
    return 1
  fi
  if [[ "$status_output" != *"excel_dir=$abs_package_dir/logs"* ]]; then
    error "status from arbitrary cwd did not use executable directory as excel_dir"
    echo "$status_output" >&2
    return 1
  fi
  rm -f "$abs_package_dir"/data/dify_logs.db*
}

self_test() {
  local tmp
  tmp="$(mktemp -d)"
  trap "rm -rf '$tmp'" EXIT

  local good="$tmp/dify-log-excel-macos-arm64"
  make_fake_package "$good" "dify-log-excel"
  verify_package_dir "$good" "dify-log-excel"

  local old_nested="$tmp/old-nested"
  mkdir -p "$old_nested/$APP_NAME"
  if verify_package_dir "$old_nested" "dify-log-excel" >/dev/null 2>&1; then
    error "self-test expected directory in binary path to fail"
    return 1
  fi

  local bad_nested="$tmp/bad-nested"
  make_fake_package "$bad_nested" "dify-log-excel.exe"
  mkdir -p "$bad_nested/$APP_NAME"
  if verify_package_dir "$bad_nested" "dify-log-excel.exe" >/dev/null 2>&1; then
    error "self-test expected nested app directory to fail"
    return 1
  fi
}

make_fake_package() {
  local package_dir="$1"
  local binary_name="$2"

  mkdir -p "$package_dir/data" "$package_dir/logs"
  touch "$package_dir/config.toml" "$package_dir/config.example.toml" "$package_dir/README.md" "$package_dir/start.bat"
  printf '#!/usr/bin/env sh\n' > "$package_dir/start.sh"
  printf '#!/usr/bin/env sh\n' > "$package_dir/start.command"
  printf '#!/usr/bin/env sh\n' > "$package_dir/$binary_name"
  chmod +x "$package_dir/start.sh" "$package_dir/start.command" "$package_dir/$binary_name"
}

main() {
  if [[ "${1:-}" == "--self-test" ]]; then
    self_test
    return
  fi
  if [[ $# -lt 2 || $# -gt 3 ]]; then
    usage
    exit 2
  fi

  verify_package_dir "$1" "$2"
  if [[ "${3:-}" == "--run" ]]; then
    run_portability_smoke "$1" "$2"
  elif [[ $# -eq 3 ]]; then
    usage
    exit 2
  fi
}

main "$@"
