# 应用 GitHub Actions / Releases

构建 bundle 的工作在应用仓库，upy 自身 CLI 的二进制发布 workflow 不能直接复制成应用 workflow。

生成完整项目专用 workflow，优先沿用已验证的 Actions 版本。最小结构：tag v* 触发；全局 contents: read；checkout 对应 tag；安装项目实际工具链并冻结依赖；项目测试；构建白名单 staging；生成 JSON 格式 deploy.yaml；打 ZIP 与 SHA256；将文件作为 Actions artifact 上传；统一 release job 下载全部 artifact 后发布 GitHub Release，只有该 job contents: write。

并发控制按 workflow+tag 分组、cancel-in-progress: false，避免同 tag 发布中断；同名 Release 重跑是否覆盖资产要明确选择，默认不可变发布。使用 gh release create 并 --verify-tag；已有同 tag Release 不要默默 --clobber。

多架构 Go 使用 matrix arch: [amd64, arm64]、fail-fast: false；每个 job 输出 bundle-linux-<arch>.zip 和各自同名 .sha256，Actions artifact 名也唯一。最终 release job needs build，只在全部构建成功后发布全部资产。服务器配置 asset 精确匹配本机。静态一般一个 bundle.zip 即可，不强行架构矩阵。

构建脚本参数传版本/架构/输出路径。tag 从 GitHub env 传入 shell 变量，验证格式后使用，不直接把外部字符串插入 run shell。建议只支持 tag push；如果需要 workflow_dispatch，显式输入 tag，先验证远端 tag 存在并 checkout 该 tag，不能在默认分支构建后冒充旧版本。version 去除 v 前缀必须与 manifest 和说明一致，拒绝含 /、.. 等路径字符。

构建 ZIP 示例：
```sh
set -euo pipefail
VERSION="${RELEASE_TAG#v}"
# 项目构建后，将 JSON manifest 写入 staging/deploy.yaml
python3 scripts/upy/pack_bundle.py --stage .upy-stage --output "artifacts/bundle-linux-${GOARCH}.zip"
```

发布步骤（release job 下载产物到 artifacts 后）：
```yaml
- name: Publish Release
  env:
    GH_TOKEN: ${{ github.token }}
    GH_REPO: ${{ github.repository }}
    RELEASE_TAG: ${{ github.ref_name }}
  run: gh release create "$RELEASE_TAG" artifacts/*.zip artifacts/*.sha256 --verify-tag --generate-notes
```
checkout 必须提供 git tag（例如 fetch-depth: 0）。发布动作权限不足时准确报告，不改成将 PAT 写进 workflow。私有仓库需要服务器额外的读取凭据。

SHA256 文件用于人工/脚本校验，不代表 upy 自动校验。不要声称 CI 构建即完成了服务器 E2E；实际部署的基本流程：复制 server.deploy.yaml.example 为 <root>/deploy.yaml，准备 shared 环境和身份，再在授权测试服务器运行 `upy release <tag> --root <root>`；离线方式 `upy bundle <zip> --root <root>` 也会启动真实部署。
