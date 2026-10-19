#!/usr/bin/env bash
set -euo pipefail
# 必须安装 Node20、pnpm@9.15.0 和 Python3；从脚本位置定位项目根目录。
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
TAG="${1:?用法: bash scripts/upy/build-bundle.sh v1.2.3}"
# 限制为路径安全的 SemVer tag，避免将 tag 当成路径或 YAML 注入。
[[ "$TAG" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$ ]] || {
  echo "tag 应为 v1.2.3 或 v1.2.3-rc.1" >&2; exit 1;
}
[[ "$(node -p 'process.versions.node.split(".")[0]')" == 20 ]] || { echo '需要 Node20' >&2; exit 1; }
[[ "$(pnpm --version)" == 9.15.0 ]] || { echo '需要 pnpm@9.15.0' >&2; exit 1; }
[[ -f package.json && -f pnpm-lock.yaml ]] || { echo '缺少 package.json 或 pnpm-lock.yaml' >&2; exit 1; }
command -v python3 >/dev/null
pnpm install --frozen-lockfile
# 清除旧输出，避免旧构建被误打包；dist 必须是应用专用构建目录。
[[ ! -L dist ]] || { echo '拒绝删除符号链接 dist' >&2; exit 1; }
rm -rf -- dist
pnpm run build
[[ -f dist/index.html ]] || { echo '构建缺少 dist/index.html' >&2; exit 1; }
# 仅收集 dist 和生成的 manifest，绝不归档整个仓库。
python3 - "$TAG" <<'PYTHON'
import hashlib
import os
from pathlib import Path
import sys
import zipfile

tag = sys.argv[1]
output = Path('.upy-artifacts')
output.mkdir(exist_ok=True)
archive = output / 'bundle.zip'
manifest = f'name: demo-web\nversion: "{tag}"\ntype: static\nstatic:\n  directory: dist\n'
# 在输出目录内生成临时 ZIP，成功后才替换正式产物。
temporary = output / 'bundle.zip.tmp'
try:
    with zipfile.ZipFile(temporary, 'w', zipfile.ZIP_DEFLATED) as bundle:
        bundle.writestr('deploy.yaml', manifest)
        for path in sorted(Path('dist').rglob('*')):
            if path.is_symlink():
                raise SystemExit(f'拒绝归档符号链接: {path}')
            if path.is_file():
                bundle.write(path, path.as_posix())
    with zipfile.ZipFile(temporary) as bundle:
        assert bundle.testzip() is None, 'ZIP 校验失败'
        assert 'dist/index.html' in bundle.namelist(), 'ZIP 缺少入口'
        assert bundle.read('deploy.yaml').decode() == manifest
    os.replace(temporary, archive)
finally:
    temporary.unlink(missing_ok=True)
digest = hashlib.sha256(archive.read_bytes()).hexdigest()
(output / 'SHA256SUMS').write_text(f'{digest}  bundle.zip\n')
print(f'生成 {archive} 和 {output / "SHA256SUMS"}')
PYTHON
