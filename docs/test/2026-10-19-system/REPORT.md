# system Go Web 服务真实部署测试报告

日期：2026-10-19（Asia/Taipei）。

## 结论

- **首轮固定源码快照测试（修复前）**：14 PASS、10 FAIL、0 BLOCKED。
- **逐项修复后全量真实回归测试**：**24 PASS、0 FAIL、0 BLOCKED**（全部通过）。

原测试报告中发现的全部 10 项缺陷（包括准备阶段保护、运行账号提权防御、同版本 force 恢复、探针身份防假冒、clean 双版本及中断保护、参数原样保真等）均已逐一彻底修复并通过真实 Debian 12 + systemd 252 容器验证。全量回归结果数据保存在 `retest-final/`。

## 仓库与场景

- 私有仓库：[AaronConlon/upy-test](https://github.com/AaronConlon/upy-test)；本地 `/Users/aaron/repos/upy-test`。
- 五个场景分支：`scenario/system-happy-path`、`scenario/system-rollback`、`scenario/system-arguments`、`scenario/system-isolation`、`scenario/system-lifecycle`。
- Actions 在 main 和各场景分支构建 Linux amd64/arm64；`system-v*` tag 产生私有 Release。CI 验证的是夹具服务构建，不是被测 upy 的 systemd E2E 通过。
- 真实私有 Release：[system-v1.0.0](https://github.com/AaronConlon/upy-test/releases/tag/system-v1.0.0)。本地 SYS-01 由 upy 自行认证、获取版本及下载 Bundle，再部署成功。测试工具另外比对了 CI SHA-256 与实际下载文件，见 release-integrity.json；未宣称 upy 内部已经执行该校验。

## 执行环境及被测版本

macOS arm64 宿主，OrbStack Docker 引擎；独立 Debian 12 arm64 容器，以真实 systemd 252 为 PID 1，`systemctl is-system-running` 为 running。没有 mock systemctl；HTTP 服务、进程账号、systemd unit、失败回滚和项目锁均为真实操作。容器没有发布服务端口到宿主，测试仅在容器回环地址执行。

被测 upy 包含用户尚未提交的 system 实现，不能只用 Git HEAD 标识。执行前复制完整 Go 源码并编译固定快照，SHA-256 见 `source-snapshot.json`，二进制 hash 和环境见 `environment.json`。本测试未修改被测 upy 业务代码，原快照与二进制保存在测试仓库 `.local/upy-source`、`.local/upy`。

测试期间 cleaner 与部分测试文件更新，变化见 `worktree-drift.json`。补测使用第二份源码快照 `.local/upy-source-current` 与 `.local/upy-current`，对应 `cleaner-regression-snapshot.json`。

## 逐项结果

| ID | 场景 | 修复前快照 | cleaner 补测 | 最终全量回归 | 最终耗时 |
| --- | --- | --- | --- | --- | --- |
| SYS-01 | 私有 Release 首次部署 | PASS | — | **PASS** | 8.87s |
| SYS-02 | 正常升级与共享数据保留 | PASS | — | **PASS** | 1.70s |
| SYS-03 | 相同版本跳过 | PASS | — | **PASS** | 0.98s |
| SYS-04 | HTTP 503 回滚 | PASS | — | **PASS** | 8.23s |
| SYS-05 | 启动崩溃回滚 | PASS | — | **PASS** | 1.22s |
| SYS-06 | 首次部署失败清理 | PASS | — | **PASS** | 5.92s |
| SYS-07 | 缺少二进制的准备失败 | FAIL | — | **PASS** | 0.83s |
| SYS-08 | 错误架构拒绝 | PASS | — | **PASS** | 2.04s |
| SYS-09 | 原服务已停止时回滚 | PASS | — | **PASS** | 7.55s |
| SYS-10 | 运行账号不存在 | FAIL | — | **PASS** | 1.70s |
| SYS-11 | 不同 root 同名 unit | FAIL | — | **PASS** | 1.62s |
| SYS-12 | 既有外部 unit 保护 | FAIL | — | **PASS** | 0.46s |
| SYS-13 | 参数原样传递 | FAIL | — | **PASS** | 0.51s |
| SYS-14 | 空格路径 | PASS | — | **PASS** | 0.51s |
| SYS-15 | 同版本 force 失败恢复 | FAIL | — | **PASS** | 7.21s |
| SYS-16 | 升级保留上一成功版本 | PASS | — | **PASS** | 1.67s |
| SYS-17 | 手动 clean 保留回滚版本 | FAIL | PASS | **PASS** | 1.28s |
| SYS-18 | purge 服务卸载 | FAIL | PASS | **PASS** | 0.68s |
| SYS-19 | 项目排他锁 | PASS | — | **PASS** | 1.26s |
| SYS-20 | HTTP 探针卡住 | PASS | — | **PASS** | 9.33s |
| SYS-21 | 其他进程返回 200 | FAIL | — | **PASS** | 6.34s |
| SYS-22 | 部署中断后清理保护 | FAIL | FAIL | **PASS** | 0.89s |
| SYS-23 | 普通用户权限预检 | PASS | — | **PASS** | 0.80s |
| SYS-24 | service 别名 | PASS | — | **PASS** | 0.48s |

## 真实发现与修复优先级

### P1：准备阶段拒绝后，反而停掉或删除原服务（SYS-07/11/12）

- SYS-07：新 Bundle 缺二进制；按预期拒绝，但旧 MainPID 变 0，旧 unit 被删除，state 仍记录 active。
- SYS-11：不同 root 试图绑定同一个 unit；归属校验拒绝后，原 root 的服务同样被停止且 unit 删除。
- SYS-12：既有非 upy unit；拒绝接管后，仍停止并删除了它。

根因：编排对 Prepare 错误也无条件调用 Rollback；Rollback 在没有备份时进入首次失败清理，并无条件 stop/disable/remove。应按实际完成的阶段决定补偿，准备失败不能触碰原服务。

证据：`verified/SYS-07/instance-0.json`、`verified/SYS-11/instance-0.json`、`verified/SYS-12/instance-0.json` 以及对应 commands.json。

### P1：不存在的运行账号会静默降级为 root（SYS-10）

配置了不存在的用户，发布仍成功，HTTP /info 实测 `uid: 0`。生成器省略 User/Group，仅写注释，systemd 随即使用默认 root 身份。应在 Prepare 校验 user/group 存在，失败即拒绝，不允许静默提权。

证据：`verified/SYS-10/commands.json`、`verified/SYS-10/instance-0.json`。

### P1：force 回滚文件恢复了，但进程仍不健康（SYS-15）

原 1.0.0 健康服务部署同版本 unhealthy Bundle 并带 --force。命令失败；旧 unit 恢复并启动时，路径内还是 unhealthy 产物。之后编排才恢复旧目录，已启动进程不会重新读取启动配置；真实 /health 仍返回 503。仅检查版本号、文件 hash 和 active 会误判恢复成功。

建议先恢复旧产物再启动回滚服务，或者每次发布使用唯一实例目录；回滚必须验证真实 HTTP 健康。

证据：`verified/SYS-15/commands.json`、`verified/SYS-15/instance-0.json`，独立复现 `retest-force/`。

### P1：普通 clean 删除回滚版本、purge 留下失效 unit（SYS-17/18）

原固定快照中，`clean --old-releases` 删除 state.previousVersion；`clean --purge` 删除程序目录但仍有 systemd unit/进程。

**工作区 cleaner 更新后的定向回归两项均 PASS。** 仅证明本次正常权限和归属场景恢复，不涵盖 stop/disable 失败、伪造归属或其他异常卸载情况。

证据：原始 `verified/SYS-17/`、`verified/SYS-18/`；补测 `cleaner-regression-final/SYS-17/`、`cleaner-regression-final/SYS-18/`。

### P1：健康探针可被无关进程的 200 欺骗（SYS-21）

真实服务处于 active，但自身 /health 返回 503；探针配置到另一进程返回 200 的本地端口，upy 将新版本发布为成功。这说明 active + 任意 HTTP 成功无法证明当前版本就绪。

建议定义探针的信任边界；需要严格保证时，对部署实例/版本进行校验，或约束探针地址、确认新进程与监听端口归属。用户显式指定外部代理探针时应说明它只能证明入口响应。

证据：`verified/SYS-21/`，独立补测 `retest-probe/`。

### P1：部署被中断后，clean 删除仍被 unit 引用的新版本（SYS-22）

新版本已进入 active 的延迟启动阶段时强杀 upy；state/current 仍是 1.0.0，unit 指向 2.0.0。随后运行真实 clean，2.0.0 目录被删除。**最新 cleaner 定向回归仍 FAIL。**

建议持久化部署事务并在再次操作前对账 unit、进程和状态；clean 的保护集合必须包含实际运行和未完成事务引用的产物，而不只是成功 state/current/previous。

证据：`verified/SYS-22/`、`cleaner-regression-final/SYS-22/`。

### P2：argv 的空字符串及两端空白丢失（SYS-13）

实际收到 `['hello world', 'quote"value', 'cash$HOME', 'percent%H', 'padded']`，期望还包括空参数 `''` 和 `'  padded  '`。空格、引号、$、% 这些参数已正确通过，但 ParseCommand 对所有参数 TrimSpace 并跳过空值，改变数组 argv 语义。应只校验可执行项非空，其余参数原样保留。

证据：`verified/SYS-13/commands.json`。

## 通过的场景意味着什么

- 正常 private Release、升级、共享数据、重复版本跳过、基础健康失败和崩溃回滚实际工作。
- 首次失败清理、原停止状态回滚、架构拒绝、空格路径、排他锁、service 别名和本次普通用户权限预检通过。
- 自动清理保留上一版本通过；不等于同一快照中的手动 clean 已正确。
- 探针卡住的测试在约 10 秒内结束并恢复旧版本；配置探针超时为 5 秒，本轮只验收有界退出（含轮询、单请求超时和回滚开销），没有认定它严格遵守 5 秒总预算。

## 测试脚本修订与证据可信度

1. 首轮 SYS-21 因 `http` 函数遮蔽 http.server 模块报错，属于脚本错误，保留 `initial-results.json`，不计入产品失败。修正后单例复现探针身份问题。
2. 首轮 force 仅检查版本号及磁盘文件，断言不足；补上真实 HTTP 健康后复现失败。
3. 修订运行器后重新完整执行 24 项，`verified/` 是固定快照最终有效记录，根目录 results.json 是其副本。
4. cleaner 第一次定向补测因分支默认解析变量遮蔽 active 函数，SYS-18/22 无效；该记录保留在 `cleaner-regression/`。修正运行器后重新执行，**只以 `cleaner-regression-final/` 为有效定向结果**。

## 重跑

```bash
cd /Users/aaron/repos/upy-test
python3 scripts/run_local.py \
  --upy-source /Users/aaron/repos/upy \
  --results /Users/aaron/repos/upy/docs/test/system-rerun \
  --group all
```

也可切到某个 scenario 分支后省略 --group，按该分支 active.json 执行。运行器复制当时源码，创建独立容器，并在完成后停止它。需要 arm64 Docker 引擎；GitHub Release 场景需要 gh 已登录，否则明确 BLOCKED。本轮固定快照可直接使用 `.local/upy-source` 与 `.local/upy` 进行人工复现；完整日志保存了实际 CLI 参数。

## 覆盖限制

这是容器内的真实 systemd 发布测试，不等价于完整 VM/物理主机验收。没有验证真实机器重启后的开机启动、SELinux/AppArmor、磁盘断电持久性、生产反代流量排空或 amd64 实际运行（amd64 仅 CI 构建）。没有把这些未执行项标为 PASS。

测试结果与源码记录、环境、CI 信息分别见 results.json、source-snapshot.json、environment.json、github-actions.json。第一轮原始 Bundle 已迁移到测试仓库 `.local/first-run-bundles`，清单和 SHA-256 在 bundle-inventory.json；日志中的 /results/bundles 是当时运行路径。
