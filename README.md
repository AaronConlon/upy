# Upy (Go)

轻量级 Web 项目部署 CLI 的 **Go 实现**（原 Bun/TypeScript 版本在 `WeiaiHealth-Software/uply`）。

- 单二进制，无运行时依赖，约 **9MB**（Bun 版约 60MB）
- 功能与 Bun 版一致：`release` / `bundle` / `version` / `update`
- 支持两类部署：静态 SPA（切 current 软链）和 Docker Compose 集成项目
- 私有 GitHub Release 部署、bundle 本地缓存复用、SQLite 数据卷持久化
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

## 自更新

已安装 `upy` 后，可直接通过内置命令自更新到最新版本：

```bash
upy update
```

若目标 release 属于私有仓库，需配置带有 `Contents: Read-only` 权限的 GitHub Token：

```bash
export DEPLOY_GITHUB_TOKEN='你的token'
upy update
```

## 部署通知

用户级配置放在家目录，不进项目仓库。GitHub token 优先读这里的 `github.token`，环境变量 `DEPLOY_GITHUB_TOKEN` 只做兜底，避免不同 shell 里变量不一致：

```bash
mkdir -p ~/.upy
cp config.example.yaml ~/.upy/config.yaml
```

示例：

```yaml
github:
  token: github_pat_xxx
notify:
  bark:
    - name: phone
      serverUrl: https://api.day.app
      token: your-bark-device-key
      group: upy.phone
```

可写多条 Bark。部署成功、部署失败会逐条发送；通知本身失败只打警告，不影响部署结果。也可用环境变量 `UPY_CONFIG` 指定配置文件路径。

