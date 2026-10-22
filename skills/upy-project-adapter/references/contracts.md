# 配置契约

依据：AaronConlon/upy v0.8.0，internal/config/{manifest,project,user}.go、internal/crypto、internal/server 与 internal/deployer。升级后核对目标版本。

## 服务器 <root>/deploy.yaml
```yaml
name: example-app
release:
  provider: github
  repository: owner/repository
  asset: bundle-linux-arm64.zip
service: # 仅 system，预先创建用户/组，确认部署目录可读、数据目录可写
  user: example-app
  group: example-app
  unit: example-app.service
# 可选: bundle 压缩包本地缓存保留数量 (默认最多保留 3 个，范围 0~100)
bundles:
  keep: 3
# 可选: 纯 Go 原生 AES-256-GCM bundle 强解密 (若在公开 Release 发布了加密 bundle，配置解密密码)
crypto:
  password: "仅保存在服务器 deploy.yaml 中的解密密码"
```
static/docker 删除 service。asset 必须与 Release 中文件名完全一致；多架构不能统一用模糊名字。name 与 bundle 一致。`releases.keep` 当前仅兼容读取，清理实现不按它控制数量；自动清理保留 current 和 state 中的 previous。不要通过 keep: 3 承诺保留三版，模板默认省略该字段。备份字段是另行核对的能力，不作为默认接入配置。若配置了 `crypto.password`，upy 会自动在部署时对加密 bundle（UPYC 魔数格式）进行防篡改校验并解密；若上传了未加密 bundle 或密码错误，部署将安全拦截。

私有仓库 token 保存在用户级 `~/.upy/config.yaml` 的 github.token / github.tokens，或使用 `DEPLOY_GITHUB_TOKEN` 环境变量；不要假定普通 GITHUB_TOKEN 是 upy 的读取入口。只按目标仓库授予读取 Release 所需权限。CI 发布使用 github.token 是另一条认证链，不能拿它当服务器长期 token。不要打印、提交或打包 token。

## CI 入站 Webhook（upy v0.4.0+）

`upy server` 提供给 CI 触发部署的接口。它与项目配置中的 `notify.webhook`（部署结果的出站通知）是两条相反方向的链路，不要混用。

服务器用户配置 `~/.upy/config.yaml` 可设置监听地址与全局 API Token：

```yaml
server:
  listen: "127.0.0.1:8999"
  token: "仅保存在服务器上的全局 API Token"
```

项目服务器配置 `<root>/deploy.yaml` 可设置项目专属入站密钥；该文件不进入 Bundle：

```yaml
name: example-app
release:
  provider: github
  repository: owner/repository
  asset: bundle.zip
webhook:
  secret: "仅保存在服务器与 GitHub Actions Secret 中的共享密钥"
```

接口为 `POST /api/v1/webhook/{project}`，其中 `{project}` 必须与 `deploy.yaml` 的 `name` 及首次成功部署登记的本地项目名一致。推荐 CI 在 Release 创建成功后发送精确 tag，不要依赖 `latest`：

```http
POST /api/v1/webhook/example-app
Content-Type: application/json
X-Hub-Signature-256: sha256=<hex_hmac>

{"target":"v1.2.3","force":false}
```

HMAC 使用请求正文原文和 `webhook.secret` 计算 SHA-256；也可使用 `Authorization: Bearer <token>` 或 `X-Upy-Token: <token>`。鉴权优先使用项目 secret，缺失时回退到全局 `server.token`；两者都未配置时拒绝请求。请求也支持 `target`/`force` query 参数，缺少 target 时默认为 `latest`。

成功响应为 `202 Accepted`，表示异步部署任务已创建，不表示部署已完成。常见失败状态为 `401`（鉴权失败）、`404`（项目尚未登记或项目名错误）和 `409`（该项目已有运行中的部署任务）。首次启用 CI Webhook 前，必须在服务器上手动成功部署一次以登记项目，并通过反向代理或其他网络方式让 Actions 能访问 `server.listen`；不要把服务直接暴露公网而不配置密钥。

## Bundle ZIP 根目录 upy.yaml (兼容 deploy.yaml)
公共字段：name、version、type；仅选择 static/docker/system 中一个配置块。version 必须非空且安全作为目录名，建议严格 SemVer；发布新版本使用新 tag/version，不默认 --force 覆盖。

```yaml
name: example-app
version: 1.2.3
type: system
target:
  os: linux
  arch: arm64
system:
  command: ["./bin/server", "--port", "8080"]
healthcheck:
  url: http://127.0.0.1:8080/health
  timeout: 30s
```
可选 system.workingDir，默认 release 根目录。command 推荐数组，字符串会按空白拆分，不提供 shell 引号语义。数组也受目标版本解析限制：空参数/边界空白需实际回归。不要依赖 shell 展开、管道或环境变量展开。

healthcheck 放顶层，timeout 为正整数秒或 30s/1m 等时长；不要添加 interval、retries、expectedStatus 等不受支持的字段。URL 指向该实例本身，避免反向代理缓存或其他服务返回200而误判；还应检查进程、unit 和业务版本。
