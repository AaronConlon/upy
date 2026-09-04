# Upy (Go)

轻量级 Web 项目部署 CLI 的 **Go 实现**（原 Bun/TypeScript 版本在 `WeiaiHealth-Software/uply`）。

- 单二进制，无运行时依赖，约 **9MB**（Bun 版约 60MB）
- 功能与 Bun 版一致：`release` / `deploy` / `bundle` / `version` / `update` / `init`
- 支持两类部署：静态 SPA（切 current 软链）和 Docker Compose 集成项目
- Compose 部署顺序安全：新镜像构建成功后先对上一版本执行 `down --remove-orphans` 释放端口与容器名，再启动新版本；健康检查失败自动停掉失败新栈并 `up -d` 回滚上一版本
- 私有 GitHub Release 部署、多 Token 组织归属自动路由、bundle 本地缓存复用、SQLite 数据卷持久化
- Release 资产名不固定也支持：先精确匹配配置的 `asset`，未命中时按「项目名 + 版本号」模糊匹配（如 `test-deploy-website-v0.5.1-20260815.zip`）
- 中文 CLI（横幅 / 目录树 / 颜色 / emoji）

## 一键安装

在 Linux / macOS 上，运行以下命令自动识别架构并安装到 `/usr/local/bin`：

```bash
curl -fsSL https://raw.githubusercontent.com/AaronConlon/upy/main/install.sh | bash
```

> 如果 `/usr/local/bin` 需要管理员权限，脚本会自动调用 `sudo` 完成安装。

### 自定义安装选项

支持通过环境变量指定安装目录或固定版本：

```bash
# 安装到自定义目录（如无需 sudo 的 ~/.local/bin）
INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/AaronConlon/upy/main/install.sh | bash

# 安装指定版本
VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/AaronConlon/upy/main/install.sh | bash
```

安装器会自动识别并提示当前 shell；对 zsh、bash、fish 自动配置 Tab 一级命令补全（`release`、`deploy`、`bundle`、`version`、`update`、`init`）。新开终端后即可使用，例如 `upy rel<Tab>`。

每个 Release 还会附带 `SHA256SUMS`。安装器与 `upy update` 都会先校验当前平台二进制的 SHA-256；缺少清单或校验不一致时会拒绝替换本地二进制。

## 初始化与多 Token 配置

`upy` 内置了 `init` 命令，支持快速创建用户配置文件，并**支持多 Token 归属管理**。当拉取不同组织或个人私有仓库时，自动根据仓库的 Owner 匹配对应的 Token。

### 1. 命令行快速初始化

```bash
# 设置全局默认 / 兜底 Token
upy init ghp_your_default_token

# 为指定组织设置专属 Token (如 WeiaiHealth-Software)
upy init WeiaiHealth-Software ghp_org_token

# 为个人用户名设置专属 Token (如 AaronConlon)
upy init AaronConlon ghp_personal_token

# 交互式引导 (自动提示归属与密码密文输入)
upy init
```

命令会自动创建或追加合并到 `~/.upy/config.yaml`，并将文件权限设置为 `600` 保护敏感信息。

### 2. 配置文件说明 (`~/.upy/config.yaml`)

可复制模板快速新建：

```bash
mkdir -p ~/.upy
cp config.example.yaml ~/.upy/config.yaml
```

配置文件内容示例：

```yaml
github:
  # 全局默认 / 兜底 token (可选)
  token: github_pat_default_xxx

  # 多组织 / 个人归属 token 映射 (可选)
  # 从对应组织或个人仓库 (如 WeiaiHealth-Software/app) 拉取时优先使用
  tokens:
    WeiaiHealth-Software: github_pat_org_xxx
    AaronConlon: github_pat_personal_xxx

notify:
  bark:
    - name: phone
      serverUrl: https://api.day.app
      token: your-bark-device-key
      group: upy.phone

# 由 upy release 成功后自动维护；可在任意目录执行 upy 选择项目
projects:
  - name: codia
    root: /opt/apps/codia
```

**Token 解析与匹配优先级**：
1. `github.tokens.<owner>`（匹配项目所在组织或个人用户名，大小写不敏感）
2. `github.token`（全局默认兜底）
3. 环境变量 `DEPLOY_GITHUB_TOKEN`（仅作最后兜底）

## 本地构建与 CI 发布

### 本地构建

```bash
./scripts/build.sh                     # 当前平台, 版本取最近 git tag
UPLY_VERSION=v0.1.0 ./scripts/build.sh # 指定版本构建
```

构建出的产物位于 `dist/upy-<os>-<arch>`。

### CI 跨平台发布 (GitHub Actions)

项目已配置 GitHub Actions 跨平台构建工作流（`.github/workflows/release.yml`）：

- **触发条件**：推送 tag（如 `v0.1.0`）或在 Actions 页面手动触发 `workflow_dispatch`。
- **构建矩阵**：并发构建 4 大平台二进制：
  - `upy-linux-x64`
  - `upy-linux-arm64`
  - `upy-darwin-x64`
  - `upy-darwin-arm64`
- **自动化发布**：自动创建 GitHub Release 并上传 4 平台二进制资产。

```bash
# 发布新版本示例
git tag -a v0.1.0 -m "release v0.1.0"
git push origin v0.1.0
```

## 自更新与版本切换

已安装 `upy` 后，可通过内置命令一键升级或切换版本：

```bash
# 1. 自动检查并升级到 Releases 最新版本
upy update

# 2. 切换/更新/回滚到指定版本
upy update v0.1.0

# 3. 强制重新下载并覆盖当前版本
upy update --force
upy update v0.1.0 --force
```

`upy update` 同样会输出识别到的 shell，并同步 Tab 命令补全。

### 私有仓库授权机制

若 `upy` 所在的仓库为**私有仓库 (Private Repository)**，自更新会自动按以下顺序提取 Token 访问 GitHub API 进行鉴权与资产下载：

1. `~/.upy/config.yaml` 中为归属 `AaronConlon` 配置的专属 Token (`github.tokens.AaronConlon`)
2. 配置文件中的全局默认 Token (`github.token`)
3. 环境变量 `DEPLOY_GITHUB_TOKEN`

> 推荐只需运行一次 `upy init AaronConlon <token>` 即可无缝自更新与切换版本。

## 本地项目快捷部署

每次 `upy release` 成功后，upy 会将项目名称和根目录保存进 `~/.upy/config.yaml` 的 `projects` 字段。直接执行 `upy` 时，会先过滤并清理根目录已不存在、不是目录或缺少 `deploy.yaml` 的项目，再交互选择项目，并进入常规的 Release 版本选择。

```bash
# 在任意目录选择已部署项目，再选择要部署的版本
upy

# 直接为已登记项目拉取并部署最新正式版本
upy deploy codia
```
