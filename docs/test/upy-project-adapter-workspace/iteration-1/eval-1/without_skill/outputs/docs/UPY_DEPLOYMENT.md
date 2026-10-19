# owner/demo-web 的 upy 接入说明

## 集成范围

已知事实：Vite Vue SPA、pnpm@9.15.0、已有 pnpm-lock.yaml、Node20、构建目录 dist。这里假定 package.json 中已有 `build` script，应用通过站点根路径 `/` 提供服务。若实际使用子路径，需要同步调整 Vite base、Vue Router history base 和 Nginx location 后再构建。

将 scripts/upy/build-bundle.sh 和 .github/workflows/release.yml 合并至目标仓库对应位置，不覆盖已有同名逻辑。服务器配置保留在文档目录供运维复制，不要把 server/deploy.yaml 放到 ZIP 中。建议给现有 .gitignore 增加 `.upy-artifacts/`，无需替换已有内容。package.json 如需声明包管理器，可合并 `"packageManager": "pnpm@9.15.0"`，无需创建新 lockfile。

## 构建与 Release

本地先准备 Node20、pnpm@9.15.0、Python3。在目标项目根目录运行：

```bash
bash scripts/upy/build-bundle.sh v1.2.3
```

脚本安装冻结依赖、删除旧 dist、运行 build、检查入口并生成 `.upy-artifacts/bundle.zip` 和 `SHA256SUMS`。这些命令是集成后的操作说明，本次未执行。构建前核对 dist 确为可删除的产物目录。

ZIP 必须是以下布局，没有外层项目目录：

```text
bundle.zip
├── deploy.yaml        # name=demo-web, version=v1.2.3, type=static
└── dist/
    ├── index.html
    └── ...构建资源
```

workflow 由 `v*` tag push 触发，在 CI 使用 Node20 和固定 pnpm 构建，再以 GITHUB_TOKEN 创建 Release 并上传固定名称 bundle.zip 与 SHA256SUMS。仓库需允许 workflow 的 contents:write；无需服务器 SSH 凭据。它只发布资产，不触发部署。tag 必须符合脚本接受的版本格式；带 `-` 的 tag 标记为 prerelease，不进入 latest 正式发布流程。正式版不应覆盖或移动 tag；重跑仅恢复尚未发布的 draft。

应用如需 VITE_* 构建变量，请在 build step 的 env 中按实际项目增加 GitHub Variables/Secrets 映射。VITE_* 会进入浏览器资源，不能存放服务端秘密；服务器运行时环境变量不会改变已构建 SPA。

## 服务器准备

示例采用 Linux + Nginx，项目根目录为 `/srv/demo-web`，域名为 `demo-web.example.com`。路径、域名、HTTPS 证书和访问控制均需根据真实环境确认。

由运维创建项目目录，赋予部署账号写权限，并将 server/deploy.yaml 复制为 `/srv/demo-web/deploy.yaml`。Nginx 账号必须可遍历项目及 releases 目录、读取静态文件。upy CLI 应预先按实际服务器架构安装。服务器不需要 Node、pnpm 或源码。

将 nginx.conf.example 合并至服务器站点配置，替换域名并配置生产 HTTPS。Nginx root 必须是 `/srv/demo-web/current`，因为 upy 会将此软链接直接指向 `releases/<version>/dist`。Vue Router history 的深层路由依靠 try_files 回退 index.html；缺失 assets 返回 404。启用前由运维运行 nginx 配置检查并按其管理方式 reload。

公开仓库通常可匿名读取；私有仓库需在部署账号环境中安全设置 DEPLOY_GITHUB_TOKEN，权限至少可读取 owner/demo-web 的 Contents/Release。upy 优先使用用户配置中的 owner token、默认 token，再回退此环境变量；排查时留意旧配置覆盖。不要把 token 写入 deploy.yaml、仓库或 bundle。

## 手动部署与验证

以下均为待运维执行的命令，本次没有运行：

```bash
upy release v1.2.3 --root /srv/demo-web
# 后续明确希望部署最新正式版时：
upy release latest --root /srv/demo-web
```

首次成功运行 release 后才会登记本机项目，此后可用 `upy deploy demo-web` 部署最新正式版。upy bundle 是部署本地 ZIP 的命令，不是构建打包命令。

首次部署前 current 尚不存在，站点暂不可用。部署后检查 current 指向目标版本 dist、首页返回成功、深层路由刷新正常、静态资源及 API 请求正常。按实际域名使用 curl 和浏览器验证。本示例未配置 manifest healthcheck，因为没有真实服务器 URL；因此 upy 的激活成功不能代替业务可用性验收。确认真实探活地址后可在构建脚本生成的 manifest 中加入顶层 healthcheck.url 和 timeout。

SHA256SUMS 用于人工验证下载产物：将两个资产放在同一目录后执行 `sha256sum -c SHA256SUMS`。目前读取的 upy 应用 Release 流程没有自动下载并核验该校验文件，不应宣称自动校验已启用。

## 回退与保留

需要恢复已发布版本时，显式指定旧 tag，例如 `upy release v1.2.2 --root /srv/demo-web`；若默认版本比较跳过降级，使用 `upy release v1.2.2 --force --root /srv/demo-web`，并重新验收。私有仓库 token 和旧 Release 资产必须仍可用。--force 也会重新下载缓存并强制部署，谨慎用于同版本重部署。

releases.keep 在当前契约中仅作兼容，清理逻辑只保留当前版本，不能依靠设置更大的 keep 获得多版本留存。失败时的自动回滚是 best-effort，上一版本目录不存在时无法恢复；服务器首次部署也没有上一版本。应保留 GitHub 历史 Release/资产以支持重新下载恢复。

## 验证边界

本次只有配置契约阅读与文件生成，没有目标应用源码、服务器或凭据；未执行依赖安装、真实构建、Actions、Release 发布、Nginx 检查或部署，也未生成真实 bundle。上述验收项均待集成后执行。
