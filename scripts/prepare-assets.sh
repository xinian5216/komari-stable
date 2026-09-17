#!/usr/bin/env bash
# 薄包装：repo/tag/hash/下载/校验的全部逻辑都在 scripts/prepare-assets.py（跨平台单一事实来源）。
set -euo pipefail
cd "$(dirname "$0")/.."

PY=""
for candidate in python3 python; do
  if command -v "$candidate" >/dev/null 2>&1 && "$candidate" -c 'import zipfile' >/dev/null 2>&1; then
    PY="$candidate"
    break
  fi
done
if [ -z "$PY" ]; then
  echo "FAIL: python3 (or python) with the zipfile module is required" >&2
  exit 1
fi
exec "$PY" scripts/prepare-assets.py "$@"
