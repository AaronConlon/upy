# upy Agent Skills

`upy-project-adapter` 帮助 Agent 为应用仓库创建 upy 部署配置、bundle 与 GitHub Actions Release 流程，覆盖 static SPA、Docker/Compose、Linux system 服务。

## 安装

将 `upy-project-adapter` 整个目录复制到目标应用的 `.agents/skills/upy-project-adapter`，或复制到个人 `~/.codex/skills/upy-project-adapter`。不同 Agent 按其支持的 Skill 目录安装。只复制单个 SKILL.md 会丢失契约、样例和脚本。

本仓库目录是维护源，个人安装是副本；更新时重新复制整个目录。不要把评估 workspace 安装到 Skill 目录。

## 使用

```text
$upy-project-adapter 将当前项目接入 upy，生成配置、构建 bundle 的脚本和 GitHub Actions Release；目标 Linux arm64，部署 root /srv/demo。
```

Agent 会先读取项目事实与目标版本，再选择部署类型。仅生成接入文件不会自动发布或部署。

## 维护与验证

- `SKILL.md`：入口与接入步骤。
- `references/`：当前 upy 契约和分类细节。
- `assets/`：结构样例，使用前替换项目参数。
- `scripts/pack_bundle.py`：仅依赖 Python3 标准库，读取 Bundle 根目录 JSON 格式的 `upy.yaml`，生成 ZIP 和 SHA256。服务器侧的 `deploy.yaml` 不属于 Bundle。校验核心布局与路径边界，不能替代完整 manifest parser、ELF 检查或部署测试；敏感文件名检查也不能代替人工白名单审查。
- `evals/evals.json`：Agent 场景评估定义。

运行脚本测试：

```sh
python3 -m unittest discover -s skills/upy-project-adapter/scripts -p 'test_*.py' -v
```

真实 system 测试经验见 [docs/test](../docs/test/2026-10-19-system/REPORT.md)，Skill 对比评估记录单独保存在 docs/test/upy-project-adapter-workspace。
