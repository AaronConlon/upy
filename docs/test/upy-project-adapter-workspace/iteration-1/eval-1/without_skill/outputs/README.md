# owner/demo-web upy 集成文件

本目录为独立评估产物，仅生成集成材料。没有修改目标应用，也没有进行真实构建、发布或部署；未读取任何 upy-project-adapter skill。

文件映射：

- `scripts/upy/build-bundle.sh` → 目标仓库同路径，使用 bash 调用。
- `.github/workflows/release.yml` → 目标仓库同路径，tag v* 构建并发布 Release 资产。
- `server/deploy.yaml` → 服务器 `/srv/demo-web/deploy.yaml`。
- `server/nginx.conf.example` → 供运维合并至实际 Nginx 站点配置。
- `docs/UPY_DEPLOYMENT.md` → 集成步骤、打包布局、服务器准备、部署验收及回退说明。
- `README.md` → 本文件，保存及使用说明。

契约依据：当前 upy internal/config/project.go、internal/config/manifest.go、internal/deployer/static.go、internal/commands/release.go、internal/commands/bundle.go、internal/config/user.go。两份 deploy.yaml 用途不同：服务器文件声明 Release 来源，ZIP 根清单声明 version/type/static.directory。ZIP 清单由构建脚本按 tag 生成。

没有目标源码，故不替换 package.json、lockfile 或 Vite 配置。默认已有 pnpm run build；真实域名、TLS、VITE_* 和服务器权限须集成时按环境配置。
