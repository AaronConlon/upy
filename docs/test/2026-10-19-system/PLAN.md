# system 部署真实测试计划

日期：2026-10-19。测试仓库：`/Users/aaron/repos/upy-test`，GitHub 私有仓库 `AaronConlon/upy-test`。

## 范围和方法

- 被测对象是本项目当前工作区的 system 实现，含未提交修改；执行前复制并编译固定快照，记录文件 SHA-256，避免把后续修改混入结果。
- 本机 macOS；在独立 Debian Linux 容器内以真实 systemd 为 PID 1，部署真实 Go ELF Web 服务。不是 systemctl mock，也不等价于独立 VM 的开机/完整主机安全验收。
- 通过真实 upy CLI 运行本地 Bundle 部署，并从 GitHub 私有 Release 下载部署；GitHub Actions 构建 Linux amd64/arm64 Bundle、输出校验文件。
- 每例独立 root、服务名、端口和用户；断言退出码、HTTP 版本、MainPID、unit 内容、current、state、文件保留及运行 UID；危险和失败注入只针对测试资源。
- 顺序执行，PASS / FAIL / BLOCKED 分开。失败也保存 stdout/stderr、systemctl show、journal、state、unit 与现场信息。测试结束停止测试容器；不把依赖不足或未执行标成 PASS。

## 场景分支

`main` 保存服务、构建和运行器；`scenario/system-happy-path`、`scenario/system-rollback`、`scenario/system-arguments`、`scenario/system-isolation`、`scenario/system-lifecycle` 保存各组激活场景说明和默认参数。所有分支使用同一可重复运行的场景引擎。

## 逐项测试矩阵

| ID | 场景 | 预期 |
| --- | --- | --- |
| SYS-01 | 私有 Release 下载并首次部署 | 验证真实认证、下载、校验、active、版本、enabled、非 root UID |
| SYS-02 | 正常升级 | HTTP 和 unit/current/state 指向新版本，共享数据保留 |
| SYS-03 | 相同版本重复部署 | 跳过，不改变 MainPID |
| SYS-04 | 健康检查 503 | 非零退出，停止新进程并恢复旧版本 HTTP |
| SYS-05 | 启动即退出 | 非零退出，恢复旧服务 |
| SYS-06 | 首次部署失败 | 无成功 state、不留可运行的新 unit |
| SYS-07 | 缺少可执行文件 | 准备阶段失败，旧服务 PID 与 HTTP 保持 |
| SYS-08 | 错误 target 架构 | 准备阶段拒绝，旧服务不受影响 |
| SYS-09 | 原服务已停止时升级失败 | 还原 unit，仍保持停止 |
| SYS-10 | 缺少服务用户 | 必须拒绝，不能静默以 root 运行 |
| SYS-11 | 不同 root 使用同名 unit | 拒绝冲突且原服务仍运行 |
| SYS-12 | 既有非 upy unit | 拒绝接管，unit 与 PID 均保持 |
| SYS-13 | 参数空格、引号、$、%、空字符串、两端空白 | 原样传入 argv |
| SYS-14 | root 路径包含空格 | unit 可被 systemd 正确执行 |
| SYS-15 | 同版本 --force 发布失败 | 原产物可恢复，旧服务仍可用 |
| SYS-16 | 升级后保留上一成功版本 | 至少保留 current 与 previous |
| SYS-17 | clean --old-releases | 不删除承诺的回滚版本 |
| SYS-18 | clean --purge | 拒绝活跃服务或安全卸载，不能留下指向缺失产物的 unit |
| SYS-19 | 部署锁冲突 | 第二个部署立即拒绝，首个部署可继续 |
| SYS-20 | HTTP 卡住不响应 | 有界超时并回滚 |
| SYS-21 | 同 unit 外部服务占用探针端口 | 不能将其他进程响应误认作新版本健康 |
| SYS-22 | 新版本启动后强杀 upy | 记录未完成事务，下次可恢复/拒绝，clean 不得误删运行产物 |
| SYS-23 | 普通用户无 unit 写权限 | 停旧服务前拒绝，旧服务仍运行 |
| SYS-24 | service 类型别名 | 与 system 相同部署行为 |

## 交付

本目录保存计划、环境与源码记录、JSON 逐项结果、Markdown 测试报告及每例日志。测试仓库保存应用源码、分支、Actions、构建脚本和顺序执行器；凭据不写入文件或日志。
