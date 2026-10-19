<p align="center">
  <img src="assets/upy-gopher-skateboard-logo.png" width="220" alt="踩着滑板的 Go Gopher，upy 项目 Logo">
</p>

<h1 align="center">upy</h1>

<p align="center">一个二进制，从 Release 直达生产。
<br>
<a href="https://aaronconlon.github.io/upy/">官网</a> · <a href="https://github.com/AaronConlon/upy/releases">下载</a> · <a href="#如何安装">开始使用</a>
</p>

Go 单二进制 Web 部署与运维工具（~8.5MB，零外部依赖，极低内存占用）。

支持 **极速部署**（GitHub Releases / 本地 bundle、SPA 原子软链切换、Docker 平滑启停与失败自动回滚）、**磁盘清理**（空间分析、活跃版本保护、缓存释放）与 **约定备份**（脚本探测、跨进程加锁、自动轮转），构筑简洁可靠的单机运维闭环。

---

## 目录

- [这是什么](#这是什么)
- [如何安装](#如何安装)
- [如何初始化](#如何初始化)
- [如何部署](#如何部署)
  - [1. 服务器端项目配置](#1-服务器端项目配置)
  - [2. 常用部署命令](#2-常用部署命令)
  - [3. bundle.zip 打包规范](#3-bundlezip-打包规范)
- [如何清理与维护](#如何清理与维护)
- [如何升级](#如何升级)
- [常用命令速查](#常用命令速查)

---

## 这是什么

`upy` 专为轻量级、低心智负担的 Web 应用发布流程而设计：

- **单二进制零依赖**：体积仅 ~8.5MB，开箱即用，无需 Node.js、Python 或任何额外运行时环境。
- **静态 SPA 原子部署**：通过 `current` 软链实现无感知秒级切换；自动规整目录与文件权限（`0755` / `0644`），彻底避免 Nginx / Caddy 出现 `403 Forbidden`。
- **Docker / Compose 容器管理**：新版本镜像准备就绪后，平滑停止旧栈释放端口，新栈健康检查不通过时自动回滚上一版本。
- **GitHub Release 驱动**：支持公开与私有仓库，多组织 Token 自动匹配路由，断点续传与 SHA-256 完整性强校验。
- **本地项目快速调度**：在任意路径直接运行 `upy`，即可列出本机已纳管的项目并交互式选择部署。
- **通知与观测**：内置全流程真实耗时统计与 Bark 移动端推送。

---

## 如何安装

在 Linux / macOS 终端执行以下一键安装脚本，自动识别系统架构（x64 / arm64）并安装到系统路径：

```bash
curl -fsSL https://raw.githubusercontent.com/AaronConlon/upy/main/install.sh | bash
```

> **说明**：默认安装至 `/usr/local/bin`（必要时自动请求 sudo 权限）。脚本会自动为 `zsh`、`bash`、`fish` 配置 Tab 命令补全，新开终端窗口后生效。

### 自定义安装选项

支持通过环境变量指定安装路径或固定版本：

```bash
# 安装到用户个人目录（无需 root 权限）
INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/AaronConlon/upy/main/install.sh | bash

# 安装指定版本
VERSION=v0.1.11 curl -fsSL https://raw.githubusercontent.com/AaronConlon/upy/main/install.sh | bash
```

---

## 如何初始化

如果你的项目托管在 **GitHub 私有仓库**，需要配置 GitHub Token 才能拉取 Release 资产。`upy` 支持全局默认 Token 以及按组织/个人归属的多 Token 自动路由。

### 1. 命令行快速配置

```bash
# 设置全局默认 Token
upy init ghp_your_default_token

# 为指定组织（如 MyOrg）设置专属 Token
upy init MyOrg ghp_org_token

# 为个人用户名（如 AaronConlon）设置专属 Token
upy init AaronConlon ghp_personal_token

# 交互式引导配置（密码密文输入）
upy init
```

配置会自动保存至 `~/.upy/config.yaml` 并将权限严格限制为 `600`。

### 2. 配置文件说明 (`~/.upy/config.yaml`)

可手动新建或修改配置文件，同时支持配置部署完成后的通知推送：

```yaml
github:
  # 全局默认兜底 Token（可选）
  token: ghp_default_xxx

  # 多组织 / 个人专属 Token 映射（可选，拉取对应仓库时优先使用）
  tokens:
    MyOrg: ghp_org_xxx
    AaronConlon: ghp_personal_xxx

notify:
  bark:
    - name: phone
      serverUrl: https://api.day.app
      token: your-bark-device-key
      group: upy
```

---

## 如何部署

### 1. 服务器端项目配置

在目标服务器的项目根目录（例如 `/var/www/my-app`）下创建一个 `deploy.yaml` 文件：

```yaml
name: my-app
release:
  provider: github
  repository: owner/repo      # GitHub 仓库路径 (组织/仓库名)
  asset: bundle.zip          # 资产文件名（可选，默认 bundle.zip，支持正则/模糊匹配）
```

> **Nginx 配置提示（SPA 静态站点）**：  
> 将 Nginx 的 `root` 指向项目根目录下的 `current` 软链即可，例如：`root /var/www/my-app/current;`。每次发布新版本，`upy` 会自动原子切换该软链。

### 2. 常用部署命令

在服务器项目根目录下执行：

```bash
# 1. 交互式选择要部署的 Release 版本（推荐）
upy release

# 2. 直接部署最新正式版本
upy release latest

# 3. 部署指定版本
upy release v1.0.0

# 4. 查看仓库的 Release 发布历史
upy release list

# 5. 强制重新下载并部署（即使当前已是该版本）
upy release latest --force
```

#### 跨项目与全局快捷操作

一旦在某个目录成功执行过一次部署，`upy` 会自动登记该项目。之后你可以在**服务器的任何路径**快速操作：

```bash
# 在任意目录输入 upy，直接交互选择已登记的项目并部署
upy

# 快捷为指定项目部署最新版本
upy deploy my-app
```

#### 本地离线包部署

如果服务器无法连通外网 GitHub，也可以直接部署本地传入的 zip 压缩包：

```bash
upy bundle ./bundle.zip
```

---

### 3. bundle.zip 打包规范

在 CI/CD（如 GitHub Actions）构建产物并打包发布时，压缩包内部需包含应用文件以及一份描述清单 `deploy.yaml`：

#### A. 前端静态应用（SPA）

压缩包内结构：
```text
bundle.zip
├── deploy.yaml
└── dist/
    ├── index.html
    ├── assets/
    └── ...
```

清单 `deploy.yaml` 格式：
```yaml
name: my-spa-app
version: 1.0.0
type: static
static:
  directory: dist   # 静态资源所在目录，默认 dist
```

#### B. Docker / Compose 容器项目

压缩包内结构：
```text
bundle.zip
├── deploy.yaml
├── docker-compose.yml
└── ...
```

清单 `deploy.yaml` 格式：
```yaml
name: my-api-service
version: 1.0.0
type: docker
docker:
  mode: compose
  composeFile: docker-compose.yml
healthcheck:
  url: http://localhost:8080/health
  timeout: 30s      # 健康检查超时，支持 30s 或数字秒
```

---

## 如何清理与维护

随着持续迭代部署，项目目录下会产生历史 Bundle 压缩包与历史发布版本。通过 `upy clean` 可以随时进行可视化磁盘分析与安全清理：

```bash
# 1. 交互式选择项目与清理内容（推荐，自动分析磁盘占用）
upy clean

# 2. 清理指定项目的数据
upy clean my-app

# 3. 仅清理下载的 Bundle 缓存压缩包
upy clean --bundles

# 4. 仅清理历史非当前版本与中断残留目录
upy clean --old-releases

# 5. 一键常规瘦身并跳过确认（清理缓存 + 旧版本 + 临时目录，安全保留当前生效版本）
upy clean --all -y
```

> **安全保护**：`upy clean` 受到**活跃版本保护机制**约束，默认不会影响 `current` 软链所指向的当前线上服务。若需彻底重置项目，可在交互菜单中选择“彻底清空部署产物”。

---

## 如何升级

`upy` 拥有完善的跨分区自更新与在运行进程替换机制，随时保持工具最新：

```bash
# 检查并自动升级到最新版本
upy update

# 更新或回滚到指定版本
upy update v0.1.11

# 强制重新下载并覆盖更新
upy update --force

# 查看当前运行版本及环境
upy version
```

---

## 常用命令速查

| 命令 | 说明 |
| :--- | :--- |
| `upy` | 在任意目录唤起已登记项目列表，交互选择项目并部署 |
| `upy release [latest\|<版本>]` | 在项目目录下从 GitHub Release 拉取并部署（缺省参数时交互选版） |
| `upy release list` | 查看当前项目的 GitHub Release 版本历史与资产信息 |
| `upy deploy <项目名>` | 快捷为已登记的本地项目部署最新正式版本 |
| `upy clean [项目名]` | 可视化分析并清理项目数据 (Bundle 缓存、旧版本、临时目录或注销) |
| `upy bundle <文件>` | 直接部署本地 zip 压缩包（离线模式，不访问 GitHub） |
| `upy init [owner] <token>` | 初始化或添加 GitHub 访问令牌（支持组织/个人专属） |
| `upy update [版本]` | 自更新到最新版本或切换至指定版本 |
| `upy version` | 打印当前版本并检查是否有新版本可用 |

---

## 开源协议

MIT License

## 官网维护

官网源码位于 `website/`，使用原生 HTML、CSS 和 JavaScript。共享设计变量位于根目录 `tokens.css`，Logo 位于 `assets/upy-gopher-skateboard-logo.png`；发布时自动同步到站点目录。

推送官网相关文件到 `main` 后，`.github/workflows/pages.yml` 会自动部署到 [GitHub Pages](https://aaronconlon.github.io/upy/)，也可以在 Actions 中手动运行。仓库 Pages 的发布来源为 GitHub Actions。

本地预览：在仓库根目录运行 `python3 -m http.server 8080 --directory website`，然后打开 `http://localhost:8080`。
