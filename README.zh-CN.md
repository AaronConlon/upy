<p align="center">
  <img src="assets/upy-gopher-skateboard-logo.png" width="220" alt="踩着滑板的 Go Gopher，upy 项目 Logo">
</p>

<h1 align="center">upy</h1>

<p align="center">
一个二进制，从 Release 直达生产。
<br>
<a href="https://aaronconlon.github.io/upy/zh.html">官网</a> · <a href="https://github.com/AaronConlon/upy/releases">下载</a> · <a href="#如何安装">开始使用</a>
<br><br>
<a href="README.md">English</a> · <b>简体中文</b>
</p>

Go 单二进制 Web 部署与运维工具（~8.5MB，零外部依赖，极低内存占用）。

`upy` 专为轻量级、低心智负担的单机 Web 生产环境设计，深度原生支持**三大部署模式**：
1. **SPA 静态站点**：前端单页应用（Vue / React / Svelte 等），解压至独立版本目录并通过 `current` 软链秒级原子切换，自动规整目录与文件权限（`0755` / `0644`），彻底杜绝 Nginx / Caddy 出现 `403 Forbidden`。
2. **Docker / Compose 容器应用**：新镜像拉取与配置就绪后，平滑停止旧栈释放端口，内置 HTTP 存活性探针，启动异常时自动回滚至上一可用版本。
3. **Systemd 原生服务（Go 二进制 Web 服务）**：采用标准轻量生产架构（upy 管版本与发布 $\longleftrightarrow$ systemd 管守护与资源 $\longleftrightarrow$ Nginx/Caddy 管公网反代），支持不可变版本归档、启动参数原样保真、运行账号权限严格预检（拒绝静默提权）、端口身份防伪与失败秒级无损回滚。

配合内置的**磁盘瘦身与活跃版本保护**（`upy clean`）、**跨进程项目排他锁**与**约定备份**，构筑简洁可靠的单机运维闭环。

---

## 目录

- [三大部署方式](#三大部署方式)
- [核心特性](#核心特性)
- [如何安装](#如何安装)
- [如何初始化](#如何初始化)
- [如何部署](#如何部署)
  - [1. 服务器端项目配置](#1-服务器端项目配置)
  - [2. 常用部署命令](#2-常用部署命令)
  - [3. 三大项目打包规范 (bundle.zip)](#3-三大项目打包规范-bundlezip)
    - [A. 前端静态应用 (SPA)](#a-前端静态应用-spa)
    - [B. Docker / Compose 容器项目](#b-docker--compose-容器项目)
    - [C. 原生系统服务 (system / Go 二进制 Web 服务)](#c-原生系统服务-system--go-二进制-web-服务)
- [如何清理与维护](#如何清理与维护)
- [如何备份与归档](#如何备份与归档)
- [客户端项目与磁盘分析 (upy analysis)](#客户端项目与磁盘分析-upy-analysis)
- [通知服务 (Bark 移动端推送 & 项目专属 Webhook)](#-通知服务-bark-移动端推送--项目专属-webhook)
- [Server 服务与 Webhook 自动化](#server-服务与-webhook-自动化)
- [日志系统与 upy log 命令](#日志系统与-upy-log-命令)
- [如何升级](#如何升级)
- [常用命令速查](#常用命令速查)
- [Agent Skill](#agent-skill)
- [开源协议](#开源协议)

---

## 三大部署方式

| 部署模式 | 适用场景 | 架构与核心保障 | 失败回滚机制 |
| :--- | :--- | :--- | :--- |
| **SPA 静态站点**<br>`type: static` | Vue / React / Vite / 静态 HTML | 部署至 `releases/<version>`，切换 `current` 软链；自动修复 `0755/0644` 权限 | 软链秒级指回旧版本目录 |
| **Docker 容器**<br>`type: docker` | 容器化 Web 应用 / Docker Compose | 新镜像就绪后停旧启新；平滑释放宿主端口冲突 | 自动拉起上一可用镜像与配置 |
| **Systemd 原生服务**<br>`type: system` | Go / Rust / 二进制 Web 服务 | upy 生成并更新 systemd unit；不可变版本隔离；环境变量与持久化数据放入 `shared/`；运行账号强校验防 root 提权 | 停止失败新服务，还原旧 unit 并拉起旧进程，执行真实 HTTP 存活验证 |

---

## 核心特性

- **单二进制零依赖**：体积仅 ~8.5MB，开箱即用，无需 Node.js、Python、Docker 宿主绑定或额外运行时环境。
- **三大部署模式原生覆盖**：无论静态前端、容器微服务还是原生 Go 二进制，使用完全一致的 CLI 交互与生命周期规范。
- **GitHub Release 驱动**：支持公开与私有仓库，多组织/个人 Token 自动路由匹配，断点续传与 SHA-256 完整性强校验。
- **多重安全防线**：静态准备阶段与激活回滚正交隔离，跨根目录 unit 归属校验，外部已有 unit 防串扰保护。
- **磁盘分析与安全清理**：`upy clean` 支持空间分析、Bundle 缓存清理与旧版本安全裁剪，双版本保护（当前活跃与最近回滚版本）与 live unit 引用保护。
- **全机项目与磁盘分析**：`upy analysis` 实时盘点全机部署项目，以最大占用项目为 100% 基准绘制直观条形图，置顶聚焦最近一次部署动态，支持 `--detail` 深度展开与 `--json` 数据导出。
- **约定式数据备份**：零配置项目自动探测（`backup.sh`）、跨进程互斥锁防护、智能多版本轮转与全量一键批处理。
- **本地项目快速调度**：在任意路径直接运行 `upy`，即可列出本机已纳管的项目并交互式选择部署。
- **多接收人 Bark 移动端推送与项目专属 Webhook**：部署与备份结果即时推送至 iOS 设备与团队业务系统。底层基于 [Bark](https://bark.day.app/) 移动端即时推送与项目专属 Webhook（自带 HMAC-SHA256 签名校验，支持企微、飞书、钉钉机器人）。支持全局/项目双层配置、多人广播、智能去重与跨量级智能耗时展示（秒、分、时）。

---

## 如何安装

在 Linux / macOS 终端执行以下一键安装脚本，自动识别系统架构（x64 / arm64）并安装到系统路径：

```bash
curl -fsSL https://raw.githubusercontent.com/AaronConlon/upy/main/install.sh | bash -s -- --lang zh-CN
```

> **说明**：默认安装至 `/usr/local/bin`（必要时自动请求 sudo 权限）。脚本会自动为 `zsh`、`bash`、`fish` 配置 Tab 命令补全，新开终端窗口后生效。

### 自定义安装选项

支持通过环境变量指定安装路径或固定版本：

```bash
# 安装到用户个人目录（无需 root 权限）
curl -fsSL https://raw.githubusercontent.com/AaronConlon/upy/main/install.sh | INSTALL_DIR=~/.local/bin bash -s -- --lang zh-CN

# 安装指定版本
curl -fsSL https://raw.githubusercontent.com/AaronConlon/upy/main/install.sh | VERSION=v0.2.0 bash -s -- --lang zh-CN
```

---

### CLI 语言

CLI 默认使用英文。在 `~/.upy/config.yaml`（或 `UPY_CONFIG` 指定的文件）中设置：

```yaml
language: zh-CN # 支持 en、zh-CN，默认 en
```

安装时使用 `--lang en` 或 `--lang zh-CN`，会同时设置安装提示和后续 CLI 的语言。同版本重新安装也会保存语言选择，并保留已有 Token、server 设置和项目注册。单次运行可用 `UPY_LANG=zh-CN upy analysis` 覆盖配置。本次覆盖 CLI 与安装脚本，Web 控制台、通知、外部命令原始输出和 JSON 字段保持不变。

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
  enabled: true # 可选总开关 (默认开启)
  bark:
    # 格式 1: 支持多人接收设备 Token 或完整 Bark 推送 URL 列表
    - your-bark-device-key-1
    - https://api.day.app/your-bark-device-key-2

    # 格式 2: 完整配置对象 (支持自定义音效、推送分组或自建 Bark 服务)
    - name: 运维值班
      serverUrl: https://api.day.app
      token: on-call-device-key
      group: upy.production
      sound: chime
```

---

## 如何部署

### 1. 服务器端项目配置

在目标服务器的项目根目录（例如 `/var/www/my-app` 或 `/opt/my-api`）下创建一个 `deploy.yaml` 文件：

```yaml
name: my-app
release:
  provider: github
  repository: owner/repo      # GitHub 仓库路径 (组织/仓库名)
  asset: bundle.zip          # 资产文件名（可选，默认 bundle.zip，支持正则/模糊匹配）

# 可选: Bundle 压缩包保留策略 (默认最多保留 3 个，支持设置范围 0~100)
bundles:
  keep: 3

# 可选: 历史版本目录保留策略 (清理逻辑默认保留当前与上一版本)
releases:
  keep: 1

# 可选: 项目级独立配置 Bark 通知接收人 (与全局 ~/.upy/config.yaml 智能合并并自动去重)
notify:
  bark:
    - alice-device-key
    - https://api.day.app/bob-device-key
    - name: 核心架构组
      token: arch-device-key
      sound: bell

# 若为 system 类型部署，可额外指定系统运行账号（可选，防提权）
service:
  user: my-app
  group: my-app
  unit: upy-my-app.service
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

### 3. 三大项目打包规范 (bundle.zip)

在 CI/CD（如 GitHub Actions）构建产物并打包发布时，压缩包内部需包含应用文件以及一份描述清单 `upy.yaml`：

> 💡 **包内清单与服务器部署配置解耦**：  
> 为了与服务器项目根目录的配置 `deploy.yaml`（负责 Release 源、系统服务账号等部署策略）严格解耦，Bundle 压缩包内的清单文件统一推荐命名为 `upy.yaml`（`upy` 部署引擎仍向前兼容历史 `deploy.yaml`）。两者职责分明，彻底杜绝同名文件覆盖与概念混淆。

#### A. 前端静态应用 (SPA)

压缩包内结构：
```text
bundle.zip
├── upy.yaml
└── dist/
    ├── index.html
    ├── assets/
    └── ...
```

清单 `upy.yaml` 格式：
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
├── upy.yaml
├── docker-compose.yml
└── ...
```

清单 `upy.yaml` 格式：
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

#### C. 原生系统服务 (system / Go 二进制 Web 服务)

适用于轻量级 Go/Rust/Node 等单二进制或本地常驻 Web 服务。采用标准轻量生产架构：**upy 管版本与发布，systemd 管进程守护与 cgroups，Nginx/Caddy 管公网入口与 SSL**。

压缩包内结构：
```text
bundle.zip
├── upy.yaml
└── bin/
    └── my-api
```

清单 `upy.yaml` 格式：
```yaml
name: my-api
version: 1.0.0
type: system             # 原生系统服务（支持 service 别名）
target:
  os: linux
  arch: amd64            # 平台与硬件架构严格校验
system:
  command: ["./bin/my-api"]  # 启动命令及参数列表（原样保真传递）
  workingDir: ""             # 可选，默认指向当前版本目录
healthcheck:
  url: http://127.0.0.1:8080/health
  timeout: 30s               # 健康检查超时（单次请求 3s 保护）
```

> **持久化与数据隔离规范**：  
> 在服务器项目目录下通过 `/opt/my-api/shared/service.env` 提供环境变量（如 `LISTEN_ADDR=127.0.0.1:8080` 与 `DATA_DIR=/opt/my-api/shared/data`），程序持久化业务数据写入 `shared/data/`。版本目录 `releases/<version>` 保持纯净与不可变，支持秒级无损回滚。

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

> **安全保护机制**：  
> - **双版本保护**：`upy clean` 受到**活跃版本保护机制**约束，同时保护当前生效版本（`CurrentVersion`）与最近一个上一成功版本（`PreviousVersion`），确保随时可安全切回。
> - **Live Unit 引用保护**：若部署曾被外部异常中断导致 unit 已切换但 state 尚未记录，clean 自动动态解析 live systemd unit 引用，严防误删。
> - **Purge 彻底注销**：若需卸载整个服务，使用 `upy clean --purge` 彻底停止服务、注销 unit 并清理产物，同时安全保留 `shared/` 业务持久化数据。

---

## 如何备份与归档

在数据库迁移、大版本发布、环境变更或日常维护前，确保生产数据与状态的基线快照备份至关重要。各类技术栈（MySQL、PostgreSQL、SQLite、Redis、挂载卷及静态媒体文件）的数据备份方式差异极大，`upy` 遵循**约定优先（Convention over Configuration）**原则：由项目自身提供特定场景的备份脚本，由 `upy` 统一驱动、注入上下文变量、跨进程排他锁防护、产物轮转归档与通知推送。

### 1. 约定式自动探测（零配置）

`upy backup` 在项目根目录下自动按以下优先级寻找备份脚本并执行：
1. `<项目根目录>/backup.sh`
2. `<项目根目录>/scripts/backup.sh`
3. `<项目根目录>/bin/backup.sh`

*(亦可在 `deploy.yaml` 中通过 `backup.script` 显式指定脚本路径)*。

### 2. `upy` 注入给备份脚本的环境变量

执行备份时，`upy` 会自动注入标准化环境变量，脚本内部直接读取，无需硬编码服务器路径：

| 环境变量名 | 说明 | 示例 |
| :--- | :--- | :--- |
| `UPY_PROJECT_NAME` | 项目名称 | `my-api-service` |
| `UPY_PROJECT_ROOT` | 项目根目录绝对路径 | `/opt/my-api-service` |
| `UPY_BACKUP_DIR` | 建议备份产物存放目录绝对路径 | `/opt/my-api-service/backups` |
| `UPY_CURRENT_VERSION` | 当前生效运行的版本号 | `v1.2.0` |
| `UPY_TIMESTAMP` | 标准格式时间戳 | `20261019-120000` |
| `UPY_DEPLOY_TYPE` | 项目部署类型 | `docker`、`system` 或 `static` |

### 3. 典型 `backup.sh` 示例（Docker Compose + MySQL 导出）

```bash
#!/usr/bin/env bash
set -euo pipefail

# 确保目标输出目录就绪
mkdir -p "${UPY_BACKUP_DIR}"

BACKUP_FILE="${UPY_BACKUP_DIR}/dump-${UPY_PROJECT_NAME}-${UPY_TIMESTAMP}.sql.gz"
echo "正在导出数据库到: ${BACKUP_FILE}..."

# 在容器内安全执行 mysqldump 导出并压缩
docker compose exec -T db mysqldump -u root -p"${DB_PASSWORD}" "${DB_NAME}" | gzip > "${BACKUP_FILE}"

echo "数据库导出成功！"
```

### 4. `deploy.yaml` 进阶配置（可选）

```yaml
name: my-app
type: docker
docker:
  mode: compose
  composeFile: docker-compose.yml

# 压缩包与历史版本保留策略（可选）
bundles:
  keep: 3                      # 保留最近的 Bundle 压缩包数量 (默认 3，支持 0~100)
releases:
  keep: 1                      # 历史版本保留份数 (默认保留当前与上一版本)

# 备份功能配置（可选）
backup:
  script: backup.sh            # 自定义脚本路径（相对项目根目录，缺省自动探测）
  dir: backups                 # 备份产物归档目录（相对项目根目录，默认 backups）
  retention: 7                 # 自动轮转保留的最大备份份数（默认 7，0 表示不自动清理）
  timeout: 15m                 # 备份最大超时时间（默认 15m，支持 30s, 5m, 1h）
```

### 5. 常用备份命令

```bash
# 1. 交互式选择已登记项目执行备份
upy backup

# 2. 直接针对指定项目执行备份
upy backup my-app

# 3. 查看项目历史备份文件列表与占用体积
upy backup my-app --list

# 4. 手动触发备份轮转，保留最近 N 个备份
upy backup my-app --prune 5

# 5. 一键批量备份所有已登记项目
upy backup --all -y
```

> **安全保护机制**：  
> - **跨进程排他锁（`.deploy.lock`）**：`upy backup` 执行时自动获取项目排他锁（`action: 备份中`），严格互斥正在进行的部署（`upy release`）或磁盘清理（`upy clean`），杜绝备份产生半更新脏数据。  
> - **自动轮转防爆盘**：内置基于修改时间的智能轮转清理，自动保留最近 N 份备份（默认 7 份），防止历史备份文件无限膨胀撑爆服务器磁盘。  
---

## 客户端项目与磁盘分析 (upy analysis)

随着服务器纳管的应用与历史发布逐渐增多，运维需要一个全局直观的视角洞察各项目占用的磁盘空间，并快速追踪最新部署状态。`upy analysis` 提供工业级排版的终端可视化分析看板：

### 核心亮点

- **全机项目动态盘点**：自动发现本机所有通过 `upy` 纳管的项目，综合呈现**运行状态**、**部署类型**（SPA / Docker / Systemd）、**当前版本号**与**磁盘占用**。
- **最大空间 100% 基准对比**：以占用磁盘最大的项目为 **100.0% 基准**，自动绘制纤细的项目色条（如 `━━━━━━━━────────────`），不展示数字百分比，一眼识别存储大户。
- **最近一次部署动态置顶**：在顶栏 Spotlight 卡片中高亮显示最近一次部署的项目名称、版本号、部署类型、相对时间（如“22分钟前”）及运行状态。
- **杜绝软链双重计数**：精准排除 `current` 软链接对版本目录的重复深度遍历，保障磁盘统计真实可靠。
- **深度子模块展开 (`--detail`)**：细分各项目 `releases/`（活跃版本与历史旧版）、`bundles/` 缓存、`backups/` 备份归档与共享存储的体积分布。
- **结构化输出 (`--json`)**：支持输出标准 JSON 格式，方便与监控大屏、定时巡检脚本或 CI/CD 流程无缝集成。

### 终端效果展示

```text
upy analysis — 客户端项目与磁盘分析

━ 最近一次部署动态 ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  📦 项目: my-api-service (v1.2.0)
  🧩 类型: Systemd 系统服务 (system) · upy-my-api-service.service
  ⏱  时间: 22分钟前 (2026-10-19 19:40:02)
  🟢 状态: 正常运行 (active)

━ 项目分析 (状态 · 空间基准对比 · 类型 · 版本号) ━━━━━━━━━━━━━━━━━━━━━━━━━━━
#   项目名称            状态           类型     当前版本    磁盘占用    基准对比 (Max)
1   my-compose-app      🟢 active      docker   v2.1.0      1.45 GB     ━━━━━━━━━━━━━━━━━━━━
2   my-api-service      🟢 active      system   v1.2.0      482.3 MB    ━━━━━━━─────────────
3   portal-frontend     🟢 active      static   v0.9.4       86.1 MB    ━───────────────────
4   demo-spa            ⚪ undeployed  static   -            14.2 MB    ━───────────────────

━ 存储汇总统计 ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  📊 纳管项目: 4 个
  💾 累计占用: 2.03 GB (平均每个项目 519.6 MB)
  👑 空间首位: my-compose-app (1.45 GB, 占全机 upy 项目的 71.4%)
  💡 提示: 可使用 upy analysis --detail 查看各子目录详细空间分布
```

### 常用命令示例

```bash
# 1. 默认展示全机项目状态、版本、磁盘占用基准图与最近一次部署
upy analysis

# 2. 展开各项目 releases、bundles、backups 等子目录明细
upy analysis --detail

# 3. 查看指定单项目的空间占用与内部结构
upy analysis my-api-service

# 4. 指定排序规则（可选 size、time、name，默认按 size 降序）
upy analysis --sort time

# 5. 输出 JSON 结构化数据供自动化脚本处理
upy analysis --json
```

---

## 🔔 通知服务 (Bark 移动端推送 & 项目专属 Webhook)

`upy` 将实时通知内建于发布与备份的全生命周期中，提供极简、快速、可靠的团队运维观测闭环。

---

### 一、多接收人 Bark 实时通知

[Bark](https://bark.day.app/) 是一款针对苹果 iOS / iPadOS 设备的轻量、开源即时通知推送服务。只需要发送简单的 HTTP 请求，即可秒级将自定义通知推送至手机设备，完全摆脱重型机器人或第三方中转平台的羁绊。

#### 核心亮点

- **统一标准配置模型**：采用「通用属性 + 按人分列用户（`users:`）」的统一结构。通用属性对所有成员默认生效，用户级字段支持独立定制且优先级高于通用属性，未设置项自动继承通用属性。
- **项目与全局双层配置，智能合并去重**：支持在项目目录的 `deploy.yaml` 中配置项目专属通知人，并在 `~/.upy/config.yaml` 中配置机器级兜底通知人。`upy` 会在通知触发时自动聚合两处配置，并基于服务端与 Token 进行智能去重，杜绝单人收到多条重复告警。
- **多人协同广播**：支持同时配置多位研发、运维或测试人员，单次部署或备份事件并行推送到全体相关人员设备。
- **项目级静音开关**：如果某个内部项目无需推送通知，只需在项目的 `deploy.yaml` 中声明 `notify: { enabled: false }`，即可独立关闭该项目的所有通知。
- **全方位运维上下文**：推送卡片自动携带项目名称、目标版本、部署类型、执行耗时；若发生异常失败，还会自动附带精准的排障失败原因。

#### 标准统一配置写法

`upy` 的 Bark 配置统一采用 **通用属性**（外层全局默认）配合 **`users` 列表**（按 `- name` 区分具体接收人）的标准格式：

```yaml
notify:
  bark:
    # 1. 通用属性 (对全体成员生效 / 默认继承)
    serverUrl: "https://api.day.app" # 默认为官方地址，自建私有节点可填私有域名
    group: "payment-api"             # 通用推送分组
    sound: "chime"                   # 通用提示音 (如 bell, chime, glass, minuet 等)
    level: "active"                  # 通用时效等级 (active / timeSensitive / critical)

    # 2. 用户级属性 (按 - name 分多用户，字段优先级高于外层通用属性)
    users:
      - name: "前端负责人"
        token: "DEVICE_KEY_ALICE"
        sound: "glass"                 # 用户专属覆盖：使用专属提示音 glass
      - name: "值班运维"
        token: "DEVICE_KEY_BOB"
        serverUrl: "https://bark.ops.internal" # 用户专属覆盖：推送到内部自建 Bark 节点
        level: "timeSensitive"
      - name: "测试工程师"
        token: "DEVICE_KEY_CAROL"      # 未指定独立属性，无缝继承外层通用配置
```

> **字段优先级与继承规则**：  
> `users[i]` 内部字段（如 `sound`, `serverUrl`, `group`, `level`, `icon`）具有最高优先级；若用户未显式指定某字段，则自动回退继承外层通用属性；若外层亦未填写，则使用系统默认值。

> **单人极简配置**：  
> 若当前环境仅需通知单人，亦可省略 `users` 列表，直接在 `bark` 下声明 `token` 即可开箱即用：
> ```yaml
> notify:
>   bark:
>     serverUrl: "https://api.day.app"
>     group: "my-service"
>     sound: "bell"
>     token: "DEVICE_KEY_SINGLE"
> ```

#### 推送内容与卡片示例 (Title & Body 详解)

`upy` 在部署或备份完成时，会自动将全方位的运维上下文（项目名称、版本号、部署类型、项目根目录、执行耗时、失败排障原因）规范组装为结构化的 **标题 (Title)** 与 **正文 (Body)** 推送至 Bark，**无需任何手动模板拼接**。

##### 1. 部署成功通知示例
- **标题 (Title)**: `upy 部署成功 · my-api-service`
- **正文 (Body)**:
  ```text
  项目: my-api-service
  版本: v1.2.0
  类型: Systemd 系统服务
  目录: /opt/my-api-service
  耗时: 3.2s
  ```

##### 2. 部署失败告警示例 (敏感等级自动升级为 `timeSensitive`)
- **标题 (Title)**: `upy 部署失败 · my-compose-stack`
- **正文 (Body)**:
  ```text
  项目: my-compose-stack
  版本: v1.1.0
  类型: Docker Compose 集成项目
  目录: /opt/my-compose-stack
  原因: healthcheck probe http://127.0.0.1:8080/health failed (timeout 30s)
  ```

##### 3. 备份成功通知示例
- **标题 (Title)**: `upy 备份成功 · my-api-service`
- **正文 (Body)**:
  ```text
  项目: my-api-service
  归档: dump-my-api-service-20261019-120000.sql.gz (42.5 MB)
  目录: /opt/my-api-service
  耗时: 2.4s
  ```

##### 4. 备份失败告警示例
- **标题 (Title)**: `upy 备份失败 · my-api-service`
- **正文 (Body)**:
  ```text
  项目: my-api-service
  目录: /opt/my-api-service
  原因: exit status 1: mysqldump: Got error: 2003: Can't connect to MySQL server
  ```

#### 配置放置位置

##### 位置 1：项目级专属配置（`<项目根目录>/deploy.yaml`）
只接收该项目的部署与备份通知。建议将关注该项目的研发/测试人员配置在此处：

```yaml
name: my-api-service
release:
  provider: github
  repository: owner/my-api-service

# 项目专属通知配置
notify:
  bark:
    group: "my-api-service"
    sound: "chime"
    users:
      - name: "开发同学"
        token: "DEV_TOKEN_1"
      - name: "测试同学"
        token: "DEV_TOKEN_2"
        sound: "glass"
```

> **项目独立静音**：如果某个测试项目无需推送，配置 `notify: { enabled: false }` 即可屏蔽该项目的所有通知。

##### 位置 2：机器全局兜底配置（`~/.upy/config.yaml`）
整台服务器通用的通知渠道（例如运维大群或 SRE 负责人），对这台机器上所有纳管项目默认生效：

```yaml
github:
  token: ghp_xxxxxx

# 机器全局通知渠道
notify:
  enabled: true
  bark:
    serverUrl: "https://api.day.app"
    group: "Host-Alerts"
    sound: "bell"
    users:
      - name: "SRE Lead"
        token: "SRE_DEVICE_KEY"
        level: "timeSensitive"
```

#### 智能合并与去重机制

- **自动合并**：触发部署或备份时，`upy` 会同时收集 **当前项目的 `deploy.yaml`** 与 **机器全局的 `~/.upy/config.yaml`**。
- **智能去重**：如果同一个 Token（或相同服务端 + Token）在项目配置和全局配置中同时存在，`upy` 会**自动去重仅推送一次**，并以项目级的自定义配置优先。

---

### 二、项目专属 Webhook 通知

不同项目通常对接不同的群机器人或团队服务（如企业微信机器人、飞书机器人、钉钉机器人或内部监控服务），**每个项目都不一样，无需也不应该在全局配置文件中进行统一配置**。`upy` 支持在项目根目录的 `deploy.yaml` 中配置项目专属 Webhook。

#### 1. 配置写法示例（`<项目根目录>/deploy.yaml`）

```yaml
name: my-api-service
release:
  provider: github
  repository: owner/my-api-service

# 项目专属通知配置（仅对当前项目生效，不污染全局配置）
notify:
  # 方式 A：单一 URL 极简直配
  webhook: "https://ops.example.com/api/deploy-webhook"

  # 方式 B：多端点配置、自定义 Header 与 HMAC-SHA256 签名防伪
  # webhook:
  #   - url: "https://ops.example.com/api/deploy-webhook"
  #     secret: "SEC_KEY_FOR_HMAC_SHA256"      # 自动在 Header 附带 X-Upy-Signature
  #     headers:
  #       X-Environment: "production"
  #   - url: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
```

#### 2. 安全签名与请求头规范

当配置了 `secret` 时，`upy` 会基于该密钥对请求正文计算 HMAC-SHA256 签名，并在 HTTP POST 请求头中注入标准元数据：
- `Content-Type: application/json; charset=utf-8`
- `User-Agent: upy/<version>`
- `X-Upy-Event`: 事件类型（`deploy.success`, `deploy.failure`, `backup.success`, `backup.failure`）
- `X-Upy-Delivery`: 每次请求唯一的 UUIDv4 标识符，方便接收方去重与链路追踪
- `X-Upy-Signature`: 签名值，格式为 `sha256=<hex_hmac>`，接收方可用相同的 `secret` 校验来源真实性

#### 3. Webhook JSON 请求体 Payload

```json
{
  "event": "deploy.success",
  "project": "my-api-service",
  "status": "success",
  "action": "deploy",
  "version": "v1.2.0",
  "type": "system",
  "directory": "/opt/my-api-service",
  "elapsed": "3.2s",
  "elapsed_seconds": 3.2,
  "timestamp": "2026-10-19T12:00:00Z"
}
```

> 💡 **智能跨量级耗时换算**：  
> 在 Bark 推送、Webhook Payload 及终端输出中，耗时严谨按阶梯智能换算——小于 60s 格式化为秒（如 `3.2s`），超过 60s 换算为分钟（如 `1m15s`），超过 60m 自动以小时开头（如 `1h2m30s`），拒绝机械输出全秒。

---

## Server 服务与 Webhook 自动化

`upy server` 提供了轻量级的后台常驻守护进程，为 CI/CD 流水线（如 GitHub Actions、GitLab CI）提供免 SSH 密钥的安全入站部署端点，并内置基于 TOTP 的 2FA 双因素安全 Web 控制台。

### 1. 快速启动与管理

```bash
# 后台常驻模式启动 (默认监听 127.0.0.1:8999)
upy server start -d

# 自定义监听端口与地址
upy server start -d --listen 127.0.0.1:8999

# 查看运行状态与 PID
upy server status

# 安全停止守护进程
upy server stop
```

### 2. CI/CD 入站 Webhook 自动部署

在项目的 `deploy.yaml` 中配置 `webhook.secret`：

```yaml
name: my-api-service
type: system
target:
  os: linux
  arch: amd64

# 入站 CI 触发配置
webhook:
  secret: "whsec_your_strong_random_secret"
```

在 GitHub Actions 流水线中，发布 Release 成功后直接调用 `POST /api/v1/webhook/{project}`：

```yaml
- name: Trigger upy deployment
  env:
    UPY_WEBHOOK_URL: "https://deploy.example.com/api/v1/webhook/my-api-service"
    UPY_WEBHOOK_SECRET: "${{ secrets.UPY_WEBHOOK_SECRET }}"
    TAG: "${{ github.ref_name }}"
  run: |
    payload='{"target":"'${TAG}'","force":false}'
    sig=$(printf %s "$payload" | openssl dgst -sha256 -hmac "$UPY_WEBHOOK_SECRET" | sed 's/^.*= //')
    curl -fsS -X POST "$UPY_WEBHOOK_URL" \
      -H "Content-Type: application/json" \
      -H "X-Hub-Signature-256: sha256=$sig" \
      -d "$payload"
```

> 💡 **免 SSH 私钥与异步调度**：  
> - 彻底告别在 CI 仓库 Secrets 存入服务器私钥（`SSH_PRIVATE_KEY`）的历史隐患。  
> - Webhook 接口校验签名后立即返回 `202 Accepted` 并异步执行部署，避免 CI 流程因长任务超时。  
> - 相同项目并发请求自动通过互斥锁排队/拒绝（返回 `409 Conflict`），保障部署一致性。

### 3. TOTP 2FA 双因素 Web 控制台

`upy server` 内置极简 Web 控制台，保护日志与服务状态不被未经授权的恶意扫描窥探：

- **终端绑定 2FA（`upy server auth`）**：管理员在 VPS 的交互终端执行命令，用 Google Authenticator、1Password 等扫描终端中的二维码。Web `/auth` 路由已移除，始终返回 404；不再通过浏览器分发绑定密钥。
- **动态码验证登录 (`/`)**：访问根路由输入 6 位 TOTP 动态码，校验成功后签发有效期为 30 分钟的 `HttpOnly` + `SameSite` 安全 Cookie。
- **全量日志浏览 (`/logs`)**：已鉴权用户可按项目查阅历史部署记录，并直接在线查看高保真终端 ANSI 原色渲染日志。

首次绑定或给管理员添加认证器时，请以运行 server 的同一系统用户、相同 `UPY_CONFIG` 执行：

```bash
upy server auth
# 使用自定义配置时：
UPY_CONFIG=/path/to/config.yaml upy server auth
```

该命令复用已有 `totp.json` 密钥，不会重置现有绑定；为避免密钥长期暴露，受最大查看次数限制（配置项 `server.totp_max_views`，默认 5 次），次数耗尽后禁止再次展示。二维码仅写入交互终端，不写入 upy 日志，也不支持管道重定向；不需要停止 server。升级前已绑定的认证器继续有效。密钥文件损坏时会报错，不会静默生成新密钥。

控制台采用浅色移动端布局，桌面端使用居中的 iPhone 17 容器（402px × 874px），头部与底栏固定、中央内容独立滚动，日志容器预留右侧手势通道避免触控拦截，Lucide SVG 图标、吉祥物 WebP Logo 与样式全部内嵌在二进制中：

- **验证登录（`/`）**：输入认证器中的 6 位动态验证码，设有防爆破频率限制（每分钟最多 10 次失败尝试，成功不计入，超限直接禁止访问，重启 server 可重置状态）。
- **总览（`/overview`）**：登录后的默认入口，展示主机、server 运行时间、项目与最近部署结果。
- **项目（`/projects`）**：支持名称 / 目录搜索，详情展示 `state.json` 中的当前版本、前一版本、服务单元及项目部署记录。
- **日志（`/logs`）**：支持项目、状态和关键词筛选，每页 20 条；保留 Server 与 upy 运行日志入口。
- **日志详情（`/logs/{id}`）**：支持复制与完整文件下载。执行中的任务与运行日志每 3 秒刷新；会话过期时提示重新登录。

页面、日志刷新与下载均受同一 TOTP Session 保护。控制台只读，不提供部署操作。“部署成功”表示部署结果，不代表应用实时健康；历史日志缺少明确结束标记时显示“结果未知”。大日志仅预览末尾 256 KiB，导出仍获取完整原始文件。

控制台支持 PWA：通过 HTTPS 访问后，可在支持的浏览器中安装至桌面，以独立窗口打开；iPhone / iPad 使用分享菜单“添加到主屏幕”。启动入口为 `/overview`，未登录时仍需 TOTP 验证，Cookie 有效期保持 30 分钟。Service Worker 仅缓存公开图标与无业务数据的离线提示页，不缓存登录内容、项目、日志、下载或 API；断网时提示重新连接。新版本就绪后由用户点击刷新，不会自动打断正在阅读的日志。

---

## 日志系统与 upy log 命令

`upy` 建立了一套分工明确的系统化日志体系，覆盖 CLI 执行、Server 运行与各项目部署过程，并保证所有敏感信息自动脱敏。

### 1. 三路存储与自动脱敏规范

- `~/.upy/logs/upy.log`：记录每一次终端 CLI 调用流水（时间戳、PID、执行命令、耗时、退出码），自动剥离 ANSI 方便 grep/awk，所有 Token 与密钥自动正则掩码。
- `~/.upy/logs/server.log`：记录 Server 守护进程的启动、关闭与 HTTP 请求访问（含来源 IP、方法、路径、状态码、响应耗时）。
- `~/.upy/logs/<project>/<timestamp>.log`：保留每个项目历次部署的完整过程日志（支持 Web 终端高亮展示）。

### 2. upy log 命令速查

```bash
# 交互式选择菜单（光标直接选择 Server、CLI 或具体项目）
upy log

# 实时追踪 Server 运行与访问日志 (类似 tail -f)
upy log server -f
upy log -s -f

# 查看 upy 命令行自身最近 100 行执行记录
upy log upy -n 100

# 查看指定项目最近一次部署的完整输出
upy log my-api-service

# 列出本机所有日志文件及体积占用
upy log --list

# 清空指定的日志文件
upy log server -c
upy log upy -c
```

> 💡 **防死循环机制**：  
> 当执行 `upy log` 系列命令时，系统会自动跳过向 `upy.log` 写入，彻底消除 `upy log -f` 追踪自身时产生无限循环自 tail 的问题。

---

## 如何升级

可选在 `~/.upy/config.yaml` 中开启 upy 自身升级下载加速：

```yaml
update:
  proxy_enabled: true
```

默认关闭（`false` 或不配置）。开启后，`upy update`（含指定版本）通过 `https://ghfast.top/` 加上 GitHub Release 原始 URL 下载二进制及 `SHA256SUMS`，继续执行大小检查和 SHA-256 校验。版本查询仍直连 GitHub API；项目部署下载不受影响，加速请求不携带 GitHub Token。服务不可用时设为 `false` 恢复原下载路径。此开关不控制已有的系统 HTTP / HTTPS 代理。

`upy` 拥有完善的跨分区自更新与在运行进程替换机制，随时保持工具最新：

```bash
# 检查并自动升级到最新版本
upy update

# 更新或回滚到指定版本
upy update v0.2.0

# 强制重新下载并覆盖更新
upy update --force

# 查看当前运行版本及环境
upy version
```

> 💡 **自动同步 Shell 命令补全**：  
> 在执行 `upy update` 升级或切换版本时，`upy` 会自动跨 Shell 探测当前主机环境（支持 `zsh`、`bash` 与 `fish`），并一键就地同步更新命令补全脚本。即使新版本加入了新一级命令（如 `analysis`），无需手动执行任何脚本，新开终端（或 `source` 配置文件）即可直接体验流畅的 Tab 补全。

---

## 常用命令速查

| 命令 | 说明 |
| :--- | :--- |
| `upy` | 在任意目录唤起已登记项目列表，交互选择项目并部署 |
| `upy release [latest\|<版本>]` | 在项目目录下从 GitHub Release 拉取并部署（缺省参数时交互选版） |
| `upy release list` | 查看当前项目的 GitHub Release 版本历史与资产信息 |
| `upy deploy <项目名>` | 快捷为已登记的本地项目部署最新正式版本 |
| `upy clean [项目名]` | 可视化分析并清理项目数据 (Bundle 缓存、旧版本、临时目录或注销) |
| `upy backup [项目名]` | 执行项目约定备份 (`backup.sh`)，管理归档轮转并推送通知 |
| `upy analysis [项目名]` | 分析全机已部署项目，以最大占用为 100% 基准展示磁盘分布与最近部署 |
| `upy server [start\|stop\|restart\|status\|auth]` | 启动 Webhook 服务与 Web 控制台，支持 `-d` 静默守护进程 |
| `upy log [server\|upy\|<项目>]` | 交互式或指定查看运行日志，支持 `-f` 实时追踪与 `-n` 行数截取 |
| `upy bundle <文件>` | 直接部署本地 zip 压缩包（离线模式，不访问 GitHub） |
| `upy init [owner] <token>` | 初始化或添加 GitHub 访问令牌（支持组织/个人专属） |
| `upy update [版本]` | 自更新到最新版本或切换至指定版本 |
| `upy version` | 打印当前版本并检查是否有新版本可用 |

---

## Agent Skill

使用 [upy-project-adapter](skills/upy-project-adapter/SKILL.md) 让 Agent 为应用生成 upy 配置、部署 bundle 和 GitHub Actions Release，覆盖 SPA、Docker/Compose 与 system 服务。安装方式与调用示例见 [Skills 说明](skills/README.md)。

## 开源协议

MIT License
