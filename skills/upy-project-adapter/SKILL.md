---
name: upy-project-adapter
description: 为应用项目接入 upy 部署，生成服务器配置、bundle upy.yaml、GitHub Actions 构建、GitHub Release 资产和 CI Webhook 自动部署。用户提到适配 upy、upy 配置、发布 bundle、Release 自动部署、CI webhook 部署、Go 二进制 systemd 部署、SPA 或 Docker Compose 接入 upy 时使用，包括检查和修复已有接入。适用于应用仓库，不要与 upy CLI 自身的发布流程混淆。
---

# upy 项目接入

目标是交付可运行、可验证的接入文件，而不是只解释方案。使用中文说明，保留项目原有技术栈与包管理器。

## 1. 识别项目与目标

先读取项目规则、现有 workflows、构建脚本、lockfile、运行入口、健康检查、数据与环境变量配置。已有文件先检查再修改，保留无关工作。

确认 GitHub owner/repository、目标 Linux 架构、upy 版本、部署 root、端口和健康路径。仓库信息可以从 remote 读取。能从代码得出的不要反复问；无法推断且影响正确性的事实才询问。暂未确定的服务器信息写成明确占位值并列出待办，不能声称已完成真实部署。

选择类型：
- 纯静态产物（含 SPA）：`static`。Next.js SSR、Python 服务不能因为含前端就归为 static。
- 已有 Dockerfile / Compose、多运行时依赖：`docker`，优先保留已有容器方案。
- Linux 原生常驻服务，尤其 Go binary：`system`，由 systemd 托管。先核对目标 upy 与系统能力。
- Java JAR、Python 可以通过 system 调用服务器已有 runtime，但必须明确 Java/Python 版本与依赖交付方式；venv 不跨机器打包。依赖复杂时建议 Docker 并说明取舍。

读取 [配置契约](references/contracts.md)，再按需读取 [静态与容器](references/static-docker.md)、[system](references/system.md)、[Release 流程](references/release.md)。契约基于 upy v0.8.0 源码；目标版本的实现优先于本文。不要把工作区未发布功能写成已发布能力。

upy 支持 `upy server` 的 CI 入站 Webhook。若用户要求 CI 发布后自动部署，确认服务器已运行 `upy server`、项目已经成功手动部署过一次并登记到本机项目列表；服务器项目配置中的 `webhook.secret` 用于验证 CI 请求。不要把项目配置里的入站 `webhook.secret` 与 `notify.webhook` 的出站通知端点混为一谈。

upy v0.8.0 起支持纯 Go 原生 AES-256-GCM bundle 强解密（`crypto.password`）。若项目希望在组织公开 Release 分发以借力国内加速镜像免 Token 跑满带宽，同时保护业务代码私密性，可在 CI 中通过专用的超轻量 CLI `upy-crypt`（仅 ~2MB，无 Web/控制台冗余代码，免下载全量 upy）生成加密包，并在服务器 `deploy.yaml` 中配置 `crypto.password`，部署时由 upy 自动防篡改校验并透明解密。


## 2. 生成接入文件

推荐布局（根据现有项目调整）：
```
deploy/upy/server.deploy.yaml.example  # 复制到服务器 <root>/deploy.yaml
scripts/upy/build-bundle.sh           # 项目实际构建 + 白名单 staging
scripts/upy/pack_bundle.py            # 从本 Skill scripts 复制
.github/workflows/upy-release.yml
docs/upy-deployment.md
```
Bundle manifest 在 staging 根目录生成官方文件名 `upy.yaml`；upy v0.4.0 仍兼容读取旧版 `deploy.yaml`，但新接入不要继续生成旧名。服务器配置仍是 `<root>/deploy.yaml`，不得误放入 ZIP。不要提交真实 token、service.env、数据库或整个工作目录。构建 staging 只复制已列明的产物和运行必需文件。

可参考 [完整接入示例](assets/examples/README.md) 中的配置、构建脚本和 workflow。使用 assets 中 server 和 manifest 样例作结构参考，必须替换项目名、仓库、目录、端口、入口等。生成 workflow 时遵循 references/release.md，不能只交付未替换的模板。

若要求 CI 发布后自动部署，workflow 必须在 Release 创建成功之后再执行 deploy job，向 `POST /api/v1/webhook/{project}` 发送精确 tag（例如 `v1.2.3`）作为 `target`；优先用项目级 secret 计算 `X-Hub-Signature-256: sha256=<hex>`，不要把密钥写入仓库。服务器端 `<root>/deploy.yaml` 的 `webhook.secret` 与 GitHub Actions Secret（例如 `UPY_WEBHOOK_SECRET`）必须一致，Webhook URL 必须包含与配置 `name` 完全一致的项目名。该接口是异步部署，`202 Accepted` 只代表任务已创建，不代表服务已完成升级；不要在 workflow 中把它描述成部署 E2E 已通过。

打包工具用标准库读取 **JSON 格式** manifest（文件名必须为 `upy.yaml`；JSON 是 YAML 的子集）。调用方式：
```
python3 scripts/upy/pack_bundle.py --stage .upy-stage --output artifacts/bundle.zip
```
如果项目使用 YAML manifest，保留 YAML 并使用 zip 工具或项目已有 YAML parser 校验；不要直接将 YAML 传给此 JSON 工具。

## 3. 验证并交付

在可用环境执行项目构建、打包、ZIP 内容检查。system 检查 ELF 平台与 executable 权限；Compose 执行 `docker compose ... config --quiet`，缺 Docker 时明确未验证。不能用编译成功代替部署测试。

`upy bundle <zip> --root <dir>` 是**真实部署命令，不是打包或 dry-run**。只在授权的隔离目标上执行；system 要 Linux + systemd PID1，不能把 macOS 编译通过当作真实运行通过。没有服务器时交付步骤与未验证项。

部署说明必须包含：服务器配置与 bundle 区别、运行依赖、shared 环境变量与持久化、精确资产名、认证、首次部署（或后续跨目录调度 `upy deploy <project>`）、升级与失败演练；若采用加密 bundle 分发，写明在 CI 中调用专用的超轻量 CLI `upy-crypt`（或 `upy encrypt`）加密以及在服务器 `deploy.yaml` 中配置 `crypto.password`；启用 CI Webhook 时还要写明 `server.listen`/反向代理、`POST /api/v1/webhook/{project}`、HMAC 或 Bearer 鉴权、Actions Secret、`202` 异步语义及 `401`/`404`/`409` 的处理。检查实际 `upy --help` 后再引用命令，不能凭空生成 `upy rollback` / `upy status` / `upy validate`。

交付时列出文件、选择依据、执行过的验证和待办。只有用户要求发布/部署时才向远端推送或运行服务；单纯接入生成不需要发布 Release。回滚与 clean 相关历史问题需用目标版本重测，不能承诺零停机或绝对安全回滚。
