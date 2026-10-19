# 配置契约

依据：AaronConlon/upy v0.2.0，internal/config/{manifest,project,user}.go 与 internal/deployer。升级后核对目标版本。

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
```
static/docker 删除 service。asset 必须与 Release 中文件名完全一致；多架构不能统一用模糊名字。name 与 bundle 一致。`releases.keep` 当前仅兼容读取，清理实现不按它控制数量；v0.2.0 自动清理保留 current 和 state 中的 previous。不要通过 keep: 3 承诺保留三版，模板默认省略该字段。备份字段是另行核对的能力，不作为默认接入配置。

私有仓库 token 保存在用户级 `~/.upy/config.yaml` 的 github.token / github.tokens，或使用 `DEPLOY_GITHUB_TOKEN` 环境变量；不要假定普通 GITHUB_TOKEN 是 upy 的读取入口。只按目标仓库授予读取 Release 所需权限。CI 发布使用 github.token 是另一条认证链，不能拿它当服务器长期 token。不要打印、提交或打包 token。

## Bundle ZIP 根目录 deploy.yaml
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
