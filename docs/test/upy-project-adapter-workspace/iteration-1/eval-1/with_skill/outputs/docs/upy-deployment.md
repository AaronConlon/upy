# owner/demo-web 接入 upy

## 范围与版本

已知项目为 Vite Vue SPA、Node 20、pnpm@9.15.0、pnpm-lock.yaml，输出 dist，选择 static，无需 Node 常驻服务或架构矩阵。假定 package.json 的 build 命令执行项目正式构建；集成时确认。未获得应用源码、测试脚本或服务器，未进行真实构建、CI、HTTP 或部署验证。

配置依据技能中的 upy v0.2.0 契约。只读执行仓库现有二进制 --help，显示 v0.1.11，确认 release、bundle 和 --root 命令存在；这不证明 v0.2.0 已发布或安装。服务器使用前需确认实际 upy 版本对 static 配置的支持，未核对的能力不要直接用于生产。

## 集成与打包

将 deploy/、scripts/、.github/、docs/ 下文件按相对路径复制到 owner/demo-web，合并已有文件，保留已有 workflows。无需用此样例覆盖 package.json 或 lockfile。要求 Node 20、pnpm 9.15.0、Python 3、Bash；在应用根目录执行：

```sh
bash scripts/upy/build-bundle.sh v1.2.3 artifacts/bundle.zip
unzip -l artifacts/bundle.zip
(cd artifacts && sha256sum -c bundle.zip.sha256)
```

脚本会冻结安装依赖，清空 dist 再执行 pnpm run build，只复制 dist 到临时 staging。确认 dist 全部内容适合公开，特别检查 source map、配置与敏感数据。ZIP 根层结构为 deploy.yaml、dist/index.html、dist 内资源；没有项目外层目录。deploy.yaml 为 JSON 格式的 bundle manifest，版本为 1.2.3，不含 v。服务器 YAML 不进入 ZIP。

推送 v* tag 会触发 workflow（本次未推送）；只支持 vMAJOR.MINOR.PATCH，其他 tag 会在构建前被拒绝。精确 Release 资产为 bundle.zip、bundle.zip.sha256。只有 release job 有 contents: write，重跑不覆盖已有 Release；修复发布内容使用新版本 tag。项目测试命令未知，workflow 明确留有集成注释，没有声称运行测试。Actions 中的 github.token 仅用于 CI 发布。

## 服务器准备

部署 root 暂定 /srv/demo-web，域名占位 demo-web.example.com，HTTP 示例端口 80；部署前确认目录、域名、TLS、操作系统与 upy 安装版本。static 资产不依赖 CPU 架构，但 upy 二进制必须匹配服务器 OS/架构。服务器需 upy、Nginx、下载 Release 所需网络与 CA；部署用户需能写 root，Nginx 用户需能遍历目录并读取文件。运行构建所需 Node/pnpm 不要求安装在服务器。

由服务器管理员创建目录并设置权限，将 deploy/upy/server.deploy.yaml.example 复制为 /srv/demo-web/deploy.yaml。该文件定义仓库、资产、保留数量；bundle 中 deploy.yaml 定义本次产物类型、版本和目录，两者不能互换。保留数量 3 不等于可靠回滚保证。

参考 nginx.conf.example 配置站点，root 必须指向 /srv/demo-web/current（upy 已将 current 指向该版本 dist），不要再追加 dist。先配置正确域名/TLS，在服务器校验 Nginx 配置后启用。示例假定部署在域名根路径；子路径需同步修改 Vite base 和 Nginx 路径。SPA 深层路由回退 index.html，缺失 /assets/ 文件应返回 404。

私有仓库为部署用户配置 ~/.upy/config.yaml 的 github.token 或 github.tokens.owner，也可通过 DEPLOY_GITHUB_TOKEN 注入最小读取权限凭据；不要把普通 GITHUB_TOKEN 当成 upy 自动读取入口。配置文件限制为仅该用户可读，不提交 token，不把 CI 临时 token 当长期服务器凭据。公开仓库按实际 API 限额决定是否配置认证。

SPA 无服务端进程，本方案不需要 shared/.env；VITE_* 在构建时写入公开 JS，不能存放秘密。服务器修改 shared/.env 不会改变已构建前端。若以后增加运行时公开配置需另行实现加载方案。此方案无数据库或可写应用数据，未来的持久化文件应放 /srv/demo-web/shared 的独立目录，不能放 releases 下；发布前单独备份。

## 首次部署与升级（仅供未来授权操作）

以下命令会真正部署，本次均未执行。先在隔离服务器确认 upy --help 及目标版本支持，再准备上述配置、权限与 Nginx。指定已存在的应用 Release：

```sh
upy release v1.2.3 --root /srv/demo-web
```

离线交付先在同一目录校验 SHA256，再部署本地 ZIP：

```sh
sha256sum -c bundle.zip.sha256
upy bundle ./bundle.zip --root /srv/demo-web
```

SHA256 是人工/脚本校验，不代表 upy 自动校验，也不能代替可信来源验证。部署后查看 current 实际指向，访问首页、静态资源、刷新深层路由，检查页面版本与浏览器报错。upy 不负责提供 HTTP 服务；此 static manifest 不含伪造健康检查。

升级使用新 tag（例如 v1.2.4）重复指定版本部署，检查新资源与缓存行为。保存旧 Release 资产及校验文件，不能只依赖服务器保留目录。

## 失败演练与恢复

在授权隔离环境先记录 current 和当前页面版本，用缺少 dist/index.html 的损坏包验证拒绝行为，再检查 current 与原页面是否保持可用；不要在生产直接试验。另行演练下载失败、权限不足以及前端资源 404。打包器会拒绝缺入口产物，但只有服务器演练能确认部署失败后的实际状态。

需要恢复时，先备份当前部署目录并确认旧资产完整，在隔离环境验证重新部署上一 tag 的行为；目标版本可能对已存在目录或同版本跳过有差异，不承诺通用自动回滚，不默认使用 --force，不运行 clean。这里没有 upy rollback/status/validate 命令。恢复后的首页、深链、资源与业务版本仍需复查，不承诺零停机。
