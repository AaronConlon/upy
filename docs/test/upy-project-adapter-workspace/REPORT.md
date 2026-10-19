# upy-project-adapter Skill 验证

日期：2026-10-19。维护源：skills/upy-project-adapter。

## 已执行

- Skill frontmatter 校验通过。
- pack_bundle.py 的8项单元检查通过：布局、checksum、system权限、缺产物、越界路径、软链、敏感文件名、重复输出。
- static/docker compose/system 三份 manifest 样例通过 upy 当前 config.ParseManifest 实际解析。
- 真正构建 Linux amd64、arm64 最小 Go 产物，检查 ELF machine 与 ZIP executable 位，两者通过。
- 2个Agent场景 × 使用/不使用Skill，共4次独立生成；14个脚本、Python与YAML文件通过语法检查。
- system 两组 Agent 生成脚本均在临时最小 HTTP 项目执行双架构真实编译与打包；结果在 iteration-1/fixture-build-results.json。
- 契约断言：使用Skill 10/10，不使用Skill 9/10。基线缺明确shared/data与权限说明。每场景仅1次，没有统计显著性结论；断言不能覆盖所有质量风险。

## 发现与修正

源码复核发现初稿误将 releases.keep 当作有效保留策略。实际仅兼容读取，当前自动清理保留current和state.previous。已修正最终Skill契约与分发示例并省略该字段；iteration-1的Agent原始输出保留，未重跑完整评估，不能将它们当作最终推荐配置。

基线SPA全局contents:write范围较大，system基线没有checksum和明确shared/data策略；基线对清理只保留当前版的描述也不符合v0.2.0实现。按固定断言评分之外单独记录这些风险。

## 验证边界

没有实际目标应用仓库作为输入，Agent依据给定事实生成集成文件。Go真实构建用的是临时最小HTTP fixture，不是用户生产服务。SPA未执行依赖安装或真实Vite构建，Compose没有运行时测试。没有执行GitHub Actions、发布Release或服务器部署。不能据此宣称端到端部署通过。

当前平台未返回精确逐Agent时长和token用量，benchmark中标为null。构建fixture和原始文件支持人工复核。审阅页 review.html 由skill-creator自带viewer生成；可下载填写反馈后继续迭代。
