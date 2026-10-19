#!/usr/bin/env bash
set -euo pipefail
# 在目标应用仓库内执行；版本与架构必须显式提供。
if [[ $# -ne 3 ]]; then
  echo "usage: $0 <version-without-v> <amd64|arm64> <output-directory>" >&2
  exit 2
fi
VERSION="$1"
ARCH="$2"
OUT="$3"
# 限制为无 build metadata 的 SemVer；预发布标识也不允许数字前导零。
python3 - "$VERSION" <<'CHECK'
import re, sys
v = sys.argv[1]
pattern = r'(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?'
m = re.fullmatch(pattern, v)
if not m or (m[4] and any(p.isdigit() and len(p)>1 and p[0]=='0' for p in m[4].split('.'))):
    sys.exit('version must be SemVer without v or build metadata')
CHECK
case "$ARCH" in amd64|arm64) ;; *) echo 'unsupported architecture' >&2; exit 2 ;; esac
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"
mkdir -p "$OUT"
OUT="$(cd "$OUT" && pwd)"
ASSET="$OUT/bundle-linux-$ARCH.zip"
if [[ -e "$ASSET" || -e "$ASSET.sha256" ]]; then
  echo 'refusing to overwrite existing assets' >&2
  exit 1
fi
# 每次使用独立 staging，绝不复制工作目录或环境文件。
STAGE="$(mktemp -d "${TMPDIR:-/tmp}/demo-api-bundle.XXXXXXXX")"
trap 'rm -rf "$STAGE"' EXIT
mkdir -p "$STAGE/bin"
CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" go build -mod=readonly -trimpath -o "$STAGE/bin/server" ./cmd/api
chmod 0755 "$STAGE/bin/server"
# 直接检查 ELF header，避免把宿主机 binary 或错架构产物打入包。
python3 - "$STAGE" "$VERSION" "$ARCH" <<'MANIFEST'
import json, pathlib, struct, sys
stage, version, arch = pathlib.Path(sys.argv[1]), sys.argv[2], sys.argv[3]
header = (stage/'bin/server').read_bytes()[:20]
if len(header)<20 or header[:6] != b'\x7fELF\x02\x01' or struct.unpack('<H', header[18:20])[0] != {'amd64':62,'arm64':183}[arch]:
    sys.exit('expected matching Linux 64-bit little-endian ELF')
manifest = {
    'name':'demo-api', 'version':version, 'type':'system',
    'target':{'os':'linux', 'arch':arch},
    'system':{'command':['./bin/server']},
    'healthcheck':{'url':'http://127.0.0.1:8080/health', 'timeout':'30s'},
}
(stage/'deploy.yaml').write_text(json.dumps(manifest, indent=2)+'\n')
MANIFEST
python3 scripts/upy/pack_bundle.py --stage "$STAGE" --output "$ASSET"
# 验证最终 ZIP 的白名单、manifest 和 executable 权限。
python3 - "$ASSET" "$ARCH" "$VERSION" <<'VERIFY'
import json, stat, sys, zipfile
with zipfile.ZipFile(sys.argv[1]) as z:
    assert z.testzip() is None
    assert sorted(z.namelist()) == ['bin/server', 'deploy.yaml']
    m = json.loads(z.read('deploy.yaml'))
    assert m['target'] == {'os':'linux','arch':sys.argv[2]}
    assert m['version'] == sys.argv[3]
    assert stat.S_IMODE(z.getinfo('bin/server').external_attr >> 16) == 0o755
VERIFY
