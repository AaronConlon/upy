<p align="center">
  <img src="assets/upy-gopher-skateboard-logo.png" width="220" alt="Go Gopher on a skateboard, upy project logo">
</p>

<h1 align="center">upy</h1>

<p align="center">
One single binary, from Release to production.
<br>
<a href="https://aaronconlon.github.io/upy/">Website</a> · <a href="https://github.com/AaronConlon/upy/releases">Releases</a> · <a href="#installation">Quick Start</a>
<br><br>
<b>English</b> · <a href="README.zh-CN.md">简体中文</a>
</p>

A lightweight, single-binary Web deployment and ops tool written in Go (~8.5MB, zero external dependencies, minimal memory footprint).

Designed for single-machine production environments with minimal cognitive overhead, `upy` natively supports **three core deployment modes**:

1. **SPA Static Sites**: Single Page Applications (Vue, React, Svelte, Vite, etc.) extracted to immutable version folders, switched atomically in milliseconds via the `current` symlink, with automatic permission normalization (`0755/0644`) to eliminate Nginx / Caddy `403 Forbidden` errors.
2. **Docker / Compose Applications**: Smooth handover by preparing new images and configurations before stopping previous containers to prevent port conflicts, backed by built-in health probes and automatic rollback on failure.
3. **Systemd Native Services (Go / Rust / Binary Web Services)**: Clean production architecture (**upy manages releases $\longleftrightarrow$ systemd manages processes & cgroups $\longleftrightarrow$ Nginx/Caddy manages ingress & SSL**). Features immutable release directory archives, 100% argument fidelity, strict user/group pre-flight checks (rejecting silent root escalation), port listener PID verification, and zero-loss automatic rollback.

Paired with intelligent **disk space pruning and active version protection** (`upy clean`), **cross-process mutual exclusion locks**, and **convention-based backups**, `upy` forms a concise and reliable single-host operations loop.

---

## Table of Contents

- [Three Deployment Modes](#three-deployment-modes)
- [Core Features](#core-features)
- [Installation](#installation)
- [Initialization](#initialization)
- [How to Deploy](#how-to-deploy)
  - [1. Server-Side Project Configuration](#1-server-side-project-configuration)
  - [2. Common Deployment Commands](#2-common-deployment-commands)
  - [3. Packaging Specifications (bundle.zip)](#3-packaging-specifications-bundlezip)
    - [A. Frontend Static Applications (SPA)](#a-frontend-static-applications-spa)
    - [B. Docker / Compose Containers](#b-docker--compose-containers)
    - [C. Native Systemd Services (Go Binary Web Services)](#c-native-systemd-services-go-binary-web-services)
- [Disk Maintenance & Pruning](#disk-maintenance--pruning)
- [Convention Backups](#convention-backups)
- [Project & Disk Analysis (upy analysis)](#project--disk-analysis-upy-analysis)
- [Multi-Recipient Bark Notifications & Project Webhook](#multi-recipient-bark-notifications--project-webhook)
- [Server Daemon & Inbound Webhook](#server-daemon--inbound-webhook)
- [Unified Logging & the upy log Command](#unified-logging--the-upy-log-command)
- [Self-Update](#self-update)
- [Command Cheat Sheet](#command-cheat-sheet)
- [Agent Skill](#agent-skill)
- [License](#license)

---

## Three Deployment Modes

| Deployment Mode | Typical Stacks | Architecture & Guarantees | Failure Recovery |
| :--- | :--- | :--- | :--- |
| **SPA Static Sites**<br>`type: static` | Vue, React, Vite, Astro, Static HTML | Extracted into `releases/<version>`; atomic `current` symlink switch; auto permission normalization to `0755/0644` | Symlink instantly points back to previous version |
| **Docker Containers**<br>`type: docker` | Containerized Web apps, Docker Compose stacks | Pulls and prepares new containers before stopping old ones; frees host ports gracefully | Automatically starts previous working image & compose stack |
| **Systemd Native Services**<br>`type: system` | Go, Rust, Node, C++ compiled binaries | Generates and switches systemd unit atomically; immutable version isolation; `shared/` persistence; strict user check prevents root escalation | Stops failed service, restores prior unit, restarts old process, and verifies HTTP health |

---

## Core Features

- **Single Binary, Zero Dependencies**: Only ~8.5MB. Runs out of the box without requiring Node.js, Python, or host daemon locks.
- **Three Deployment Stacks in One**: SPA static sites, Docker Compose stacks, and native systemd binaries share a unified CLI workflow and release lifecycle.
- **GitHub Release Driven**: Supports both public and private repositories, multi-org/user token routing, resumable chunked downloads, and SHA-256 checksum verification.
- **Robust Failure Safety**: Strict pre-flight checks during `Prepare` (permissions, target architecture, binary validity, service user existence). Prepare failures never touch existing running services.
- **Disk Analysis & Safe Pruning**: `upy clean` analyzes disk footprint and frees Bundle archives while strictly protecting both the current active version, the previous rollback version, and live unit references.
- **Host-Wide Project & Disk Analysis**: `upy analysis` catalogs all managed projects on the machine, plots proportional horizontal benchmark bars using the largest project as the 100% baseline, spotlights the most recent deployment, and supports `--detail` breakdowns and `--json` exports.
- **Cross-Project Global Dispatching**: Run `upy` in any directory to interactively list and deploy across all managed projects on the host.
- **Convention Backups**: Zero-config project discovery (`backup.sh`), exclusive concurrency locks, automatic retention rotation, and multi-project batch runs.
- **Multi-Recipient Bark Push Alerts & Project Webhooks**: Instant mobile alerts powered by [Bark](https://bark.day.app/) and project-isolated Webhooks (with HMAC-SHA256 signature verification for WeChat Work, Feishu/Lark, DingTalk, and custom endpoints). Features dual-level configuration, team fan-out, smart deduplication, and intelligent multi-scale duration formatting (seconds, minutes, hours).

---

## Installation

Run the one-line installation script on Linux or macOS to automatically detect architecture (x64 / arm64) and install `upy`:

```bash
curl -fsSL https://raw.githubusercontent.com/AaronConlon/upy/main/install.sh | bash
```

> **Note**: Default install location is `/usr/local/bin` (requests sudo when needed). The installer automatically configures shell tab completions for `zsh`, `bash`, and `fish`.

### Custom Installation Options

You can specify custom install paths or pin specific release versions:

```bash
# Install to user home directory (no root required)
INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/AaronConlon/upy/main/install.sh | bash

# Install a specific version
VERSION=v0.2.0 curl -fsSL https://raw.githubusercontent.com/AaronConlon/upy/main/install.sh | bash
```

---

## Initialization

If your project is hosted in a **private GitHub repository**, configure your GitHub Token to allow `upy` to download Release assets. `upy` supports a global fallback token as well as multi-token routing based on organization or username.

### 1. Quick CLI Setup

```bash
# Set global default token
upy init ghp_your_default_token

# Set organization-specific token (e.g., MyOrg)
upy init MyOrg ghp_org_token

# Set user-specific token (e.g., AaronConlon)
upy init AaronConlon ghp_personal_token

# Interactive setup (masked password input)
upy init
```

Configuration is securely written to `~/.upy/config.yaml` with file permissions restricted to `600`.

### 2. Configuration Reference (`~/.upy/config.yaml`)

```yaml
github:
  # Global fallback token (optional)
  token: ghp_default_xxx

  # Organization / user-specific tokens (optional, prioritized by repo owner)
  tokens:
    MyOrg: ghp_org_xxx
    AaronConlon: ghp_personal_xxx

notify:
  enabled: true # Optional master switch (defaults to true)
  bark:
    # Format 1: Multi-recipient device tokens or full URLs
    - your-bark-device-key-1
    - https://api.day.app/your-bark-device-key-2

    # Format 2: Detailed channels with custom sound, group, or self-hosted server
    - name: DevOps On-Call
      serverUrl: https://api.day.app
      token: on-call-device-key
      group: upy.production
      sound: chime
```

---

## How to Deploy

### 1. Server-Side Project Configuration

Create a `deploy.yaml` in your project's server root directory (e.g., `/var/www/my-app` or `/opt/my-api`):

```yaml
name: my-app
release:
  provider: github
  repository: owner/repo      # GitHub repo (owner/name)
  asset: bundle.zip          # Asset name (defaults to bundle.zip, supports regex/wildcard)

# Optional: Bundle archive retention (default: 3, allowed: 0~100)
bundles:
  keep: 3

# Optional: Release directory retention (default: keep current and previous releases)
releases:
  keep: 1

# Optional: Project-level multi-recipient Bark notifications
notify:
  bark:
    - alice-device-key
    - https://api.day.app/bob-device-key
    - name: Team SRE
      token: sre-device-key
      sound: bell

# Optional for system native services: dedicated service account to prevent root escalation
service:
  user: my-app
  group: my-app
  unit: upy-my-app.service
```

> **Nginx Tip for SPA Sites**:  
> Point Nginx's `root` directive to the `current` symlink in your project folder: `root /var/www/my-app/current;`. `upy` switches this symlink atomically during deployment.

### 2. Common Deployment Commands

Execute inside your server project directory:

```bash
# 1. Interactively select a Release version to deploy (recommended)
upy release

# 2. Deploy latest official release directly
upy release latest

# 3. Deploy a specific version
upy release v1.0.0

# 4. View GitHub Release history
upy release list

# 5. Force re-download and re-deploy (even if version is already active)
upy release latest --force
```

#### Cross-Project Global Operations

Once a project has been deployed, `upy` automatically registers it. You can deploy it from **any directory** on your server:

```bash
# Launch interactive selector across all registered projects
upy

# Directly deploy latest release for a registered project
upy deploy my-app
```

#### Offline Local Bundle Deployment

If the server cannot access GitHub, deploy locally uploaded `.zip` archives directly:

```bash
upy bundle ./bundle.zip
```

---

### 3. Packaging Specifications (bundle.zip)

When building artifacts in CI/CD (e.g., GitHub Actions), package your application along with a manifest `upy.yaml` inside `bundle.zip`:

> 💡 **Bundle Manifest vs Server Configuration Decoupling**:  
> To prevent naming collisions and conceptual confusion with the server-side project configuration `deploy.yaml` (which configures Release sources, system service user accounts, etc.), the manifest inside `bundle.zip` is now officially standardized as `upy.yaml` (the `upy` engine maintains backward compatibility with legacy `deploy.yaml`).

#### A. Frontend Static Applications (SPA)

Bundle structure:
```text
bundle.zip
├── upy.yaml
└── dist/
    ├── index.html
    ├── assets/
    └── ...
```

Manifest `upy.yaml`:
```yaml
name: my-spa-app
version: 1.0.0
type: static
static:
  directory: dist   # Static files directory inside the bundle (defaults to dist)
```

#### B. Docker / Compose Containers

Bundle structure:
```text
bundle.zip
├── upy.yaml
├── docker-compose.yml
└── ...
```

Manifest `upy.yaml`:
```yaml
name: my-api-service
version: 1.0.0
type: docker
docker:
  mode: compose
  composeFile: docker-compose.yml
healthcheck:
  url: http://localhost:8080/health
  timeout: 30s      # Healthcheck timeout (supports duration string or seconds integer)
```

#### C. Native Systemd Services (Go Binary Web Services)

Best suited for lightweight single-binary or daemon Web applications (Go, Rust, Node, etc.). Implements the battle-tested lightweight production pattern: **upy manages releases $\longleftrightarrow$ systemd manages processes & cgroups $\longleftrightarrow$ Nginx/Caddy manages ingress & SSL**.

Bundle structure:
```text
bundle.zip
├── upy.yaml
└── bin/
    └── my-api
```

Manifest `upy.yaml`:
```yaml
name: my-api
version: 1.0.0
type: system             # Native systemd service (also accepts alias: service)
target:
  os: linux
  arch: amd64            # Strict target OS and architecture verification
system:
  command: ["./bin/my-api"]  # Command arguments (fidelity preserved, empty strings & whitespace retained)
  workingDir: ""             # Optional, defaults to current release directory
healthcheck:
  url: http://127.0.0.1:8080/health
  timeout: 30s               # Healthcheck timeout (with bounded 3s per-request limit)
```

> **Data Persistence & Shared Environment**:  
> Configure persistent runtime variables in `/opt/my-api/shared/service.env` (e.g., `LISTEN_ADDR=127.0.0.1:8080` and `DATA_DIR=/opt/my-api/shared/data`). Write persistent state (SQLite, user uploads) to `shared/data/`. Release directories `releases/<version>` remain immutable, enabling instant zero-loss rollbacks.

---

## Disk Maintenance & Pruning

As deployments accumulate, cached bundle archives and legacy release directories consume disk space. Use `upy clean` for visual disk inspection and safe pruning:

```bash
# 1. Interactively inspect disk usage and choose pruning targets (recommended)
upy clean

# 2. Prune a specific project
upy clean my-app

# 3. Only delete downloaded Bundle zip caches
upy clean --bundles

# 4. Only delete old releases and interrupted temporary folders
upy clean --old-releases

# 5. One-click routine pruning without confirmation (caches + old releases, preserves active version)
upy clean --all -y
```

> **Safety Safeguards**:  
> - **Dual Version Protection**: `upy clean` enforces active version protection, safeguarding both the active version (`CurrentVersion`) and the immediate previous version (`PreviousVersion`) to guarantee rollback readiness.
> - **Live Unit Reference Protection**: If a deployment was abruptly killed while systemd had already switched to a new release, `clean` inspects the live systemd unit to protect the actively referenced directory from deletion.
> - **Full Service Uninstallation (`--purge`)**: Using `upy clean --purge` safely stops the service, unregisters the unit, and deletes deployment artifacts while keeping `shared/` persistent data intact.

---

## Convention Backups

Production data integrity is paramount during database migrations, system upgrades, or scheduled maintenance. Since databases (MySQL, PostgreSQL, SQLite, Redis) and persistent volumes differ widely, `upy` adheres to **Convention over Configuration**: your application provides the domain-specific backup script, while `upy` handles **process locking**, **runtime context injection**, **archive rotation**, and **push alerts**.

### 1. Zero-Config Convention Discovery

`upy backup` automatically discovers and executes your project's backup script without requiring extra configuration:
1. `<projectRoot>/backup.sh`
2. `<projectRoot>/scripts/backup.sh`
3. `<projectRoot>/bin/backup.sh`

*(Or specify a custom script in `deploy.yaml` via `backup.script`)*.

### 2. Runtime Context Injected by `upy`

`upy` automatically injects standardized environment variables into the script subprocess:

| Variable | Description | Example |
| :--- | :--- | :--- |
| `UPY_PROJECT_NAME` | Project name | `my-api-service` |
| `UPY_PROJECT_ROOT` | Absolute path of project root | `/opt/my-api-service` |
| `UPY_BACKUP_DIR` | Target archive directory | `/opt/my-api-service/backups` |
| `UPY_CURRENT_VERSION` | Currently active release version | `v1.2.0` |
| `UPY_TIMESTAMP` | Formatted timestamp | `20261019-120000` |
| `UPY_DEPLOY_TYPE` | Deployment stack type | `docker`, `system`, or `static` |

### 3. Typical `backup.sh` Example (Docker Compose + Database Dump)

```bash
#!/usr/bin/env bash
set -euo pipefail

# Ensure target directory exists
mkdir -p "${UPY_BACKUP_DIR}"

BACKUP_FILE="${UPY_BACKUP_DIR}/dump-${UPY_PROJECT_NAME}-${UPY_TIMESTAMP}.sql.gz"
echo "Exporting database to ${BACKUP_FILE}..."

# Execute dump within the container
docker compose exec -T db mysqldump -u root -p"${DB_PASSWORD}" "${DB_NAME}" | gzip > "${BACKUP_FILE}"

echo "Database export completed successfully!"
```

### 4. Optional `deploy.yaml` Configuration

```yaml
name: my-app
type: docker
docker:
  mode: compose
  composeFile: docker-compose.yml

# Archive and release retention (optional)
bundles:
  keep: 3                      # Retain latest N bundle archives (default: 3, allowed: 0~100)
releases:
  keep: 1                      # Retain latest release directories (default: current & previous)

# Optional backup settings
backup:
  script: backup.sh            # Custom script path (relative to project root)
  dir: backups                 # Archive directory (defaults to backups/)
  retention: 7                 # Retain latest N archives (default: 7, 0 disables auto-pruning)
  timeout: 15m                 # Maximum timeout (supports 30s, 5m, 1h)
```

### 5. Common Backup Commands

```bash
# 1. Interactively select a registered project to backup
upy backup

# 2. Backup a specific project
upy backup my-app

# 3. View historical backup archives and disk usage
upy backup my-app --list

# 4. Manually rotate and retain only the latest N archives
upy backup my-app --prune 5

# 5. One-click batch backup of all registered projects
upy backup --all -y
```

> **Key Safeguards**:  
> - **Mutual Exclusion Lock (`.deploy.lock`)**: `upy backup` acquires an exclusive project lock (`action: 备份中`), strictly preventing race conditions against concurrent deployments (`upy release`) or cleanup (`upy clean`).  
> - **Automatic Retention Pruning**: Automatically rotates and cleans older snapshots beyond the retention threshold (default: 7), ensuring backup files never exhaust disk space.  
> - **Alerting & Notifications**: Automatically reports elapsed time, archive file sizes, and failure reasons to Bark mobile notifications.

---

## Host-Wide Project & Disk Analysis (upy analysis)

As servers accumulate more web applications, container stacks, and release histories, operators need a quick, unified terminal cockpit to assess total disk space consumption and inspect recent release activity. `upy analysis` delivers an industrial-grade terminal analysis dashboard:

### Highlights

- **Complete Host Project Catalog**: Automatically discovers all projects deployed via `upy`, synthesizing their **running status**, **deployment type** (SPA / Docker / Systemd), **active version**, and **disk space**.
- **100% Benchmark Relative Comparison**: Uses the project with the largest disk footprint as the **100.0% baseline reference**, rendering uniform ANSI horizontal bar charts (e.g. `[████████░░░░░░░░░░░░]  40.0%`) to immediately spot storage hogs.
- **Top Spotlight for Latest Deployment**: Prominently spotlights the most recently deployed project at the top card, showing its name, version, stack type, humanized relative duration (e.g. `22 minutes ago`), and service status.
- **Strict Symlink Protection**: Does not recurse into `current` symlinks, guaranteeing that active releases are never double-counted in disk space calculations.
- **Deep Subdirectory Breakdowns (`--detail`)**: Visualizes individual subdirectories including `releases/` (active vs historical old versions), `bundles/` download cache, `backups/` archives, and `shared/` persistence.
- **Structured JSON Export (`--json`)**: Emits standard JSON for automated SRE monitoring dashboards, audit pipelines, or scheduled health checks.

### Terminal Demonstration

```text
upy analysis — 客户端项目与磁盘分析

━ 最近一次部署动态 ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  📦 项目: my-api-service (v1.2.0)
  🧩 类型: Systemd 系统服务 (system) · upy-my-api-service.service
  ⏱  时间: 22分钟前 (2026-10-19 19:40:02)
  🟢 状态: 正常运行 (active)

━ 项目分析 (状态 · 空间基准对比 · 类型 · 版本号) ━━━━━━━━━━━━━━━━━━━━━━━━━━━
#   项目名称            状态           类型     当前版本    磁盘占用    基准对比 (Max)
1   my-compose-app      🟢 active      docker   v2.1.0      1.45 GB     [████████████████████] 100.0%
2   my-api-service      🟢 active      system   v1.2.0      482.3 MB    [███████░░░░░░░░░░░░░]  32.5%
3   portal-frontend     🟢 active      static   v0.9.4       86.1 MB    [█░░░░░░░░░░░░░░░░░░░]   5.8%
4   demo-spa            ⚪ undeployed  static   -            14.2 MB    [▏░░░░░░░░░░░░░░░░░░░]   0.9%

━ 存储汇总统计 ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  📊 纳管项目: 4 个
  💾 累计占用: 2.03 GB (平均每个项目 519.6 MB)
  👑 空间首位: my-compose-app (1.45 GB, 占全机 upy 项目的 71.4%)
  💡 提示: 可使用 upy analysis --detail 查看各子目录详细空间分布
```

### Common Usage

```bash
# 1. Default host-wide project catalog, status, version, 100% benchmark bars & latest deployment
upy analysis

# 2. Expand detailed breakdown of releases, bundles, and backups
upy analysis --detail

# 3. Analyze a specific target project
upy analysis my-api-service

# 4. Sort by size, time, or name (defaults to size descending)
upy analysis --sort time

# 5. Output structured JSON for scripts or monitoring
upy analysis --json
```

---

## 🔔 Notification Services (Bark & Project Webhooks)

`upy` builds instant delivery alerts directly into the release and backup lifecycles, ensuring a lightweight and observable operations loop.

---

### 1. Multi-Recipient Bark Notifications

[Bark](https://bark.day.app/) is a lightweight, open-source push notification service for Apple iOS devices. By using simple HTTP requests, you can deliver instant push notifications directly to iPhones and iPads without setting up complex third-party bot frameworks or heavy webhook middleware.

#### Highlights

- **Unified Standard Configuration**: Structured as shared attributes (outer defaults) plus a `users:` list differentiated by `- name`. Shared settings apply by default, per-user fields override shared settings with highest priority, and omitted fields seamlessly inherit defaults.
- **Dual-Level Merging & Smart Deduplication**: Configure team-wide channels in `~/.upy/config.yaml` and project-specific channels in `deploy.yaml`. `upy` merges both sources and automatically deduplicates overlapping recipients so no user receives duplicate alerts.
- **Multi-Recipient Fan-Out**: Effortlessly notify multiple teammates or on-call engineers simultaneously with a single release or backup event.
- **Independent Project Muting**: Silence notifications for a specific project at any time by declaring `notify: { enabled: false }` in its `deploy.yaml`.
- **Rich Context & Diagnostics**: Notifications include project name, deployed version, stack type, elapsed execution time, and exact failure diagnostics if an error occurs.

#### Standard Unified Configuration Format

`upy` standardizes Bark notification configuration using **shared attributes** (outer defaults) combined with a **`users` list** (distinguished per recipient with `- name`):

```yaml
notify:
  bark:
    # 1. Shared / Global defaults (inherited by all recipients)
    serverUrl: "https://api.day.app" # Default official API, or private self-hosted domain
    group: "payment-api"             # Shared notification group
    sound: "chime"                   # Shared ringtone (bell, chime, glass, minuet, etc.)
    level: "active"                  # Shared sensitivity level (active / timeSensitive / critical)

    # 2. Per-user settings (per-recipient overrides, prioritized over shared settings)
    users:
      - name: "Frontend Lead"
        token: "DEVICE_KEY_ALICE"
        sound: "glass"                 # Overrides shared sound
      - name: "DevOps On-Call"
        token: "DEVICE_KEY_BOB"
        serverUrl: "https://bark.ops.internal" # Overrides shared serverUrl
        level: "timeSensitive"
      - name: "QA Engineer"
        token: "DEVICE_KEY_CAROL"      # Inherits all shared attributes seamlessly
```

> **Attribute Priority & Inheritance**:  
> Recipient fields under `users[i]` (e.g. `sound`, `serverUrl`, `group`, `level`, `icon`) take highest precedence. When a field is omitted, it automatically falls back to the outer shared attribute, and ultimately to built-in system defaults.

> **Minimal Single-User Mode**:  
> For single-user environments, you can also omit the `users` list and directly specify `token` under `bark`:
> ```yaml
> notify:
>   bark:
>     serverUrl: "https://api.day.app"
>     group: "my-service"
>     sound: "bell"
>     token: "DEVICE_KEY_SINGLE"
> ```

#### Notification Message Examples (Title & Body Breakdown)

Upon completing a deployment or backup, `upy` automatically extracts runtime context and generates standardized, diagnostic-rich **Title** and **Body** messages pushed directly to Bark with zero template configuration needed:

##### 1. Deployment Success Notification
- **Title**: `upy 部署成功 · my-api-service`
- **Body**:
  ```text
  项目: my-api-service
  版本: v1.2.0
  类型: Systemd 系统服务
  目录: /opt/my-api-service
  耗时: 3.2s
  ```

##### 2. Deployment Failure Alert (Automatic `timeSensitive` escalation)
- **Title**: `upy 部署失败 · my-compose-stack`
- **Body**:
  ```text
  项目: my-compose-stack
  版本: v1.1.0
  类型: Docker Compose 集成项目
  目录: /opt/my-compose-stack
  原因: healthcheck probe http://127.0.0.1:8080/health failed (timeout 30s)
  ```

##### 3. Backup Success Notification
- **Title**: `upy 备份成功 · my-api-service`
- **Body**:
  ```text
  项目: my-api-service
  归档: dump-my-api-service-20261019-120000.sql.gz (42.5 MB)
  目录: /opt/my-api-service
  耗时: 2.4s
  ```

##### 4. Backup Failure Alert
- **Title**: `upy 备份失败 · my-api-service`
- **Body**:
  ```text
  项目: my-api-service
  目录: /opt/my-api-service
  原因: exit status 1: mysqldump: Got error: 2003: Can't connect to MySQL server
  ```

#### Configuration Scopes

##### Scope 1: Project-Level (`<projectRoot>/deploy.yaml`)
Receives notifications solely for this project. Recommended for project-specific developers and testers:

```yaml
name: my-api-service
release:
  provider: github
  repository: owner/my-api-service

# Project-specific notification recipients
notify:
  bark:
    group: "my-api-service"
    sound: "chime"
    users:
      - name: "Frontend Developer"
        token: "DEV_TOKEN_1"
      - name: "QA Engineer"
        token: "DEV_TOKEN_2"
        sound: "glass"
```

> **Independent Project Muting**: If an internal test project does not require alerts, declare `notify: { enabled: false }` to silence all notifications for that project.

##### Scope 2: Machine Global Fallback (`~/.upy/config.yaml`)
Host-wide notification channels (e.g. SRE / DevOps on-call) active by default across all managed projects on the machine:

```yaml
github:
  token: ghp_xxxxxx

# Host-wide fallback notifications
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

#### Automatic Merging & Deduplication

- **Automatic Merging**: On deployment or backup completion, `upy` gathers channels from both the **current project's `deploy.yaml`** and the **global `~/.upy/config.yaml`**.
- **Smart Deduplication**: If the same recipient (Server URL + Device Key) is configured in both scopes, `upy` **deduplicates and sends only 1 notification**, prioritizing project-level custom attributes.

---

### 2. Project-Specific Webhook Notifications

Different projects often route notifications to distinct team bots (WeChat Work, Feishu/Lark, DingTalk, Slack, or internal event ingestion endpoints). **Because each project has unique destinations, Webhooks reside strictly in each project's `deploy.yaml` and do not belong in global host-wide configuration.**

#### 1. Configuration Example (`<projectRoot>/deploy.yaml`)

```yaml
name: my-api-service
release:
  provider: github
  repository: owner/my-api-service

# Project-scoped webhook (independent per project, zero global pollution)
notify:
  # Option A: Single URL direct declaration
  webhook: "https://ops.example.com/api/deploy-webhook"

  # Option B: Multi-endpoint array with HMAC-SHA256 signature and custom headers
  # webhook:
  #   - url: "https://ops.example.com/api/deploy-webhook"
  #     secret: "SEC_KEY_FOR_HMAC_SHA256"      # Computes X-Upy-Signature header
  #     headers:
  #       X-Environment: "production"
  #   - url: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
```

#### 2. Request Headers & HMAC-SHA256 Verification

When `secret` is provided, `upy` calculates an HMAC-SHA256 signature of the JSON payload and attaches it to the HTTP POST request:
- `Content-Type: application/json; charset=utf-8`
- `User-Agent: upy/<version>`
- `X-Upy-Event`: Event identifier (`deploy.success`, `deploy.failure`, `backup.success`, `backup.failure`)
- `X-Upy-Delivery`: Unique UUIDv4 generated per dispatch for idempotency & tracing
- `X-Upy-Signature`: Formatted as `sha256=<hex_hmac>` for cryptographic source verification

#### 3. Standard JSON Payload

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

> 💡 **Multi-Scale Elapsed Time Formatting**:  
> Across Bark notifications, Webhook payloads, and terminal outputs, elapsed durations are intelligently scaled: under 60 seconds displays as seconds (e.g. `3.2s`), over 60 seconds converts to minutes (e.g. `1m15s`), and over 60 minutes automatically starts with hours (e.g. `1h2m30s`).

---

## Server Daemon & Inbound Webhook

`upy server` provides a lightweight background daemon that serves as a secure, SSH-key-free inbound deployment endpoint for CI/CD pipelines (such as GitHub Actions or GitLab CI) alongside a TOTP 2FA protected Web console.

### 1. Starting & Managing the Daemon

```bash
# Start background daemon (default listens on 127.0.0.1:8999)
upy server start -d

# Custom port or listening address
upy server start -d --listen 127.0.0.1:8999

# Inspect daemon status and PID
upy server status

# Gracefully terminate daemon process
upy server stop
```

### 2. CI/CD Inbound Webhook Automation

Configure `webhook.secret` inside the project's `deploy.yaml`:

```yaml
name: my-api-service
type: system
target:
  os: linux
  arch: amd64

# Inbound webhook trigger configuration
webhook:
  secret: "whsec_your_strong_random_secret"
```

In your GitHub Actions workflow, trigger `POST /api/v1/webhook/{project}` after creating a release:

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

> 💡 **Zero SSH Keys & Non-Blocking Async Dispatch**:  
> - Eliminates the security hazard of saving host SSH private keys inside CI repository secrets.  
> - Inbound webhook verifies signature and immediately responds with `202 Accepted`, running the deployment job asynchronously in the background.  
> - Concurrent deployments for the same project are protected by inter-process mutex locks (returning `409 Conflict`) to prevent corrupt states.

### 3. TOTP 2FA Protected Web Console

`upy server` includes a lightweight, secure Web log console:

- **Terminal-only 2FA enrollment (`upy server auth`)**: Run the command in an interactive VPS terminal and scan its QR code with Google Authenticator or 1Password. The Web `/auth` route has been removed and always returns 404. Enrollment secrets are never served over HTTP.
- **Dynamic 6-Digit Code Login (`/`)**: Visiting root prompts for the 6-digit TOTP verification code. Successful authentication issues a 30-minute `HttpOnly` + `SameSite` secure session cookie.
- **Full Trace Browsing (`/logs`)**: Authenticated sessions can view deployment history across all registered projects, along with high-fidelity terminal ANSI color rendering.

To enroll an authenticator, use the same system user and `UPY_CONFIG` as the running server:

```bash
upy server auth
# With a custom configuration:
UPY_CONFIG=/path/to/config.yaml upy server auth
```

This reuses the existing `totp.json` secret without rotating it or invalidating enrolled authenticators. The old view quota no longer applies. QR output goes directly to an interactive terminal, never to upy logs or redirected stdout. The server can keep running. Invalid secret files produce an error instead of silently replacing the key.

The console uses a light mobile layout, with a centered 440px container on desktop. Lucide SVG icons and styles are embedded in the binary:

- **Overview (`/overview`)**: the default destination after login, showing host information, server uptime, projects, and recent deployment results.
- **Projects (`/projects`)**: search by name or directory; inspect current/previous versions, service units from `state.json`, and deployment history.
- **Logs (`/logs`)**: filter by project, status, or keyword, with 20 records per page and separate Server / upy runtime log views.
- **Log details (`/logs/{id}`)**: copy output or download the complete original file. Active tasks and runtime logs refresh every 3 seconds; expired sessions prompt sign-in.

All console pages, refreshes, and downloads share TOTP session protection. The console is read-only. A successful deployment is not a live application health check; archived logs without an explicit completion marker show an unknown result. Large logs preview only the last 256 KiB, while downloads include the entire original file.

The console supports PWA installation over HTTPS. Supporting browsers can install it as a standalone window; on iPhone / iPad, use the Share menu's Add to Home Screen action. It launches `/overview`, retaining TOTP login and the 30-minute cookie. The service worker caches only public icons and a data-free offline page, never authentication content, project data, logs, downloads, or APIs. Offline navigation shows a reconnect screen. Updates prompt for a user-initiated refresh instead of interrupting log reading.

---

## Unified Logging & the upy log Command

`upy` features a comprehensive structured logging system that records CLI invocations, server access, and deployment history with automated sensitive token redaction.

### 1. Storage Streams & Security Redaction

- `~/.upy/logs/upy.log`: Records every CLI command execution (timestamp, PID, command line, duration, exit code) with stripped ANSI codes for easy grep/awk parsing, and tokens/passwords automatically masked.
- `~/.upy/logs/server.log`: Records daemon lifecycle and HTTP access logs (client IP, HTTP method, path, response status code, elapsed time).
- `~/.upy/logs/<project>/<timestamp>.log`: Preserves the complete step-by-step terminal log for every deployment.

### 2. upy log Command Reference

```bash
# Interactive target picker (select Server, CLI, or project logs)
upy log

# Live stream server daemon and access logs (like tail -f)
upy log server -f
upy log -s -f

# View last 100 CLI execution history records
upy log upy -n 100

# View latest deployment log for a specific project
upy log my-api-service

# List all log files and disk consumption
upy log --list

# Truncate / clear a specific log file
upy log server -c
upy log upy -c
```

> 💡 **Anti-Loop Tail Safeguard**:  
> Running `upy log` commands automatically bypasses recording into `upy.log`, completely eliminating infinite self-tail loops when tracking with `upy log -f`.

---

## Self-Update

`upy` includes built-in cross-partition self-updates with in-place execution replacement:

```bash
# Check and upgrade to the latest version
upy update

# Switch or downgrade to a specific version
upy update v0.2.0

# Force re-download and overwrite
upy update --force

# Display current version and environment
upy version
```

> 💡 **Automatic Shell Completion Synchronization**:  
> Running `upy update` automatically scans and synchronizes shell completion rules across available host shells (`zsh`, `bash`, and `fish`). Newly introduced commands (such as `analysis`) are immediately available via Tab completion in new terminals without manual reinstallation.

---

## Command Cheat Sheet

| Command | Description |
| :--- | :--- |
| `upy` | Open interactive menu in any directory to select and deploy registered projects |
| `upy release [latest\|<version>]` | Pull and deploy Release from GitHub (interactive menu if version omitted) |
| `upy release list` | List GitHub Releases and asset availability for the current project |
| `upy deploy <project>` | Quickly deploy the latest release of a registered local project |
| `upy clean [project]` | Visually analyze and clean project data (bundles, old releases, or purge) |
| `upy backup [project]` | Run convention project backup (`backup.sh`), prune archives, and push alerts |
| `upy analysis [project]` | Analyze host projects, plot disk usage with 100% benchmark, and spotlight recent release |
| `upy server [start\|stop\|restart\|status\|auth]` | Start webhook server daemon and web console (`-d` for background mode) |
| `upy log [server\|upy\|<project>]` | View logs interactively or directly, with `-f` live stream and `-n` tail rows |
| `upy bundle <file>` | Deploy a local `.zip` bundle directly (offline mode, no GitHub connection) |
| `upy init [owner] <token>` | Initialize or configure GitHub personal/organization access tokens |
| `upy update [version]` | Self-update to latest release or switch to a target version |
| `upy version` | Print current version and check for newer releases |

---

## Agent Skill

Use [upy-project-adapter](skills/upy-project-adapter/SKILL.md) to generate application deployment configs, bundles, and GitHub Actions Releases for SPA, Docker/Compose, and system services. See [installation and usage](skills/README.md).

## License

MIT License
