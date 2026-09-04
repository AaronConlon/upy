#!/usr/bin/env bash
#
# upy 一键安装与从 Releases 自动更新脚本
#
# 用法:
#   # 安装或更新到最新 Release 版本
#   curl -fsSL https://raw.githubusercontent.com/AaronConlon/upy/main/install.sh | bash
#
# 自定义安装目录、指定版本或强制重装:
#   INSTALL_DIR=~/.local/bin curl -fsSL ... | bash
#   VERSION=v0.1.0 curl -fsSL ... | bash
#   FORCE=1 curl -fsSL ... | bash
#
set -euo pipefail

REPO="AaronConlon/upy"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
FORCE="${FORCE:-0}"

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

# 1. 识别当前平台 OS 与架构
OS_RAW="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS_RAW" in
  darwin) OS="darwin" ;;
  linux) OS="linux" ;;
  *)
    log_err "暂不支持的操作系统: $OS_RAW (目前仅支持 Linux / macOS)"
    exit 1
    ;;
esac

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
TARGET_BIN="${INSTALL_DIR}/upy"

# 2. 检测本地当前已安装版本
CURRENT_VERSION=""
if [ -x "$TARGET_BIN" ]; then
  CURRENT_VERSION="$("$TARGET_BIN" --version 2>/dev/null | awk '{print $2}' || true)"
elif command -v upy >/dev/null 2>&1; then
  CURRENT_VERSION="$(upy --version 2>/dev/null | awk '{print $2}' || true)"
fi

log_info "运行环境: ${BOLD}${OS}-${ARCH}${RESET} (目标资产: ${ASSET_NAME})"
if [ -n "$CURRENT_VERSION" ]; then
  log_info "本地当前已安装版本: ${BOLD}${CURRENT_VERSION}${RESET}"
fi

# 3. 从 GitHub Releases 解析最新版本
TARGET_VERSION="${VERSION:-}"
if [ -z "$TARGET_VERSION" ]; then
  log_step "正在从 GitHub Releases 检查最新发布版本..."
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
  log_err "无法从 Releases 获取版本号（请检查网络，或使用 VERSION=vX.Y.Z 指定）。"
  exit 1
fi

log_info "Releases 目标版本: ${BOLD}${TARGET_VERSION}${RESET}"

# 若本地已安装且版本一致，且未开启强制覆盖，直接提示退出
if [ -n "$CURRENT_VERSION" ] && [ "$CURRENT_VERSION" = "$TARGET_VERSION" ] && [ "$FORCE" != "1" ]; then
  log_ok "当前已是 Releases 最新版本 (${CURRENT_VERSION})，无需更新。"
  log_info "如需强制重新下载安装，可指定 FORCE=1，例如: FORCE=1 ./install.sh"
  exit 0
fi

# 4. 从 GitHub Releases 下载对应架构二进制
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TARGET_VERSION}/${ASSET_NAME}"
TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t 'upy-install')"
trap 'rm -rf "$TMP_DIR"' EXIT
TMP_FILE="${TMP_DIR}/${ASSET_NAME}"

if [ -n "$CURRENT_VERSION" ]; then
  log_step "正在从 Releases 拉取新版本进行更新: ${CURRENT_VERSION} -> ${TARGET_VERSION}..."
else
  log_step "正在从 Releases 下载 ${DOWNLOAD_URL}..."
fi

if ! curl -fL --progress-bar -o "$TMP_FILE" "$DOWNLOAD_URL"; then
  log_err "下载失败: ${DOWNLOAD_URL}"
  exit 1
fi

chmod +x "$TMP_FILE"

# 5. 安装/替换到目标目录
SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  if [ ! -w "$INSTALL_DIR" ] || [ ! -d "$INSTALL_DIR" ]; then
    if command -v sudo >/dev/null 2>&1; then
      SUDO="sudo"
      log_info "需要管理员权限将 upy 写入 ${INSTALL_DIR}"
    else
      log_err "无权限写入 ${INSTALL_DIR}，且未找到 sudo 命令"
      exit 1
    fi
  fi
fi

if [ ! -d "$INSTALL_DIR" ]; then
  $SUDO mkdir -p "$INSTALL_DIR"
fi

log_step "正在写入 ${TARGET_BIN}..."
$SUDO mv "$TMP_FILE" "$TARGET_BIN"
$SUDO chmod 755 "$TARGET_BIN"

# 6. 验证安装/更新结果
if [ -n "$CURRENT_VERSION" ]; then
  log_ok "upy 已成功从 Releases 更新至 ${TARGET_VERSION}！"
else
  log_ok "upy ${TARGET_VERSION} 已成功从 Releases 安装完成！"
fi

if command -v upy >/dev/null 2>&1; then
  INSTALLED_PATH="$(command -v upy)"
  log_info "可执行文件路径: ${INSTALLED_PATH}"
else
  log_info "提示: ${INSTALL_DIR} 可能尚未加入系统 PATH，请将其加入您的 shell 配置文件 (如 ~/.zshrc 或 ~/.bashrc):"
  printf "    export PATH=\"%s:\$PATH\"\n" "$INSTALL_DIR"
fi

