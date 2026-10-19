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
- [多接收人 Bark 实时通知](#多接收人-bark-实时通知)
- [如何升级](#如何升级)
- [常用命令速查](#常用命令速查)
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
- **约定式数据备份**：零配置项目自动探测（`backup.sh`）、跨进程互斥锁防护、智能多版本轮转与全量一键批处理。
- **本地项目快速调度**：在任意路径直接运行 `upy`，即可列出本机已纳管的项目并交互式选择部署。
- **多接收人 Bark 移动端推送**：部署与备份结果即时推送至 iOS 设备，底层基于 [Bark](https://bark.day.app/)（一款开源的 iOS 实时推送服务）。支持在 `deploy.yaml` 中进行项目级独立配置、在 `~/.upy/config.yaml` 中进行全局兜底配置、多接收人并行广播（纯 token、完整 URL 或高级对象格式）与智能自动去重。

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
VERSION=v0.2.0 curl -fsSL https://raw.githubusercontent.com/AaronConlon/upy/main/install.sh | bash
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

在 CI/CD（如 GitHub Actions）构建产物并打包发布时，压缩包内部需包含应用文件以及一份描述清单 `deploy.yaml`：

#### A. 前端静态应用 (SPA)

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

#### C. 原生系统服务 (system / Go 二进制 Web 服务)

适用于轻量级 Go/Rust/Node 等单二进制或本地常驻 Web 服务。采用标准轻量生产架构：**upy 管版本与发布，systemd 管进程守护与 cgroups，Nginx/Caddy 管公网入口与 SSL**。

压缩包内结构：
```text
bundle.zip
├── deploy.yaml
└── bin/
    └── my-api
```

清单 `deploy.yaml` 格式：
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
> - **通知联动推送**：备份完成后自动汇总耗时、产物文件体积，并通过 Bark 等渠道实时推送备份结果与告警。

---

## 多接收人 Bark 实时通知

[Bark](https://bark.day.app/) 是一款针对苹果 iOS / iPadOS 设备的轻量、开源即时通知推送服务。只需要发送简单的 HTTP 请求，即可秒级将自定义通知推送至你的手机设备，完全摆脱重型机器人、第三方中转平台以及繁琐 Webhook 中间件的羁绊。

`upy` 将 Bark 原生内建于发布与备份的全生命周期中，打造极简、快速、可靠的团队移动端观测闭环。

### 核心亮点

- **项目与全局双层配置，智能合并去重**：支持在项目目录的 `deploy.yaml` 中配置项目专属通知人，并在 `~/.upy/config.yaml` 中配置机器级兜底通知人。`upy` 会在通知触发时自动聚合两处配置，并基于服务端与 Token 进行智能去重，杜绝单人收到多条重复告警。
- **多人协同广播**：支持同时配置多位研发、运维或测试人员的 Device Token，单次部署或备份事件并行推送到全体相关人员设备。
- **多形态灵活配置**：既支持纯 Token 字符串、完整推送 URL 数组（兼容官方 `https://api.day.app` 及自建 Bark 节点），也支持携带独立音效（`sound`）、自定义分组（`group`）和敏感等级（`level`）的高级对象定义。
- **项目级静音开关**：如果某个内部项目无需推送通知，只需在项目的 `deploy.yaml` 中声明 `notify: { enabled: false }`，即可独立关闭该项目的所有通知。
- **全方位运维上下文**：推送卡片自动携带项目名称、目标版本、部署类型、执行耗时；若发生异常失败，还会自动附带精准的排障失败原因。

### 最常用的 3 种配置写法

#### 1. 极简字符串列表（推荐多人团队使用）
直接把每个人的 Device Key 或 Bark App 里复制的完整链接填成列表：

```yaml
notify:
  bark:
    - "DEVICE_KEY_ALICE"
    - "DEVICE_KEY_BOB"
    - "https://api.day.app/DEVICE_KEY_CAROL" # 粘贴完整 URL 也会自动解析
```

#### 2. 统一分组与铃声，指定多个 Token
如果所有人共用同一分组、通知铃声或自建 Bark 节点，可以用 `tokens:` 批量指定：

```yaml
notify:
  bark:
    serverUrl: "https://api.day.app" # 默认为官方地址，自建节点可填私有域名
    group: "payment-api"             # 推送分组
    sound: "chime"                   # 自定义铃声 (如 bell, chime, glass, minuet 等)
    tokens:
      - "DEVICE_KEY_ALICE"
      - "DEVICE_KEY_BOB"
```

#### 3. 详细对象列表（按人定制独立属性）
如果不同成员需要定制不同的标签、铃声或不同的私有 Bark 服务端：

```yaml
notify:
  bark:
    - name: "前端负责人"
      token: "DEVICE_KEY_FRONTEND"
      sound: "glass"
    - name: "值班运维"
      serverUrl: "https://bark.mycorp.internal" # 支持私有自建 Bark 服务
      token: "DEVICE_KEY_OPS"
      sound: "minuet"
      level: "timeSensitive"                     # 敏感等级 (active / timeSensitive / critical)
```

### 配置放置位置

#### 位置 1：项目级专属配置（`<项目根目录>/deploy.yaml`）
只接收该项目的部署与备份通知。建议将关注该项目的研发/测试人员配置在此处：

```yaml
name: my-api-service
release:
  provider: github
  repository: owner/my-api-service

# 项目专属通知人
notify:
  bark:
    - "DEV_TOKEN_1"
    - "DEV_TOKEN_2"
```

> **项目独立静音**：如果某个测试项目无需推送，配置 `notify: { enabled: false }` 即可屏蔽该项目的所有通知。

#### 位置 2：机器全局兜底配置（`~/.upy/config.yaml`）
整台服务器通用的通知渠道（例如运维大群或 SRE 负责人），对这台机器上所有纳管项目默认生效：

```yaml
github:
  token: ghp_xxxxxx

# 机器全局通知渠道
notify:
  enabled: true
  bark:
    - name: "SRE Lead"
      token: "SRE_DEVICE_KEY"
      sound: "bell"
```

### 智能合并与去重机制

- **自动合并**：触发部署或备份时，`upy` 会同时收集 **当前项目的 `deploy.yaml`** 与 **机器全局的 `~/.upy/config.yaml`**。
- **智能去重**：如果同一个 Token（或相同服务端 + Token）在项目配置和全局配置中同时存在，`upy` 会**自动去重仅推送一次**，并以项目级的自定义配置优先。

---

## 如何升级

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
| `upy bundle <文件>` | 直接部署本地 zip 压缩包（离线模式，不访问 GitHub） |
| `upy init [owner] <token>` | 初始化或添加 GitHub 访问令牌（支持组织/个人专属） |
| `upy update [版本]` | 自更新到最新版本或切换至指定版本 |
| `upy version` | 打印当前版本并检查是否有新版本可用 |

---

## Agent Skill

使用 [upy-project-adapter](skills/upy-project-adapter/SKILL.md) 让 Agent 为应用生成 upy 配置、部署 bundle 和 GitHub Actions Release，覆盖 SPA、Docker/Compose 与 system 服务。安装方式与调用示例见 [Skills 说明](skills/README.md)。

## 开源协议

MIT License
