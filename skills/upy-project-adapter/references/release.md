# 应用 GitHub Actions / Releases

构建 bundle 的工作在应用仓库，upy 自身 CLI 的二进制发布 workflow 不能直接复制成应用 workflow。

生成完整项目专用 workflow，优先沿用已验证的 Actions 版本。最小结构：tag v* 触发；全局 contents: read；checkout 对应 tag；安装项目实际工具链并冻结依赖；项目测试；构建白名单 staging；生成 JSON 格式 `upy.yaml`；打 ZIP 与 SHA256；将文件作为 Actions artifact 上传；统一 release job 下载全部 artifact 后发布 GitHub Release，只有该 job contents: write。

并发控制按 workflow+tag 分组、cancel-in-progress: false，避免同 tag 发布中断；同名 Release 重跑是否覆盖资产要明确选择，默认不可变发布。使用 gh release create 并 --verify-tag；已有同 tag Release 不要默默 --clobber。

多架构 Go 使用 matrix arch: [amd64, arm64]、fail-fast: false；每个 job 输出 bundle-linux-<arch>.zip 和各自同名 .sha256，Actions artifact 名也唯一。最终 release job needs build，只在全部构建成功后发布全部资产。服务器配置 asset 精确匹配本机。静态一般一个 bundle.zip 即可，不强行架构矩阵。

构建脚本参数传版本/架构/输出路径。tag 从 GitHub env 传入 shell 变量，验证格式后使用，不直接把外部字符串插入 run shell。建议只支持 tag push；如果需要 workflow_dispatch，显式输入 tag，先验证远端 tag 存在并 checkout 该 tag，不能在默认分支构建后冒充旧版本。version 去除 v 前缀必须与 manifest 和说明一致，拒绝含 /、.. 等路径字符。

构建 ZIP 示例：
```sh
set -euo pipefail
VERSION="${RELEASE_TAG#v}"
# 项目构建后，将 JSON manifest 写入 staging/upy.yaml
python3 scripts/upy/pack_bundle.py --stage .upy-stage --output "artifacts/bundle-linux-${GOARCH}.zip"

# 可选：若项目要求加密分发（例如存放在组织公开 Release 供国内高速镜像免 Token 下载），可调用 upy 加密
# upy encrypt "artifacts/bundle-linux-${GOARCH}.zip" "artifacts/bundle-linux-${GOARCH}.zip.enc" --password "$ENCRYPT_KEY"
```

发布步骤（release job 下载产物到 artifacts 后）：
```yaml
- name: Publish Release
  env:
    GH_TOKEN: ${{ github.token }}
    GH_REPO: ${{ github.repository }}
    RELEASE_TAG: ${{ github.ref_name }}
  run: gh release create "$RELEASE_TAG" artifacts/*.zip* artifacts/*.sha256 --verify-tag --generate-notes
```
checkout 必须提供 git tag（例如 fetch-depth: 0）。发布动作权限不足时准确报告，不改成将 PAT 写进 workflow。私有仓库需要服务器额外的读取凭据。

## CI 发布后触发 upy Webhook（upy v0.4.0+）

如果要求 Release 创建成功后自动部署，在 `release` job 后增加独立的 `deploy` job，并通过仓库 `Settings -> Secrets and variables -> Actions` 配置：

- `UPY_WEBHOOK_URL`：完整 URL，例如 `https://deploy.example.com/api/v1/webhook/demo-web`；路径项目名必须与服务器 `<root>/deploy.yaml` 的 `name` 和首次部署登记名完全一致。
- `UPY_WEBHOOK_SECRET`：与服务器 `<root>/deploy.yaml` 的 `webhook.secret` 完全相同的随机密钥。

服务器上先运行 `upy server`，通常只监听 `127.0.0.1:8999`，再由 HTTPS reverse proxy 转发 `/api/v1/webhook/`；不要把没有密钥的 server 监听器直接暴露到公网。项目必须先手动成功执行一次 `upy release <tag> --root <root>`，让 upy 登记本地项目，之后 CI Webhook 才不会收到 `404`。

推荐的 deploy job 使用精确 Release tag，并按 upy 支持的 GitHub HMAC 格式签名请求正文：

```yaml
  deploy:
    needs: release
    runs-on: ubuntu-latest
    permissions: {}
    steps:
      - name: Trigger upy deployment
        env:
          UPY_WEBHOOK_URL: ${{ secrets.UPY_WEBHOOK_URL }}
          UPY_WEBHOOK_SECRET: ${{ secrets.UPY_WEBHOOK_SECRET }}
          RELEASE_TAG: ${{ github.ref_name }}
        run: |
          set -euo pipefail
          test -n "$UPY_WEBHOOK_URL"
          test -n "$UPY_WEBHOOK_SECRET"
          payload="$(python3 -c 'import json, os; print(json.dumps({"target": os.environ["RELEASE_TAG"], "force": False}, separators=(",", ":")))')"
          signature="$(printf %s "$payload" | openssl dgst -sha256 -hmac "$UPY_WEBHOOK_SECRET" | sed 's/^.*= //')"
          curl --fail-with-body --silent --show-error --request POST "$UPY_WEBHOOK_URL" \
            --header 'Content-Type: application/json' \
            --header "X-Hub-Signature-256: sha256=$signature" \
            --data "$payload"
```

`202 Accepted` 只表示 upy 已异步创建部署任务；它不是服务器已完成升级的证明。workflow 应将该 job 描述为触发部署，不要声称完成了服务器 E2E。`401` 通常是 secret/签名不匹配，`404` 通常是项目未登记或 URL 中项目名错误，`409` 表示同一项目已有部署任务。若需要 CI 等待最终结果，必须基于目标 upy 版本实际提供的 task API 设计轮询与超时，不要凭空添加 `upy status` 或 `upy validate`。

如果不使用 HMAC，也可用 `Authorization: Bearer <token>` 或 `X-Upy-Token`；CI 里的 `github.token` 只用于创建 GitHub Release，不是服务器长期读取凭据或 Webhook secret。

SHA256 文件用于人工/脚本校验，不代表 upy 自动校验。不要声称 CI 构建即完成了服务器 E2E；实际部署的基本流程：复制 server.deploy.yaml.example 为 <root>/deploy.yaml，准备 shared 环境和身份，再在授权测试服务器运行 `upy release <tag> --root <root>`；离线方式 `upy bundle <zip> --root <root>` 也会启动真实部署。
