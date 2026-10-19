# 完整接入示例

static-pnpm：owner/demo-web、Vite dist、Node20、pnpm9.15.0。go-system：owner/demo-api、Go1.25、./cmd/api、8080 /health、Linux amd64/arm64。

按示例目录结构复制到应用，并将 Skill 根目录 scripts/pack_bundle.py 复制到应用 scripts/upy/pack_bundle.py。示例只适用于列出的项目事实；替换真实名称、仓库、工具链、入口、端口和服务器 root，不能直接覆盖现有 workflow。

Go 示例已在临时最小 HTTP 项目执行两种架构的编译与打包，检查 ELF 和 UNIX 权限；SPA 示例仅做语法检查，未安装真实 Vue 依赖或运行构建。未触发 GitHub Actions，未发布或部署。保持现有项目 tests，在 workflow 对应步骤执行。

server 配置默认省略 releases.keep：该字段不能决定当前清理保留数量。示例契约参见 references/contracts.md。
