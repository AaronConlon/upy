# 完整接入示例

static-pnpm：owner/demo-web、Vite dist、Node20、pnpm9.15.0。go-system：owner/demo-api、Go1.25、./cmd/api、8080 /health、Linux amd64/arm64。

按示例目录结构复制到应用，并将 Skill 根目录 scripts/pack_bundle.py 复制到应用 scripts/upy/pack_bundle.py。示例生成的 Bundle 清单统一使用 `upy.yaml`；服务器侧仍使用 `<root>/deploy.yaml`。示例只适用于列出的项目事实；替换真实名称、仓库、工具链、入口、端口和服务器 root，不能直接覆盖现有 workflow。

Go 示例已在临时最小 HTTP 项目执行两种架构的编译与打包，检查 ELF 和 UNIX 权限；SPA 示例仅做语法检查，未安装真实 Vue 依赖或运行构建。未触发 GitHub Actions，未发布或部署。保持现有项目 tests，在 workflow 对应步骤执行。

server 配置默认省略 releases.keep：该字段不能决定当前清理保留数量。示例契约参见 references/contracts.md。

两个 workflow 都提供可复制的 CI `deploy` job：它在 GitHub Release 创建成功后调用 `POST /api/v1/webhook/{project}`，发送精确 tag 并使用 `X-Hub-Signature-256` HMAC。启用它时，在仓库 Actions Secrets 配置 `UPY_WEBHOOK_URL` 与 `UPY_WEBHOOK_SECRET`，并在服务器 `<root>/deploy.yaml` 设置相同的 `webhook.secret`；不需要自动部署时删除整个 `deploy` job。`202 Accepted` 只代表异步任务已创建；项目必须先手动成功部署一次，且 `upy server` 需要通过 HTTPS reverse proxy 可被 GitHub Actions 访问。
