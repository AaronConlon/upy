#!/usr/bin/env bash
#
# upy 一键安装与从 Releases 自动更新脚本（支持公开与私有仓库）
#
# 用法:
#   # 安装或更新到最新 Release 版本
#   curl -fsSL https://raw.githubusercontent.com/AaronConlon/upy/main/install.sh | bash
#
# 自定义安装目录、指定版本或强制重装:
#   INSTALL_DIR=~/.local/bin curl -fsSL ... | bash
#   VERSION=v0.1.1 curl -fsSL ... | bash
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

# 2. 读取 Token（支持环境变量与 ~/.upy/config.yaml 自动发现）
AUTH_TOKEN="${GITHUB_TOKEN:-}"
if [ -z "$AUTH_TOKEN" ]; then
  AUTH_TOKEN="${DEPLOY_GITHUB_TOKEN:-}"
fi
if [ -z "$AUTH_TOKEN" ]; then
  CONFIG_FILE=""
  if [ -f "$HOME/.upy/config.yaml" ]; then
    CONFIG_FILE="$HOME/.upy/config.yaml"
  elif [ -f "$HOME/.uply/config.yaml" ]; then
    CONFIG_FILE="$HOME/.uply/config.yaml"
  fi
  if [ -n "$CONFIG_FILE" ]; then
    # 优先提取 AaronConlon 专属 token，其次提取全局 token
    OWNER_TOKEN="$(grep -A 10 "tokens:" "$CONFIG_FILE" 2>/dev/null | grep -iE "^\s+AaronConlon:" | awk '{print $2}' || true)"
    if [ -n "$OWNER_TOKEN" ]; then
      AUTH_TOKEN="$OWNER_TOKEN"
    else
      GLOBAL_TOKEN="$(grep -E "^\s+token:" "$CONFIG_FILE" | head -n1 | awk '{print $2}' || true)"
      if [ -n "$GLOBAL_TOKEN" ]; then
        AUTH_TOKEN="$GLOBAL_TOKEN"
      fi
    fi
  fi
fi

# 3. 检测本地当前已安装版本
CURRENT_VERSION=""
if [ -x "$TARGET_BIN" ]; then
  CURRENT_VERSION="$("$TARGET_BIN" --version 2>/dev/null | awk '{print $2}' || true)"
elif command -v upy >/dev/null 2>&1; then
  CURRENT_VERSION="$(upy --version 2>/dev/null | awk '{print $2}' || true)"
fi

log_info "运行环境: ${BOLD}${OS}-${ARCH}${RESET} (目标资产: ${ASSET_NAME})"
if [ -n "$CURRENT_VERSION" ]; then
  log_info "本地当前版本: ${BOLD}${CURRENT_VERSION}${RESET}"
fi

# 4. 从 GitHub Releases 解析最新发布版本及资产信息
TARGET_VERSION="${VERSION:-}"
AUTH_HEADER=()
if [ -n "$AUTH_TOKEN" ]; then
  AUTH_HEADER=(-H "Authorization: Bearer ${AUTH_TOKEN}")
fi

ASSET_ID=""
if [ -z "$TARGET_VERSION" ]; then
  log_step "正在从 GitHub Releases 检查最新发布版本..."
  API_URL="https://api.github.com/repos/${REPO}/releases/latest"
  RELEASE_JSON="$(curl -fsSL "${AUTH_HEADER[@]}" -H "Accept: application/vnd.github+json" "$API_URL" 2>/dev/null || true)"
  TARGET_VERSION="$(printf '%s' "$RELEASE_JSON" | grep -m1 '"tag_name":' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/' || true)"

  # 备选: Release 重定向探测 (公开仓库兜底)
  if [ -z "$TARGET_VERSION" ]; then
    LATEST_URL="$(curl -fsSLI -o /dev/null -w "%{url_effective}" "https://github.com/${REPO}/releases/latest" 2>/dev/null || true)"
    if [[ "$LATEST_URL" =~ /releases/tag/(v?[0-9a-zA-Z.-]+) ]]; then
      TARGET_VERSION="${BASH_REMATCH[1]}"
    fi
  fi
else
  API_URL="https://api.github.com/repos/${REPO}/releases/tags/${TARGET_VERSION}"
  RELEASE_JSON="$(curl -fsSL "${AUTH_HEADER[@]}" -H "Accept: application/vnd.github+json" "$API_URL" 2>/dev/null || true)"
fi

if [ -z "$TARGET_VERSION" ]; then
  log_err "无法从 Releases 获取版本号。若仓库为私有仓库，请设置 GITHUB_TOKEN 或运行 upy init 配置访问令牌。"
  exit 1
fi

log_info "Releases 目标版本: ${BOLD}${TARGET_VERSION}${RESET}"

# 若本地已安装且版本一致，未开启强制覆盖，直接提示退出
if [ -n "$CURRENT_VERSION" ] && [ "$CURRENT_VERSION" = "$TARGET_VERSION" ] && [ "$FORCE" != "1" ]; then
  log_ok "当前已是 Releases 目标版本 (${CURRENT_VERSION})，无需更新。"
  if [ -x "$TARGET_BIN" ]; then
    "$TARGET_BIN" __install-completion || log_err "Tab 命令补全安装失败，但 upy 可正常使用。"
  fi
  log_info "如需强制重新下载安装，可传 FORCE=1，例如: FORCE=1 ./install.sh"
  exit 0
fi

# 提取资产 ID (用于私有仓库走 API 端点下载)
if [ -n "${RELEASE_JSON:-}" ]; then
  ASSET_ID="$(printf '%s' "$RELEASE_JSON" | grep -B 2 -A 5 "\"name\": *\"${ASSET_NAME}\"" | grep -m1 '"id":' | tr -dc '0-9' || true)"
fi

# 5. 下载对应架构二进制
TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t 'upy-install')"
trap 'rm -rf "$TMP_DIR"' EXIT
TMP_FILE="${TMP_DIR}/${ASSET_NAME}"

if [ -n "$CURRENT_VERSION" ]; then
  log_step "正在从 Releases 拉取最新资产更新: ${CURRENT_VERSION} -> ${TARGET_VERSION}..."
else
  log_step "正在从 Releases 下载 ${ASSET_NAME} (${TARGET_VERSION})..."
fi

DOWNLOAD_SUCCESS=0
# 若有 Token 且获取到了 ASSET_ID，优先使用私有 API 端点下载 (私有仓库必需)
if [ -n "$AUTH_TOKEN" ] && [ -n "$ASSET_ID" ]; then
  API_ASSET_URL="https://api.github.com/repos/${REPO}/releases/assets/${ASSET_ID}"
  if curl -fL --progress-bar -H "Authorization: Bearer ${AUTH_TOKEN}" -H "Accept: application/octet-stream" -o "$TMP_FILE" "$API_ASSET_URL"; then
    DOWNLOAD_SUCCESS=1
  fi
fi

# 备选: 常规公开 release 资产直链
if [ "$DOWNLOAD_SUCCESS" -ne 1 ]; then
  PUBLIC_DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TARGET_VERSION}/${ASSET_NAME}"
  if curl -fL --progress-bar "${AUTH_HEADER[@]}" -o "$TMP_FILE" "$PUBLIC_DOWNLOAD_URL"; then
    DOWNLOAD_SUCCESS=1
  fi
fi

if [ "$DOWNLOAD_SUCCESS" -ne 1 ]; then
  log_err "资产下载失败。若仓库为私有，请确认 Token 是否具有目标仓库的 Contents 读取权限。"
  exit 1
fi

chmod +x "$TMP_FILE"

# 6. 安装/更新到目标目录
SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  if [ ! -w "$INSTALL_DIR" ] || [ ! -d "$INSTALL_DIR" ]; then
    if command -v sudo >/dev/null 2>&1; then
      SUDO="sudo"
      log_info "需要管理员权限写入 ${INSTALL_DIR}"
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

# 7. 验证结果
if [ -n "$CURRENT_VERSION" ]; then
  log_ok "upy 已成功更新至 ${TARGET_VERSION}！"
else
  log_ok "upy ${TARGET_VERSION} 已成功安装完成！"
fi

log_step "正在配置 Tab 命令补全..."
if ! "$TARGET_BIN" __install-completion; then
  log_err "Tab 命令补全安装失败，但 upy 已可正常使用。"
fi

if command -v upy >/dev/null 2>&1; then
  INSTALLED_PATH="$(command -v upy)"
  log_info "执行文件路径: ${INSTALLED_PATH}"
else
  log_info "提示: ${INSTALL_DIR} 可能尚未加入系统 PATH，请将其加入您的 shell 配置文件 (如 ~/.zshrc 或 ~/.bashrc):"
  printf "    export PATH=\"%s:\$PATH\"\n" "$INSTALL_DIR"
fi
