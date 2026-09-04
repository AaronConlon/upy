#!/usr/bin/env bash
# 本地构建当前平台二进制, 版本取环境变量 UPLY_VERSION 或 git 最近 tag
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${UPLY_VERSION:-$(git describe --tags --abbrev=0 2>/dev/null || echo dev)}"
GOOS="${GOOS:-$(go env GOOS)}"
GOARCH="${GOARCH:-$(go env GOARCH)}"

# 映射到 uply 资产名 (x64/arm64)
case "$GOARCH" in
  amd64) SUFFIX="$GOOS-x64" ;;
  arm64) SUFFIX="$GOOS-arm64" ;;
  *) echo "unsupported arch: $GOARCH" >&2; exit 1 ;;
esac

echo "building uply $VERSION for $SUFFIX..."
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X github.com/AaronConlon/uply/internal/version.Value=$VERSION" \
  -o "dist/uply-$SUFFIX" .
ls -lh "dist/uply-$SUFFIX"
