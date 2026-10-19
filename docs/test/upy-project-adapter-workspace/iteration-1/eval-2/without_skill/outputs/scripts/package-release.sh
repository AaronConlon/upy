#!/usr/bin/env bash
# 在目标应用仓库运行：bash scripts/package-release.sh v1.2.3
set -euo pipefail
umask 022
cd "$(dirname "${BASH_SOURCE[0]}")/.."
version="${1:-}"
# 限制版本字符，避免路径穿越和清单替换注入；保留 tag 的 v 前缀。
if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$ ]]; then
  echo '用法: bash scripts/package-release.sh v1.2.3（支持 SemVer 预发布/构建后缀）' >&2
  exit 2
fi
for tool in go python3; do command -v "$tool" >/dev/null; done
[[ "$(go env GOVERSION)" == go1.25.* ]] || { echo '需要 Go 1.25.x' >&2; exit 2; }
[[ -f go.mod && -d cmd/api ]] || { echo '请在具有 go.mod 和 cmd/api 的目标应用仓库集成' >&2; exit 2; }
mkdir -p dist
stage="$(mktemp -d "$PWD/dist/.package.XXXXXX")"
trap 'rm -rf "$stage"' EXIT
for arch in amd64 arm64; do
  mkdir -p "$stage/$arch/bin"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -o "$stage/$arch/bin/demo-api" ./cmd/api
  chmod 0755 "$stage/$arch/bin/demo-api"
  python3 - "$version" "$arch" "$stage/$arch" <<'PYZIP'
import pathlib, sys, zipfile
version, arch, stage = sys.argv[1:]
base = pathlib.Path(stage)
text = pathlib.Path('packaging/deploy.yaml.tmpl').read_text()
text = text.replace('@@VERSION@@', version).replace('@@ARCH@@', arch)
(base / 'deploy.yaml').write_text(text)
with zipfile.ZipFile(base / 'bundle.zip', 'w', zipfile.ZIP_DEFLATED) as z:
    z.write(base / 'deploy.yaml', 'deploy.yaml')
    z.write(base / 'bin/demo-api', 'bin/demo-api')
PYZIP
 done
# 两架构都构建成功后才写最终资产；ZIP 内没有多余顶层目录。
for arch in amd64 arm64; do
  mv "$stage/$arch/bundle.zip" "dist/demo-api-linux-$arch.zip"
done
