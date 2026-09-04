#!/usr/bin/env bash
#
# upy 一键安装脚本
# 用法:
#   curl -fsSL https://raw.githubusercontent.com/AaronConlon/upy/main/install.sh | bash
# 自定义安装目录或版本:
#   INSTALL_DIR=/usr/local/bin VERSION=v0.1.0 curl -fsSL ... | bash
#
set -euo pipefail

REPO="AaronConlon/upy"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# 颜色输出
if [ -t 1 ]; then
  RED=$'\033[0;31m'
  GREEN=$'\033[0;32m'
  YELLOW=$'\033[0;33m'
  CYAN=$'\033[0;36m'
  BOLD=$'\033[1m'
  RESET=$'\033[0m'
else
  RED=''
  GREEN=''
  YELLOW=''
  CYAN=''
  BOLD=''
  RESET=''
fi

log_info() {
  printf "%bℹ%b %s\n" "$CYAN" "$RESET" "$*"
}

log_step() {
  printf "%b➜%b %s\n" "$YELLOW" "$RESET" "$*"
}

log_ok() {
  printf "%b✓%b %b%s%b\n" "$GREEN" "$RESET" "$BOLD" "$*" "$RESET"
}

log_err() {
  printf "%b✗ %s%b\n" "$RED" "$*" "$RESET" >&2
}

# 1. 识别操作系统
OS_RAW="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS_RAW" in
  darwin) OS="darwin" ;;
  linux) OS="linux" ;;
  *)
    log_err "暂不支持的操作系统: $OS_RAW (目前仅支持 Linux / macOS)"
    exit 1
    ;;
esac

# 2. 识别 CPU 架构
ARCH_RAW="$(uname -m | tr '[:upper:]' '[:lower:]')"
case "$ARCH_RAW" in
  x86_64|amd64) ARCH="x64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    log_err "暂不支持的 CPU 架构: $ARCH_RAW (目前仅支持 x64 / arm64)"
    exit 1
    ;;
esac

ASSET_NAME="upy-${OS}-${ARCH}"
log_info "检测到平台: ${BOLD}${OS}-${ARCH}${RESET} (目标资产: ${ASSET_NAME})"

# 3. 解析版本
TARGET_VERSION="${VERSION:-}"
if [ -z "$TARGET_VERSION" ]; then
  log_step "正在从 GitHub 获取最新版本..."
  # 优先通过 release redirect 探测, 避免 API 速率限制
  LATEST_URL="$(curl -fsSLI -o /dev/null -w "%{url_effective}" "https://github.com/${REPO}/releases/latest" 2>/dev/null || true)"
  if [[ "$LATEST_URL" =~ /releases/tag/(v?[0-9a-zA-Z.-]+) ]]; then
    TARGET_VERSION="${BASH_REMATCH[1]}"
  else
    # 备选: GitHub API 获取
    API_URL="https://api.github.com/repos/${REPO}/releases/latest"
    AUTH_HEADER=()
    if [ -n "${GITHUB_TOKEN:-}" ]; then
      AUTH_HEADER=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
    elif [ -n "${DEPLOY_GITHUB_TOKEN:-}" ]; then
      AUTH_HEADER=(-H "Authorization: Bearer ${DEPLOY_GITHUB_TOKEN}")
    fi
    TARGET_VERSION="$(curl -fsSL "${AUTH_HEADER[@]}" "$API_URL" 2>/dev/null | grep -m1 '"tag_name":' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/' || true)"
  fi
fi

if [ -z "$TARGET_VERSION" ]; then
  log_err "无法获取最新版本号（可能仓库尚未发布任何 Release，或请使用 VERSION=vX.Y.Z 手动指定）。"
  exit 1
fi

log_info "目标版本: ${BOLD}${TARGET_VERSION}${RESET}"

# 4. 下载对应架构二进制
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TARGET_VERSION}/${ASSET_NAME}"
TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t 'upy-install')"
trap 'rm -rf "$TMP_DIR"' EXIT
TMP_FILE="${TMP_DIR}/${ASSET_NAME}"

log_step "正在下载 ${DOWNLOAD_URL}..."
if ! curl -fL --progress-bar -o "$TMP_FILE" "$DOWNLOAD_URL"; then
  log_err "下载失败: ${DOWNLOAD_URL}"
  exit 1
fi

chmod +x "$TMP_FILE"

# 5. 安装到目标目录
SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  if [ ! -w "$INSTALL_DIR" ] || [ ! -d "$INSTALL_DIR" ]; then
    if command -v sudo >/dev/null 2>&1; then
      SUDO="sudo"
      log_info "需要管理员权限将 upy 安装到 ${INSTALL_DIR}"
    else
      log_err "无权限写入 ${INSTALL_DIR}，且未找到 sudo 命令"
      exit 1
    fi
  fi
fi

if [ ! -d "$INSTALL_DIR" ]; then
  $SUDO mkdir -p "$INSTALL_DIR"
fi

TARGET_BIN="${INSTALL_DIR}/upy"
log_step "正在安装到 ${TARGET_BIN}..."
$SUDO mv "$TMP_FILE" "$TARGET_BIN"
$SUDO chmod 755 "$TARGET_BIN"

# 6. 验证
log_ok "upy ${TARGET_VERSION} 安装成功！"
if command -v upy >/dev/null 2>&1; then
  INSTALLED_PATH="$(command -v upy)"
  log_info "执行路径: ${INSTALLED_PATH}"
else
  log_info "提示: ${INSTALL_DIR} 可能尚未加入系统 PATH，请将其加入您的 shell 配置文件 (如 ~/.zshrc 或 ~/.bashrc):"
  printf "    export PATH=\"%s:\$PATH\"\n" "$INSTALL_DIR"
fi

