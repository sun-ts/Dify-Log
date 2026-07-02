#!/usr/bin/env sh
set -eu
SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
if [ -x "$SCRIPT_DIR/dify-log-excel" ]; then
  cd "$SCRIPT_DIR"
else
  cd "$SCRIPT_DIR/.."
fi
./dify-log-excel start
