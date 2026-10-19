# owner/demo-api 接入 upy system

本目录为待集成文件，依据仓库 internal 的真实契约生成。没有目标应用源码或服务器；未执行真实构建、应用测试、Actions、发布或部署，也没有推送任何内容。下列命令仅供维护者未来执行。

## 文件放置

将 `.github/workflows/release.yml`、`scripts/package-release.sh`、`packaging/deploy.yaml.tmpl` 按相同相对路径复制进 owner/demo-api。脚本通过 bash 调用，不依赖 executable bit。服务器配置 `server/deploy.yaml` 单独复制到 `/opt/demo-api/deploy.yaml`，不要作为源码根目录或 ZIP 内清单使用。路径 `/opt/demo-api` 是本方案选定的部署根目录，可统一替换。

应用事实：Go 1.25、main 为 `./cmd/api`、CGO 关闭、监听 8080、GET `/health` 返回 2xx。没有虚构 main.Version 等符号或启动参数。应用是否需要额外环境变量及其名称，需集成者根据源码补齐；当前方案不假设应用识别 PORT。

## 打包与发布

在目标应用仓库使用 Go 1.25.x、Python 3，运行：

```sh
bash scripts/package-release.sh v1.2.3
```

生成且仅发布两个 Release ZIP：`demo-api-linux-amd64.zip`、`demo-api-linux-arm64.zip`。每份 ZIP 根部为 `deploy.yaml` 和 `bin/demo-api`，清单 version 为原始 tag（如 v1.2.3），target 分别为 linux/amd64 和 linux/arm64。无 CGO 交叉构建，不会在构建机运行 arm64 二进制。未注入应用版本变量；若健康响应含非空 version，必须与清单 version 一致，若含 pid 则须对应服务进程。

Actions 在推送 v* tag 后运行测试和两架构构建，全部成功后创建 draft Release、上传两份 ZIP，再公开；带 SemVer 预发布后缀时标记 prerelease。脚本只接受上述 SemVer 形式，其他 v* tag 会失败。需仓库允许 GITHUB_TOKEN 写入 Release；无需服务器 SSH 或部署 secrets。已存在同名 Release 会失败而不覆盖；上传失败可能留下 draft，应由维护者检查后处理再重跑。此评估未触发 workflow。

## 服务器准备与环境认证

要求 Linux arm64、systemd、已安装匹配架构的 upy，现有 api 用户和 api 组。由具有 root 权限的部署管理员管理 unit 和部署目录；服务自身以 api:api 运行。部署根目录及父目录须允许 api 遍历，发布文件须可读、二进制可执行；目录不要放在不可遍历的 /root 下。

未来准备示例（本次未执行）：

```sh
sudo install -d -m 0755 /opt/demo-api /opt/demo-api/shared
sudo install -m 0644 server/deploy.yaml /opt/demo-api/deploy.yaml
# 仅首次创建；已有配置应编辑合并，不能覆盖。
sudo install -m 0600 server/service.env.example /opt/demo-api/shared/service.env
```

`shared/service.env` 是应用运行环境文件，使用 KEY=value、无 export 的 systemd 语法；root 所有、0600 即可由系统 systemd 读取。upy 自动生成 unit 并引用该文件，缺失时不会强制报错，故所需配置须提前准备。不要手写一份会被 upy 覆盖的 unit。

下载 GitHub Release 的认证独立使用部署进程环境变量 `DEPLOY_GITHUB_TOKEN`，不使用 GH_TOKEN / GITHUB_TOKEN 作为 upy 的隐式认证。私有仓库 token 应具备 owner/demo-api 的只读 Contents 权限，并满足组织授权要求。不要将真实 token 提交或打进 ZIP。

注意：upy 的 `github.tokens.owner` 和默认 `github.token` 用户配置优先于环境变量。确保实际执行 upy 的管理员账户配置中没有抢先匹配的旧 token（UPY_CONFIG / UPLY_CONFIG 也可能指定配置文件）。`sudo` 默认可能过滤环境，不能直接假设普通用户 export 会传入 root。

未来人工部署示例，在 root 管理会话内安全输入 token，避免写入历史：

```bash
read -rsp 'GitHub read token: ' DEPLOY_GITHUB_TOKEN; echo
export DEPLOY_GITHUB_TOKEN
upy release v1.2.3 --root /opt/demo-api
unset DEPLOY_GITHUB_TOKEN
```

显式版本适合首次部署；未来 `upy release latest --root /opt/demo-api` 获取最新正式版。首次成功 release 后会登记本机项目，之后才可使用 `upy deploy demo-api`。服务器精确匹配 `demo-api-linux-arm64.zip`；不要改成含糊的 bundle.zip 或期待 upy 自动挑选架构，清单 target 还会进行平台检查。

## 集成后核验与失败处理

由维护者未来检查两份 ZIP 清单、Linux ELF 架构和 bin/demo-api 的 executable bit；运行 Actions 测试并在真实 arm64 服务器验证以下命令：

```sh
systemctl status upy-demo-api.service
journalctl -u upy-demo-api.service -n 100 --no-pager
curl --fail http://127.0.0.1:8080/health
```

8080 端口应空闲且由应用主进程或其子进程监听；upy 对本地探针还会核对端口归属。upy 负责生成 unit、停止旧服务、启动新服务和健康检查，激活失败时尝试恢复旧 unit/服务，首次失败则清理新 unit；恢复也可能失败，应检查日志，不能将自动回滚等同于已验证成功。

成功部署会清理旧版本，不承诺保留多个历史 releases（keep 当前仅兼容字段）。如需回退，使用明确的旧 tag 重新下载部署，并确认该 Release 仍保留。`shared` 存放持久配置，应用数据需要根据真实存储另行设计备份；未假设此 API 有数据库或添加空备份脚本。整个接入方案仍需实际应用和服务器验证。

配置依据：internal/config/project.go、manifest.go、user.go，internal/deployer/system.go、deployer.go，internal/commands/release.go、deploy.go，internal/deploy/deploy.go。未读取任何 upy-project-adapter skill。
