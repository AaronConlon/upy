# system 类型部署设计审核

- 日期：2026-10-19
- 对象：`docs/design/2026-10-19-system-service-deploy-design.md`，当前工作区 Draft Proposal。
- 方法：逐条核对设计、现有部署/状态/项目锁/清理实现及 systemd 官方文档。
- 结论：方向可行，存在 7 项 P1 和 3 项 P2；建议先修订设计再实现。
- 范围限制：静态审核，没有在 Linux / systemd 环境执行部署实验，没有修改原方案及业务代码。以下是设计缺口或复用现有代码时的具体风险，不代表尚未实现的 system 部署器已有同样代码缺陷。

## SYS-001 [P1] 日常发布的权限模型无法完成 unit 更新

位置：方案 197–202 行；关联 240 行。

日常发布需要写入 `/etc/systemd/system/upy-<name>.service`，示例 sudoers 仅放行 systemctl。普通部署用户没有写该目录的能力，因此无法执行 Activate；若停止旧服务后才发现无权写入，会造成原本可避免的停机。方案也没有提供本地运行用户、组、unit 名称等配置的数据结构。

不应通过让运维用户任意写系统 unit 目录补齐权限：unit 可描述高权限执行行为。示例使用全体 `upy-*` 和 `(ALL)`，与“只操作指定项目”的描述不一致。

建议：明确选择 root 发布，或提供 root 持有、受限的服务管理 helper。helper 只能处理登记的 unit、固定 root 与服务用户，负责校验、原子写入、daemon-reload 和受限服务操作；启动前完成权限预检，非交互调用使用 `sudo -n`。本地服务器配置需定义用户、组及文件权限；服务账号不应能改写部署程序或被信任的配置。

验收：仅获指定项目授权的普通用户能完整发布，但不能更新其他项目或任意 unit；缺权在 stop 前失败。

## SYS-002 [P1] 回滚未停止失败的新服务，可能根本没切回旧版本

位置：方案 169–172、239–242 行。

新服务可能已启动并占有端口，只是健康检查失败。恢复旧 unit、daemon-reload 再 start 不会替换仍在运行的新进程。新服务也可能处于自动重启中，回滚必须先取消这一活动。当前写法还会在旧服务部署前原本停止时将它启动；首次部署没有旧 unit 的分支也未定义。

建议：记录是否尝试启动、旧服务原始状态和旧 unit 是否存在；回滚先显式 stop 新服务并确认进程组退出，再恢复 unit 与必要的 enable 状态。只在旧服务原本运行且已被本次停止时启动旧服务；首次部署失败则清理本次创建的 unit，保持停止。处理启动频率限制，必要时 reset-failed 后再恢复；恢复也要做有界健康检查。

验收：新进程运行但 HTTP 返回错误、自动重启循环、首次失败、旧服务预先停止、旧版本恢复失败均有确定结果。

## SYS-003 [P1] 缺少崩溃恢复事务，运行版本与状态可能永久分离

位置：方案 155–173、181 行。

unit 写入、daemon-reload、服务启动、软链和 state.json 是多次独立操作。新版本启动后，upy 被终止，或 current/state 写入失败时，unit 可能指向新版本，而 state/current 仍指向旧版本。锁会随进程退出释放，不能修复这些状态。当前 `state.State` 也没有方案要求的阶段、unit 登记及 DEGRADED / FAILED 字段。

当前部署编排在 HealthCheck 成功后的软链或状态写入失败时直接返回，并不回滚。system 类型不应直接沿用这一成功提交边界。

建议：在停止服务前持久化事务记录及旧 unit 快照，定义 preparing / stopped / started / verified / committed / rollback-failed 等阶段及恢复规则。重启后的 deploy 和 clean 必须先对账实际 unit、运行进程和事务，再决定恢复或拒绝操作。明确 current/state 提交失败时的补偿；unit 更新采用同目录临时文件、原子 rename，快照也需持久化，而非只放内存。

验收：对停止旧服务、替换 unit、启动新服务、更新软链和写 state 的每个边界注入中断，再次运行能判定并恢复实际状态。

## SYS-004 [P1] 与自动清理、手动 clean 和 purge 的契约缺失

位置：方案 167 行、268–269 行；方案涉及模块列表也未列 cleaner。

现有 `internal/deploy/deploy.go:205` 的自动清理只保留当前版本；`internal/cleaner/cleaner.go` 的旧版本识别只保护 state/current，不保护 PreviousVersion、unit 指向或未完成事务版本。因此方案中的“保留当前和最近一个旧版本”无法仅靠新增 SystemDeployer 实现。

`clean --purge` 还会删除 releases 和 state，但不会停止、禁用或注销 systemd unit：运行进程可能暂时仍在，重启后却找不到二进制，而且失去纳管记录。

建议：统一 deploy 自动清理与手动 clean 的保留集合，包含已提交版本、承诺保留的回滚版本、unit/事务实际引用的版本。Purge 明确采用“服务仍登记或运行时拒绝”，或专门的卸载流程：确认、stop、disable、验证归属、删除 unit、daemon-reload，再删除产物；保留 shared 数据的约定要继续明确。

验收：发布后和 clean 后都保留承诺的回滚版本；未提交的新运行版本不被删；purge 不留下启用但丢失产物的服务。

## SYS-005 [P1] 同版本 force 重发破坏不可变目录和回滚快照

位置：方案 61、73 行及 194 行的日常发布入口。

现有 `internal/deploy/deploy.go:121–130` 在目标版本目录已存在时，先 RemoveAll，再安装新内容，这发生在 Deactivate 前。对 `upy release <当前版本> --force`，旧服务仍运行时就替换其工作目录；旧 unit 和新 unit 又指向同一路径，恢复旧 unit 也无法找回已删除的二进制。

建议：第一版禁止覆盖被服务、状态或回滚策略引用的目录；或者区分业务版本和部署实例 ID，以唯一目录安装同版本重发，快照记录具体实例。不能将已运行/已提交目录当作可直接覆盖的下载缓存。

验收：当前版本和保留回滚版本的 force 重发不会就地删除原产物，失败后能恢复原二进制。

## SYS-006 [P1] 全局 unit 身份与项目级锁不匹配

位置：方案 96、204–206 行。

unit 按 Bundle name 命名，但锁按项目 root 划分。两个不同目录部署同名项目，会持有不同锁，同时写同一个全局 unit；一方的服务控制和清理会影响另一方。以 upy 前缀和可写 state/deploy.yaml 为依据，也不足以证明既有 unit 的归属。

建议：初始化时建立持久化绑定（规范化项目 root、unit 名、运行用户、项目 ID），由可信位置保存；拒绝同名 unit 绑定不同 root，或采用具备唯一 ID 的 unit 名。更新前核对 unit 的实际来源、执行路径及覆盖配置；同 unit 的操作使用同一把锁。首次遇到既有 unit 必须拒绝隐式接管。

验收：不同 root 同名部署明确失败或隔离；修改 Bundle name 不能控制其他服务。

## SYS-007 [P1] 健康检查缺少请求期限和目标身份校验

位置：方案 46–50、160–163、241 行。

现有 `PollHealth` 使用无客户端超时的 `http.Get`；循环总期限不能中断一次卡住的请求。若 system 实现直接复用它，旧服务已经停止后，健康检查可能无限等待，回滚无法触发。

只检查固定端口 HTTP 响应，也不能证明响应来自本次启动的版本；占用相同端口的其他进程或重定向目标可能返回成功。方案示例里的 expectedStatus、interval 当前共享健康检查结构均不支持，默认 200..399 也和现有 2xx 语义不一致。

建议：每次请求绑定总 deadline，设置连接/响应超时，定义重定向策略和响应体读取上限；确认目标 unit 为 active、MainPID 存在，再结合受控本地探针及可选版本/部署实例标识确认新服务就绪。统一 status、interval、timeout 的配置位置与默认值。

验收：连接成功但不返回响应、重定向到其他服务、错误进程返回 200、新进程立即退出时均不误判或永久等待。

## SYS-008 [P2] command 的解析与 systemd 编码规则未定义

位置：方案 44–45、219–221 行。

字符串和数组同时支持，但没有说明字符串是 shell 命令还是参数列表，也没有说明数组如何转换为 ExecStart。systemd 有自己的引号、环境变量和 % 展开规则，简单拼接会改变带空格、美元符号等参数的含义。换行等控制字符也不能直接写入 unit 字段。

建议：第一版仅接受非空字符串数组，不执行隐式 shell。将 executable 和 workingDir 规范化，明确是否只允许 release 内路径，检查符号链接和目录边界；使用明确的 systemd 参数编码，拒绝控制字符，正确处理 %、$、反斜线和引号；安装前执行 unit 校验。可执行路径来自规范化后的绝对路径，不允许 Bundle 通过前缀改变执行权限。

验收：空格路径、字面量 $/%、引号参数、控制字符及越界路径都有明确解析或拒绝结果。

## SYS-009 [P2] 初始化与配置模型尚未闭合

位置：方案 39、189–195、216–258 行。

`service init` 宣称注册并 enable 服务，但首次还没有具体 release；没有定义初始 unit 的 ExecStart 指向和何时真正 enable。支持 service 别名仅出现在工厂示例，manifest 校验/归一化没有同步设计。healthcheck 同时有顶层与 system 内部两种入口，优先级未定义。服务器用户/组等配置虽被强调不来自 Bundle，却没有相应配置结构。

建议：明确完整服务器配置示例、初始化幂等规则与 init/install/enable 的时序；优先在首次成功发布后 enable，失败恢复原有 enable 状态。service 别名在解析层统一归一为 system，所有类型判断复用规范值。第一版最好沿用顶层 healthcheck，避免两份独立语义。

验收：只有初始化但未部署时不会留下开机启动失败的服务；别名、双重探针、重复初始化和已有账号都有确定行为。

## SYS-010 [P2] 停机窗口及反向代理兜底的承诺不成立

位置：方案 209–210 行。

“通常 0.2s~1s”没有依据，方案同时允许 30 秒优雅停机及 30 秒健康检查。单一上游停止时，proxy_next_upstream 也不能保证吸收中断；失败回滚还会进一步延长不可用时间。

建议：明确第一版为有界停机发布，停机时间由退出、启动、就绪及回滚决定，不作亚秒承诺。需要不中断流量时独立设计双实例、不同端口、探活后切流和旧连接排空；不将简单重启或反代重试描述为零停机。

## 建议的准入条件

1. 补齐服务器配置、权限与 unit 身份绑定。
2. 给出首次发布、更新成功、各阶段失败及进程中断后的恢复状态表。
3. 明确新进程停止、旧 unit 恢复与原停止状态保护。
4. 联动修改自动清理、手动 clean、purge 和 force 重发策略。
5. 定义严格的 manifest 与健康检查契约，并为上述失败场景列出 Linux 集成验收项。

## 官方依据

- systemd 服务及 ExecStart 的参数/展开规则：https://www.freedesktop.org/software/systemd/man/latest/systemd.service.html
- systemd 执行环境与 EnvironmentFile：https://www.freedesktop.org/software/systemd/man/latest/systemd.exec.html
- systemctl start/stop/reload 行为：https://www.freedesktop.org/software/systemd/man/latest/systemctl.html
- Nginx proxy_next_upstream：https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_next_upstream
