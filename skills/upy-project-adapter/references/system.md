# Linux system 部署

至少核对目标 upy 支持 type: system 与 target，服务器是 Linux 且 systemd 为 PID1、有 systemctl、部署者有管理 unit 的权限。当前模板契约核对至 v0.4.0；不把尚未发布修复视为已安装。

Go：从 go.mod 获取 Go 版本，找真正 main package；测试后构建 `GOOS=linux GOARCH=<arch> CGO_ENABLED=0 go build -trimpath -o <stage>/bin/server <main-package>`。CGO_ENABLED=0 仅适用于无需 CGO 的项目；否则采用匹配 libc/架构的构建环境或 Docker。打包前 chmod 0755，检查 ELF 架构。target.os=linux、target.arch=amd64/arm64；不同架构分别生成精确资产名。

不要自行安装一份与 upy 冲突的 systemd unit；upy 根据 manifest 管理 unit。service.user/group/unit 在服务器配置中。预先创建专用非 root 用户/组；检查身份真实存在，不能假设无效用户会安全失败。服务使用非特权端口，反向代理单独配置。

环境：<root>/shared/service.env（优先，缺失时部分版本回退 shared/.env）；不要随 bundle 发布真实环境配置。数据：<root>/shared/data，给服务账户写权限；程序从 env 读取绝对路径，不能写当前版本目录或假设工作目录不变。版本目录应可读且 binary 可执行。

服务应处理 SIGTERM，在 systemd 停止超时内优雅退出；健康接口只在初始化完成后成功。升级通常涉及停止旧进程再启动新进程，并不保证零停机。不要默认 --force。演练健康失败、启动失败、用户不存在、端口占用、错架构、缺 binary、同版本覆盖、连续升级/清理及其他 unit 隔离；查看 HTTP 业务版本与 journal，不能只验证200。

Java：command 数组如 ["/usr/bin/java", "-jar", "./app.jar"]；确认工作目录与服务器 JRE 版本，JAR 一同交付。Python：不打包开发机 venv；明确服务器可用解释器、锁定依赖及安装阶段。upy 并未提供通用 pre/post install hook，不要编造 hooks 字段；无法可靠离线交付依赖时用 Docker。

历史真实测试曾暴露 Prepare 失败影响旧服务、错误身份、同版本恢复顺序、clean 与中断部署等问题。这些是目标版本的回归测试项，不是当前所有版本必然存在的结论。应用接入不能替代部署器修复。
