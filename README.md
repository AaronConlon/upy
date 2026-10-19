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
- [Multi-Recipient Bark Notifications](#multi-recipient-bark-notifications)
- [Self-Update](#self-update)
- [Command Cheat Sheet](#command-cheat-sheet)
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
- **Cross-Project Global Dispatching**: Run `upy` in any directory to interactively list and deploy across all managed projects on the host.
- **Convention Backups**: Zero-config project discovery (`backup.sh`), exclusive concurrency locks, automatic retention rotation, and multi-project batch runs.
- **Multi-Recipient Bark Push Alerts**: Instant mobile notifications for both deployments and backups powered by [Bark](https://bark.day.app/) (an open-source iOS push notification service). Supports project-level routing in `deploy.yaml`, global fallback in `~/.upy/config.yaml`, flexible multi-recipient broadcasting (tokens, full URLs, custom sounds/groups), and automatic deduplication.

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

When building artifacts in CI/CD (e.g., GitHub Actions), package your application along with a manifest `deploy.yaml` inside `bundle.zip`:

#### A. Frontend Static Applications (SPA)

Bundle structure:
```text
bundle.zip
├── deploy.yaml
└── dist/
    ├── index.html
    ├── assets/
    └── ...
```

Manifest `deploy.yaml`:
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
├── deploy.yaml
├── docker-compose.yml
└── ...
```

Manifest `deploy.yaml`:
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
├── deploy.yaml
└── bin/
    └── my-api
```

Manifest `deploy.yaml`:
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

## Multi-Recipient Bark Notifications

[Bark](https://bark.day.app/) is a lightweight, open-source push notification service for Apple iOS devices. By using simple HTTP requests, you can deliver instant push notifications directly to iPhones and iPads without setting up complex third-party bot frameworks, heavy webhook middleware, or message queues.

`upy` integrates Bark natively into the deployment and backup lifecycles, enabling instant mobile alerts for developers and DevOps teams.

### Highlights

- **Dual-Level Merging & Smart Deduplication**: Configure team-wide channels in `~/.upy/config.yaml` and project-specific channels in `deploy.yaml`. `upy` merges both sources and automatically deduplicates overlapping recipients so no user receives duplicate alerts.
- **Multi-Recipient Fan-Out**: Effortlessly notify multiple teammates or on-call engineers simultaneously with a single release or backup event.
- **Flexible Syntax Formats**: Supports plain device tokens (`KEY`), full URLs (`https://api.day.app/KEY` or self-hosted Bark domains), string lists, or detailed channel objects with custom sounds, groups, and sensitivity levels.
- **Independent Project Muting**: Silence notifications for a specific project at any time by declaring `notify: { enabled: false }` in its `deploy.yaml`.
- **Rich Context & Diagnostics**: Notifications include project name, deployed version, stack type, elapsed execution time, and exact failure diagnostics if an error occurs.

### Three Common Configuration Patterns

#### 1. Minimal String List (Recommended for Teams)
Directly list individual Device Keys or full URLs copied from the Bark app:

```yaml
notify:
  bark:
    - "DEVICE_KEY_ALICE"
    - "DEVICE_KEY_BOB"
    - "https://api.day.app/DEVICE_KEY_CAROL" # Full URL is automatically parsed
```

#### 2. Shared Group & Sound with Batch Tokens
When all teammates share the same notification sound, group name, or self-hosted server:

```yaml
notify:
  bark:
    serverUrl: "https://api.day.app" # Default official API, or private self-hosted URL
    group: "payment-api"             # Push group title
    sound: "chime"                   # Custom ringtone (bell, chime, glass, minuet, etc.)
    tokens:
      - "DEVICE_KEY_ALICE"
      - "DEVICE_KEY_BOB"
```

#### 3. Detailed Channel Objects (Per-Recipient Customization)
When individual members prefer custom tags, ringtones, or self-hosted Bark instances:

```yaml
notify:
  bark:
    - name: "Frontend Lead"
      token: "DEVICE_KEY_FRONTEND"
      sound: "glass"
    - name: "DevOps On-Call"
      serverUrl: "https://bark.mycorp.internal" # Private self-hosted Bark instance
      token: "DEVICE_KEY_OPS"
      sound: "minuet"
      level: "timeSensitive"                     # Alert level (active / timeSensitive / critical)
```

### Configuration Scopes

#### Scope 1: Project-Level (`<projectRoot>/deploy.yaml`)
Receives notifications solely for this project. Recommended for project-specific developers and testers:

```yaml
name: my-api-service
release:
  provider: github
  repository: owner/my-api-service

# Project-specific notification recipients
notify:
  bark:
    - "DEV_TOKEN_1"
    - "DEV_TOKEN_2"
```

> **Independent Project Muting**: If an internal test project does not require alerts, declare `notify: { enabled: false }` to silence all notifications for that project.

#### Scope 2: Machine Global Fallback (`~/.upy/config.yaml`)
Host-wide notification channels (e.g. SRE / DevOps on-call) active by default across all managed projects on the machine:

```yaml
github:
  token: ghp_xxxxxx

# Host-wide fallback notifications
notify:
  enabled: true
  bark:
    - name: "SRE Lead"
      token: "SRE_DEVICE_KEY"
      sound: "bell"
```

### Automatic Merging & Deduplication

- **Automatic Merging**: On deployment or backup completion, `upy` gathers channels from both the **current project's `deploy.yaml`** and the **global `~/.upy/config.yaml`**.
- **Smart Deduplication**: If the same recipient (Server URL + Device Key) is configured in both scopes, `upy` **deduplicates and sends only 1 notification**, prioritizing project-level custom attributes.

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
| `upy bundle <file>` | Deploy a local `.zip` bundle directly (offline mode, no GitHub connection) |
| `upy init [owner] <token>` | Initialize or configure GitHub personal/organization access tokens |
| `upy update [version]` | Self-update to latest release or switch to a target version |
| `upy version` | Print current version and check for newer releases |

---

## Agent Skill

Use [upy-project-adapter](skills/upy-project-adapter/SKILL.md) to generate application deployment configs, bundles, and GitHub Actions Releases for SPA, Docker/Compose, and system services. See [installation and usage](skills/README.md).

## License

MIT License
