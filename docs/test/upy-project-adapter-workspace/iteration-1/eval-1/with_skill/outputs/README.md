# demo-web upy 接入交付

使用 /Users/aaron/repos/upy/skills/upy-project-adapter/SKILL.md 及其配置、static、Release 资源生成。所有文件保存在本 outputs 中，按相对目录映射至 owner/demo-web；当前目录不是目标应用，不能在这里运行应用构建脚本。

- deploy/upy/server.deploy.yaml.example：服务器配置，复制到部署 root/deploy.yaml。
- deploy/upy/nginx.conf.example：SPA 路由与静态资源服务示例。
- scripts/upy/build-bundle.sh：Node 20 / pnpm 9.15.0 冻结安装、实际构建、白名单打包入口。
- scripts/upy/pack_bundle.py：从技能原样复制的标准库打包器，生成 ZIP 与 SHA256。
- .github/workflows/upy-release.yml：v* tag 构建、artifact、独立 Release job。
- docs/upy-deployment.md：集成、认证、首次部署、升级、失败演练及待确认项。

尚需确认服务器 root、域名/TLS、upy 目标版本、权限、项目 build 与测试命令。具体默认值与占位说明见部署文档。

验证范围：仅执行现有 upy --help，并对生成的 Bash/Python 进行语法检查；没有目标源码或服务器，未执行 pnpm 安装、应用构建、打包产物测试、Actions、Nginx 或部署测试。未推送、发布或运行部署。
