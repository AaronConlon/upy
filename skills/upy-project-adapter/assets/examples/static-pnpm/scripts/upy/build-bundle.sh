#!/usr/bin/env bash
set -euo pipefail
# 用法：bash scripts/upy/build-bundle.sh v1.2.3 [artifacts/bundle.zip]
RELEASE_TAG="${1:?需要 tag，例如 v1.2.3}"
OUTPUT="${2:-artifacts/bundle.zip}"
# 仅接受正式三段版本，避免 tag 作为路径或 shell 内容使用。
if [[ ! "$RELEASE_TAG" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
  echo '仅支持 vMAJOR.MINOR.PATCH 正式版本' >&2
  exit 1
fi
VERSION="${RELEASE_TAG#v}"
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
[[ "$(node -p 'process.versions.node.split(".")[0]')" == 20 ]] || { echo '需要 Node 20' >&2; exit 1; }
[[ "$(pnpm --version)" == 9.15.0 ]] || { echo '需要 pnpm 9.15.0' >&2; exit 1; }
[[ -f pnpm-lock.yaml ]] || { echo '缺少 pnpm-lock.yaml' >&2; exit 1; }
[[ ! -e "$OUTPUT" && ! -e "$OUTPUT.sha256" ]] || { echo '输出已存在，请使用空输出路径' >&2; exit 1; }
pnpm install --frozen-lockfile
# 清除旧产物，防止本次构建误用历史 index.html。
rm -rf -- dist
pnpm run build
[[ -f dist/index.html ]] || { echo '构建缺少 dist/index.html' >&2; exit 1; }
STAGE="$(mktemp -d "${TMPDIR:-/tmp}/demo-web-upy.XXXXXX")"
trap 'rm -rf -- "$STAGE"' EXIT
# 白名单只包含 dist；pack_bundle.py 会拒绝软链与常见敏感文件。
cp -R dist "$STAGE/dist"
python3 - "$STAGE" "$VERSION" <<'PYMANIFEST'
import json
import sys
from pathlib import Path
manifest = {"name": "demo-web", "version": sys.argv[2], "type": "static", "static": {"directory": "dist"}}
(Path(sys.argv[1]) / "deploy.yaml").write_text(json.dumps(manifest, indent=2) + "\n")
PYMANIFEST
python3 scripts/upy/pack_bundle.py --stage "$STAGE" --output "$OUTPUT"
