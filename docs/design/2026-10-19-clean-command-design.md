# upy clean 命令架构与设计方案

- **文档状态**：已实现并随 `v0.1.12` 发布
- **设计日期**：2026-10-19
- **作者**：Aaron Conlon / Antigravity
- **涉及模块**：`internal/cleaner/`、`internal/projectlock/`、`internal/commands/clean.go`、`internal/config/user.go`

---

## 一、背景与设计目标

`upy` 作为一个专为轻量化、现代化单机部署设计的工具，在服务器端纳管前端静态站点（SPA）、Docker Compose 容器应用等多类型项目。随着版本持续迭代与发布，项目根目录下会逐渐积累：
1. **历史 Bundle 压缩包缓存**：每次自 GitHub Release 拉取或本地传入的 zip 压缩包，占用数百 MB 至数 GB 磁盘。
2. **历史非活跃部署版本**：多次迭代后遗留的历史旧版本目录（如 `releases/v1.0.0`、`releases/v1.0.1`）。
3. **部署中断残留临时目录**：网络中断、进程中断或解压异常遗留的 `.tmp-deploy-*` 临时目录。

如果不定期清理，容易耗尽服务器磁盘空间，甚至导致部署因 `ENOSPC` 失败。

### 核心设计目标
1. **可视化分析**：扫描项目磁盘占用，以人类可读格式（MB / GB）展示各分类资源的体积。
2. **极简交互与灵活参数**：默认提供交互式菜单（单项目、跨项目、推荐瘦身、定向清理），同时支持纯命令行静默批处理。
3. **安全第一（双重活跃版本保护）**：绝不误删当前正在对外提供服务的活跃版本。
4. **跨进程互斥保护**：部署与清理之间互斥执行，避免清理动作误删正在解压或部署中的新版本。
5. **严谨的错误传播与空间核算**：删除失败时准确报告错误并阻断后续风险操作，释放空间仅累计真实成功的项目。

---

## 二、项目目录结构与清理维度拆解

一个典型的 `upy` 纳管项目目录结构如下：

```text
/path/to/project/
├── deploy.yaml                      <-- 项目配置文件 (受保护)
├── current -> releases/v2/dist      <-- [关键软链] 当前提供服务的入口 (受保护)
├── state.json                       <-- 部署状态记录 (受保护)
├── .deploy.lock                     <-- 跨进程互斥排他锁 (运行时自动管理)
├── bundles/                         <-- [清理项 A] 历史下载包缓存 (*.zip)
│   ├── app-v1.0.0.zip
│   └── app-v2.0.0.zip
└── releases/
    ├── v1.0.0/                      <-- [清理项 B] 历史旧版本目录
    ├── v2.0.0/                      <-- [受保护] 当前活跃版本 (禁止删除)
    └── .tmp-deploy-234021/          <-- [清理项 C] 异常中断残留的临时解压目录
```

### 清理维度矩阵

| 清理维度 | 对应文件/目录 | 风险等级 | 说明与行为 |
| :--- | :--- | :--- | :--- |
| **A. Bundle 缓存** | `<root>/bundles/*.zip` | 🟢 极低 | 清理历史下载包；后续重新部署对应版本时会自动重新拉取。 |
| **B. 残留临时目录** | `<root>/releases/.tmp-deploy-*` | 🟢 极低 | 清理历史非正常中断遗留的孤儿解压目录。 |
| **C. 历史旧版本** | `<root>/releases/<非活跃版本>` | 🟡 低 | 清理非当前运行的历史版本；清理后不可直接进行秒级软链回滚。 |
| **D. 本地项目注销** | `~/.upy/config.yaml` 的 `projects` | 🟡 低 | 仅从全局快速操作列表注销该项目，**不删除**服务器上的任何物理文件。 |
| **E. 彻底清空产物** | releases + bundles + current + state | 🔴 高 | 彻底还原为空白项目目录，需交互式二次强确认，完成清空后自动注销。 |

---

## 三、系统架构与模块设计

系统整体分为四个协作层次：

```
┌─────────────────────────────────────────────────────────┐
│                     CLI 用户交互层                       │
│    (main.go / internal/commands/clean.go / ui.Select)   │
└────────────────────────────┬────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────┐
│                     清理器核心引擎                       │
│               (internal/cleaner/cleaner.go)             │
│    AnalyzeProject / CleanBundles / CleanOldReleases     │
│       CleanTmpDeployDirs / PurgeProject / FormatBytes   │
└──────────────┬───────────────────────────┬──────────────┘
               │                           │
               ▼                           ▼
┌───────────────────────────────┐ ┌───────────────────────────────┐
│        跨进程项目排他锁        │ │       用户配置与项目注册      │
│ (internal/projectlock/lock.go)│ │   (internal/config/user.go)   │
│   syscall.Flock / LockFileEx  │ │  UnregisterLocalProject / ... │
└───────────────────────────────┘ └───────────────────────────────┘
```

### 1. 核心清理器引擎 (`internal/cleaner/cleaner.go`)

- **数据分析模型 (`ProjectStats`)**：
  ```go
  type ProjectStats struct {
      CurrentVersion       string   // 状态记录或解析出的首选当前版本
      SymlinkVersion       string   // current 符号链接指向的版本
      ProtectedVersions    []string // 受保护不被删除的全部活跃版本列表
      VersionConflict      bool     // state 与 current 软链版本是否存在分歧
      ActiveVersionUnknown bool     // 是否存在版本目录但无法判定任何活跃版本
      UnidentifiedReleases []string // 活跃版本未知时保护的版本列表

      BundleFiles     int      // bundle 缓存文件数
      BundleBytes     int64    // bundle 缓存大小 (字节)
      OldReleases     []string // 可清理的非活跃旧版本列表
      OldReleaseBytes int64    // 旧版本占用大小 (字节)
      TmpDirs         []string // 残留临时目录列表
      TmpBytes        int64    // 残留临时目录大小 (字节)
  }
  ```

- **双重版本保护与解析算法**：
  - 传统方案仅依赖 `state.json` 或单纯字符串切割，在状态落盘失败或部署切换过程中极易误删。
  - `cleaner` 分别独立读取 `state.json` 的 `CurrentVersion` 与通过 `resolveCurrentVersionFromSymlink` 规范化计算 `current` 软链目标。
  - 两者指向的版本**全部加入 `ProtectedVersions`**，即使发生状态滞后，线上实际提供服务的目录绝不会被扫描入 `OldReleases`。
  - 若 `releases/` 下存在版本目录，但两个数据源均无法确定任何活跃版本（如状态损坏且软链丢失），则触发 `ActiveVersionUnknown = true`，**禁止将任何版本视为旧版本**，并在执行旧版本清理时直接返回 `ErrUnknownActiveVersion`。

- **精确空间核算与错误聚合**：
  - 仅在物理删除成功（`os.Remove` 或 `os.RemoveAll` 返回 nil）时才累加释放字节数 `freedBytes` 与计数。
  - 遇到权限不足或只读文件系统等异常时，收集所有底层错误并通过 `errors.Join` 返回，杜绝“删除失败仍报告成功”。

### 2. 跨进程项目级排他互斥锁 (`internal/projectlock`)

为了防止在部署（解压、编译、容器构建）期间并发执行 `clean` 导致正在使用的 `.tmp-deploy-*` 或新版本被误删，引入统一项目锁：
- **锁文件**：`<projectRoot>/.deploy.lock`。
- **底层机制**：
  - Unix / Linux / macOS：基于 `syscall.Flock(fd, LOCK_EX|LOCK_NB)` 非阻塞排他锁。
  - Windows：基于 `LockFileEx` 非阻塞排他锁。
- **进程内可重入（Reentrant）**：
  - 采用全局互斥锁配合持有表 `heldLocks map[string]*lockEntry` 进行引用计数。
  - 部署流程或清理流程内部的嵌套调用共享同一底层文件锁，不会产生自我死锁。
- **冲突诊断**：
  - 持锁方在锁文件中写入操作类型与 PID（如 `action: 部署中\npid: 12345`）。
  - 冲突方获取失败时直接向用户报告友好错误：`项目正在进行 部署中（PID: 12345），无法获取项目锁，请稍后再试`。
- **高可用与崩溃安全**：
  - 文件锁绑定在操作系统内核的文件描述符生命周期上，进程异常终止或崩溃时内核会自动关闭描述符并释放锁，永不产生僵尸锁死问题。

### 3. 命令分派与批量处理 (`internal/commands/clean.go`)

- **单项目交互菜单**：
  - ⚡ 推荐瘦身 (Bundle 缓存 + 旧版本 + 残留临时目录)
  - 📦 仅清理 Bundle 下载缓存
  - 📂 仅清理历史旧版本与残留临时目录
  - 📑 从本地项目列表中注销此项目 (保留物理文件)
  - ⚠️ 彻底清空此项目所有部署产物 (高风险，二次确认)
- **批量项目处理 (`cleanAllProjects`)**：
  - 严格遵循命令行显式参数：如 `upy clean --bundles` 选择批量项目时，仅对所有项目清理 Bundle 缓存，绝不越权删除 release。
  - 批量操作中聚合所有项目的执行错误，如果存在部分项目失败，返回非零退出码。

---

## 四、CLI 命令使用规范

### 1. 交互模式
```bash
# 自动检测当前目录项目，若无则展示已登记项目列表供选择
upy clean

# 指定针对特定项目进行交互式清理
upy clean my-app
```

### 2. 命令行快速选项 (支持自动化脚本)
```bash
# 仅清理下载的 bundle.zip 缓存
upy clean my-app --bundles

# 仅清理历史非活跃版本与临时目录
upy clean my-app --old-releases

# 推荐常规瘦身 (缓存 + 旧版本 + 临时目录) 并跳过确认提示
upy clean my-app --all -y

# 仅注销项目登记 (不删文件)
upy clean my-app --unregister -y

# 彻底清空项目部署产物 (高风险)
upy clean my-app --purge -y

# 指定项目根路径清理
upy clean --root /var/www/my-app --all -y
```

---

## 五、测试与验证规范

已落地完备的自动化单测，覆盖全链路关键安全边界：

1. **`TestClean001_ConflictProtection`**：验证 `state.json` 为 `v1`、`current` 软链指向 `v2` 时，两者均受保护不被删除。
2. **`TestClean002_UnknownActiveVersionProtection`**：验证活跃版本未知时，常规清理拒绝删除任何 release。
3. **`TestClean003_ConcurrentLocking` & `TestDeployBundleRefusesWhenLocked`**：验证部署与清理之间的跨进程锁互斥。
4. **`TestClean004_DeletionErrorPropagation`**：验证只读或受限环境下，删除失败准确返回非空错误且不虚报释放空间。
5. **`TestClean005_BatchCleanRespectsFlags`**：验证批量模式下传入 `--bundles` 绝不触碰 releases。
6. **`TestClean005_BatchUnregisterDoesNotDeleteFiles`**：验证批量注销仅操作配置文件，物理文件完好无损。
