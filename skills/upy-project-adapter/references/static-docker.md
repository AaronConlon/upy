# Static 与 Docker

## Static
manifest：
```yaml
name: example-web
version: 1.2.3
type: static
static:
  directory: dist
```
ZIP 内容必须是 `upy.yaml` + dist/index.html + dist 内其他资源，不能套一层项目目录。若真实输出为 out，修改 directory 并复制 out。根据 packageManager / lockfile 使用冻结依赖安装：pnpm frozen-lockfile、npm ci 或对应 Yarn 版本；Node 版本来自项目。已有 build script 优先，不能盲目 npm install。

upy 只切换 current 软链，不替你配置 HTTP 服务。Nginx root 指向 <root>/current；SPA 路由回退 index.html，静态资源路径/base 按域名部署位置设置。SSR 需要常驻 runtime，使用 Docker/system。

## Docker container
```yaml
name: example-api
version: 1.2.3
type: docker
docker:
  mode: container
  dockerfile: Dockerfile
  port: "8080:8080"
healthcheck:
  url: http://127.0.0.1:8080/health
  timeout: 30s
```
Bundle 包含 Dockerfile 与 build context 的明确文件白名单。不要让 COPY 引用 ZIP 外文件。目标主机 Docker 构建需要网络/依赖；含原生二进制须匹配目标架构。container 当前默认将 <root>/shared/data 挂载 /data，额外 volumes 依据目标版本核对。

## Docker Compose
manifest docker 块用 `mode: compose`、`composeFile: compose.yaml`，不要混入 container 专属参数。ZIP 包含 compose.yaml、Dockerfile、所需源码/产物以及非敏感配置。

服务持久化使用 external named volume 或明确的共享目录绝对路径；不得 `./data:/data` 将数据库放在 releases/<version> 下。示例：`${UPY_DATA_DIR:?set UPY_DATA_DIR}:/data`，在服务器 shared/.env 配置 UPY_DATA_DIR=/srv/example/shared/data，创建目录并匹配容器 UID。

upy 会传入 shared/.env 供 Compose 插值，**不等于自动注入所有容器环境变量**。Compose 中明确 `environment: DATABASE_URL: ${DATABASE_URL:?...}` 或使用合适 env_file。数据库启动顺序/ready 检查、外部网络、迁移失败与数据恢复单独设计。不要在更新时自动删除数据卷。当前支持 release 内非敏感 .env 的行为有版本差异，保守只使用服务器 shared/.env。
