# Uply (Go)

轻量级 Web 项目部署 CLI 的 **Go 实现**（原 Bun/TypeScript 版本在 `WeiaiHealth-Software/uply`）。

- 单二进制，无运行时依赖，约 **9MB**（Bun 版约 60MB）
- 功能与 Bun 版一致：`release` / `bundle` / `version` / `update`
- 支持两类部署：静态 SPA（切 current 软链）和 Docker Compose 集成项目
- 私有 GitHub Release 部署、bundle 本地缓存复用、SQLite 数据卷持久化
- Release 资产名不固定也支持：先精确匹配配置的 `asset`，未命中时按「项目名 + 版本号」模糊匹配（如 `test-deploy-website-v0.5.1-20260815.zip`）
- 中文 CLI（横幅 / 目录树 / 颜色 / emoji）

## 构建

```bash
./scripts/build.sh                     # 当前平台, 版本取最近 git tag
UPLY_VERSION=v0.1.11 ./scripts/build.sh
```

当前公开仓库暂不配置 GitHub Actions，也不上传 Release。需要本地测试时，直接运行上面的构建脚本即可；后续有稳定的发布节奏和可用额度后，再补充自动化构建与发布。

## 自更新

```bash
export DEPLOY_GITHUB_TOKEN='你的token'
uply update
```

手工构建出的资产建议沿用 `uply-{linux|darwin}-{x64|arm64}` 命名，便于后续接入 Release 和自更新流程。


## 部署通知

用户级配置放在家目录，不进项目仓库。GitHub token 优先读这里的 github.token，环境变量 DEPLOY_GITHUB_TOKEN 只做兜底，避免不同 shell 里变量不一致：

    mkdir -p ~/.uply
    cp config.example.yaml ~/.uply/config.yaml

示例：

    github:
      token: github_pat_xxx
    notify:
      bark:
        - name: phone
          serverUrl: https://api.day.app
          token: your-bark-device-key
          group: uply.phone

可写多条 Bark。部署成功、部署失败会逐条发送；通知本身失败只打警告，不影响部署结果。也可用环境变量 UPLY_CONFIG 指定配置文件路径。
