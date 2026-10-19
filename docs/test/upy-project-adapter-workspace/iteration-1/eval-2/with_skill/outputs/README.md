# owner/demo-api 接入 upy system

这些是供复制到 **owner/demo-api 应用仓库**的具体集成文件，不是 upy CLI 自身发布配置。依据技能的 upy v0.2.0 契约生成；没有应用源码、服务器或可用的 upy 命令，因此没有执行 Go 构建、应用测试、打包、Release 发布、部署或故障演练。没有推送任何内容。生成的 workflow 和脚本中的检查将在集成后执行，不能视为已通过。

## 文件与约定

- `deploy/upy/server.deploy.yaml.example`：服务器配置，复制到 `/srv/demo-api/deploy.yaml`，选择 arm64 资产，服务身份使用已有 `api:api`。
- `scripts/upy/build-bundle.sh`：Go 交叉编译、独立 staging、ELF 检查、manifest 生成、打包和 ZIP 检查。
- `scripts/upy/pack_bundle.py`：从技能原样复制的标准库打包工具，保留 UNIX 执行权限并生成 SHA256。
- `.github/workflows/upy-release.yml`：`v*` tag 触发，Go 1.25.x，amd64/arm64 全部成功后统一发布。
- `README.md`：集成和运维说明；复制到应用时可保存为 `docs/upy-deployment.md`。

选择 system 是因为项目为无需 CGO 的 Go 常驻服务。已知入口 `./cmd/api`、8080、`/health`，没有证据表明支持 `--port` 或某个端口环境变量，故命令仅为 `["./bin/server"]`。应用必须自身监听 8080；如需配置，先核对应用真实接口再调整。部署 root `/srv/demo-api` 和 unit `demo-api.service` 是本交付采用的具体约定，可在集成前统一调整。

## 应用仓库集成

按上述路径复制文件，保留仓库已有规则和无关变更。需要已提交的 `go.mod` 和项目所需的 `go.sum`，Go 版本须兼容 1.25；`-mod=readonly` 拒绝构建时自动修改依赖。若依赖私有模块，需要另行配置最小读取权限，不把凭据放进 bundle。Go 标准库、应用依赖可在构建期下载，服务器不需要 Go 编译器。

在应用仓库根目录先运行：

```sh
CGO_ENABLED=0 go test -mod=readonly ./...
bash scripts/upy/build-bundle.sh 1.0.0 amd64 artifacts
bash scripts/upy/build-bundle.sh 1.0.0 arm64 artifacts
(cd artifacts && sha256sum -c bundle-linux-amd64.zip.sha256 && sha256sum -c bundle-linux-arm64.zip.sha256)
```

这些是待执行步骤。本地需 Bash、Python 3、Go 1.25；校验示例的 sha256sum 通常在 Linux 可用，macOS 可用 `shasum -a 256 -c <文件>`。bundle 每份仅包含根目录 `deploy.yaml`（JSON 格式，upy YAML parser 可读取）与 `bin/server`。manifest 版本为 tag 去除 v，例如 v1.0.0 → 1.0.0；包含 Linux 目标架构、system 命令和顶层 30s 健康检查。不包含服务器 deploy.yaml、token、service.env、源码或数据。

Release 精确资产名：

- `bundle-linux-amd64.zip`
- `bundle-linux-amd64.zip.sha256`
- `bundle-linux-arm64.zip`
- `bundle-linux-arm64.zip.sha256`

集成文件提交并审核后，由维护者自行推送新 tag（例如 v1.0.0）触发 workflow。本次没有执行这些操作。只支持 tag push，接受无 build metadata 的 SemVer（可带预发布标识），拒绝无效版本。测试运行于 runner 架构，arm64 是交叉编译和 ELF/ZIP 静态校验，不代表 arm64 运行测试。预发布 tag 标记为 prerelease；首次验证建议使用明确 tag，不能假定 latest 会选择 prerelease。

两个构建 job 都成功才发布；构建 job 只有 contents:read，release job 才有 contents:write，以 CI 的 github.token 发布。已有同 tag Release 时失败，不覆盖资产；部分失败或发布中断后先人工检查远端，默认采用新版本，不删除重发冒充不可变发布。SHA256 供人工校验，不能假定 upy 自动校验。

## 服务器准备与环境认证

目标必须是 Linux arm64、systemd 为 PID1，有 systemctl，安装经核对支持 system/target 的 upy。部署执行者需具备管理 unit 和部署目录的权限；api 只是服务运行身份。确认 `uname -m` 为 aarch64、`getent passwd api` 和 `getent group api` 有效，并检查 `ps -p 1 -o comm=`。不要另行创建与 upy 冲突的 systemd unit。

由管理员准备 root 和持久目录；以下是供实际服务器执行的步骤，**本次未执行**：

```sh
sudo install -d -m 0755 /srv/demo-api /srv/demo-api/shared
sudo install -d -o api -g api -m 0750 /srv/demo-api/shared/data
sudo install -m 0644 deploy/upy/server.deploy.yaml.example /srv/demo-api/deploy.yaml
```

管理员在 `/srv/demo-api/shared/service.env` 写入应用真实支持的环境变量，设为 `root:api`、0640；不要把真实环境文件提交仓库或放入 ZIP。该文件优先于部分版本兼容的 shared/.env。若应用有持久数据需求，应让应用通过其真实支持的配置读取绝对路径 `/srv/demo-api/shared/data`；本交付不虚构 DATA_DIR 等变量。确认 api 能遍历部署目录、读取发布内容、执行 binary、写入 shared/data；日志写 journal 或明确的持久目录，不写版本目录。

服务器通过 **DEPLOY_GITHUB_TOKEN 环境变量**认证（按需用于私有仓库或提高 API 配额）。使用仅对 owner/demo-api 有 Release 读取权限的凭据；不要假定 GITHUB_TOKEN 会被 upy 读取。CI github.token 与服务器长期凭据是不同认证链。在真正执行 upy 的部署进程环境中注入 token；若用 sudo，确认变量保留，不要让普通 shell 的变量在提权时丢失。示例在 Bash 管理员会话交互读取，不把 token 写在命令历史：

```bash
read -r -s -p 'GitHub Release read token: ' DEPLOY_GITHUB_TOKEN
printf '\n'
export DEPLOY_GITHUB_TOKEN
# 在同一会话按下节完成部署，结束后执行：
# unset DEPLOY_GITHUB_TOKEN
```

无人值守时由服务器的受保护凭据机制向部署进程注入环境变量，禁止打印 token 或启用 shell trace。不要把 token 填入服务器 deploy.yaml 或 service.env。公开仓库可按实际需求不设置 token。

## 首次部署与升级（待核验后执行）

当前环境没有 upy，无法检查实际 `upy --help`。以下命令来源于技能 v0.2.0 契约；操作者必须先在目标服务器检查 `upy --help`、`upy release --help`、`upy bundle --help` 和安装版本，确认语义与权限再执行。它们都是**真实部署**，不存在这里所暗示的 dry-run。

```sh
upy release v1.0.0 --root /srv/demo-api
```

离线部署可自行下载 arm64 ZIP 和同名 SHA256，在同一目录人工核验，再使用已核对的命令：

```sh
sha256sum -c bundle-linux-arm64.zip.sha256
upy bundle ./bundle-linux-arm64.zip --root /srv/demo-api
```

首次上线后检查 `systemctl status demo-api.service`、`journalctl -u demo-api.service`、`curl --fail http://127.0.0.1:8080/health`，并通过应用现有版本接口或实际业务行为确认运行的是目标版本；单纯 HTTP 200 不足以证明部署成功。应用需在初始化成功后才健康，并处理 SIGTERM，在 systemd 停止时限内退出。HTTPS 与对外反向代理另行配置；服务器还可能需要 CA certificates、时区数据等，须按应用实际依赖核对。

升级使用新的版本 tag，例如 `upy release v1.0.1 --root /srv/demo-api`，不要默认使用 --force。system 升级可能停止旧进程再启动新进程，不保证零停机。`releases.keep: 3` 只表示版本保留数量，不保证可成功回滚。升级前备份 shared 数据及环境配置，并核对数据迁移兼容性。

## 集成后的失败演练与限制

在获授权的隔离 Linux arm64 测试服务器上，准备已知健康版本，再分别演练健康失败、启动失败、端口占用、错误用户/组、错架构、缺 binary、同版本重试、连续升级与清理、部署中断和其他 unit 隔离。每次记录旧服务是否继续可用、unit 状态、journal、实际业务版本与 shared 数据，检查失败是否影响原服务。保留已知健康 ZIP；需要恢复时先核对目标版本实际支持的恢复流程及数据库兼容性，不编造 rollback/status/validate 命令，不承诺重新部署旧 ZIP 一定成功。身份失效等破坏性场景只在测试机演练。

本次已执行并通过：build-bundle.sh 的 bash -n 语法检查、pack_bundle.py 及三个内嵌 Python 代码块的 AST 语法解析、服务器配置与 Actions workflow 的 YAML 解析。这些检查不验证目标 upy 的运行行为。没有真实编译、ZIP 生成、Actions 运行、服务器 E2E 或回滚结果；upy 安装版本、应用环境变量、数据需求与运行依赖仍需集成方核对。
