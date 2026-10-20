// upy: 轻量级 Web 项目部署 CLI (Go 实现)
// 中文 CLI: 除名字外全部使用中文介绍
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AaronConlon/upy/internal/commands"
	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AaronConlon/upy/internal/platform"
	"github.com/AaronConlon/upy/internal/version"
)

func printHelp() {
	lines := []string{
		"upy " + version.String() + " — 轻量级 Web 项目部署工具",
		"",
		"用法: upy [命令] [选项]",
		"",
		"命令:",
		"  （不带命令）                  选择已登记的本地项目并交互部署",
		"  release [list|latest|<版本>]   从 GitHub Release 获取并部署 bundle",
		"                                 (不带参数时交互选择版本)",
		"  deploy [项目名称]              部署已登记项目的最新正式版本",
		"  clean [项目名称]               清理项目数据 (Bundle 缓存、旧版本、临时目录、注销或清空)",
		"  backup [项目名称]              执行项目约定备份 (执行 backup.sh 并管理 backups/ 归档)",
		"  analysis [项目名称]            分析客户端项目、磁盘空间占用与最近一次部署",
		"  bundle <文件>                  直接部署本地 bundle.zip (不访问 GitHub)",
		"  version                        查看当前版本并检查更新",
		"  update [版本]                  自更新到最新版本，或指定版本 (如: upy update v0.1.0)",
		"  init [owner] <token>           初始化或添加 GitHub 访问 token (支持按组织/用户归属)",
		"  server [start|stop|restart|status|auth]     启动 Webhook 服务或管理后台守护进程",
		"  log [server|upy|<项目>]        查看自身、server 或项目的运行与部署日志",
		"",
		"选项:",
		"  --force                        跳过 '当前版本与目标一致' 的跳过逻辑，并强制重新下载",
		"  --root <目录>                  项目目录 (默认当前目录)",
		"  -f, --follow                   log 专用: 实时追踪后续新增日志输出",
		"  -n, --tail <行数>              log 专用: 查看末尾 N 行日志 (默认 50)",
		"  -c, --clear                    log 专用: 清空指定的日志文件",
		"  -s, --server                   log 专用: 直接查看 server 服务日志",
		"  -d, --daemon                   server 专用: 以后台静默守护进程形式启动",
		"  --listen <地址>                server 专用: 指定监听地址 (如 :8999 或 127.0.0.1:8999)",
		"  --token <密钥>                 server 专用: 指定 API 鉴权 Token",
		"  --detail                       analysis 专用: 展开子目录 (releases/bundles/backups) 空间明细",
		"  --json                         analysis 专用: 输出 JSON 结构化分析数据",
		"  --sort <size|time|name>        analysis 专用: 指定排序规则 (默认按磁盘占用 size 降序)",
		"  --bundles                      clean 专用: 仅清理 Bundle 缓存压缩包",
		"  --old-releases                 clean 专用: 仅清理历史旧版本与残留临时目录",
		"  --all                          clean / backup 通用: 批量处理所有已登记项目",
		"  --purge                        clean 专用: 彻底清空项目所有部署产物",
		"  --unregister                   clean 专用: 从本地项目列表中注销此项目",
		"  --list                         backup 专用: 查看历史备份归档列表与体积",
		"  --prune <数量>                 backup 专用: 轮转保留最近 N 份备份",
		"  --script <路径>                backup 专用: 指定自定义备份脚本路径",
		"  -y, --yes                      clean / backup 专用: 跳过交互确认直接执行",
		"  -V, --version                  打印版本后退出",
		"  -h, --help                     打印帮助后退出",
		"",
		"配置:",
		"  ~/.upy/config.yaml            用户配置 (可用 UPY_CONFIG 覆盖)",
		"  github.token                   默认 GitHub 访问令牌",
		"  github.tokens.<owner>          按组织或个人归属的 GitHub 访问令牌",
		"",
		"环境变量:",
		"  DEPLOY_GITHUB_TOKEN            GitHub 访问令牌兜底 (配置文件未写时使用)",
		"  UPY_CONFIG                    覆盖用户配置文件路径",
		"",
		"示例:",
		"  upy release list              查看项目仓库的发布历史",
		"  upy release latest            部署最新版本",
		"  upy release v0.3.0            部署指定版本",
		"  upy deploy codia              部署本机项目 codia 的最新版本",
		"  upy clean                     交互式选择项目并清理数据",
		"  upy clean codia               清理指定项目 codia 的数据",
		"  upy clean --all -y            静默执行当前项目常规瘦身",
		"  upy backup                    交互式选择项目并执行约定备份",
		"  upy backup codia              执行指定项目 codia 的约定备份",
		"  upy backup codia --list       查看 codia 项目的历史备份列表",
		"  upy backup --all -y           一键批量备份所有已登记项目",
		"  upy analysis                  分析全机所有已部署项目及磁盘占用柱状图",
		"  upy analysis --detail         查看全机项目各子目录细分空间分布",
		"  upy analysis codia            查看指定项目的磁盘占用与子模块分布",
		"  upy analysis --json           以 JSON 格式输出分析报表",
		"  upy bundle ./bundle.zip       部署本地 bundle",
		"  upy init ghp_xxx              设置默认全局 GitHub token",
		"  upy init WeiaiHealth ghp_xxx  为指定组织设置专属 GitHub token",
		"  upy update                    自更新到最新版本",
		"  upy update v0.1.0             更新/切换到指定版本",
		"  upy server auth               在服务器终端显示 TOTP 绑定二维码",
		"  upy log                       交互式选择并查看日志",
		"  upy log server -f             实时查看 server 服务运行日志",
		"  upy log upy -n 100            查看 upy 自身最近 100 行执行流水",
		"  upy log codia                 查看 codia 项目最近一次部署日志",
		"  upy log --list                列出本机所有日志文件及体积",
	}
	fmt.Println(strings.Join(lines, "\n"))
}

type parsedArgs struct {
	cmd         string
	positional  []string
	force       bool
	root        string
	bundles     bool
	oldReleases bool
	all         bool
	purge       bool
	unregister  bool
	yes         bool
	list        bool
	prune       int
	script      string
	detail      bool
	json        bool
	sort        string
	daemon      bool
	foreground  bool
	listen      string
	token       string
	follow      bool
	tail        int
	clear       bool
	serverLog   bool
	showVersion bool
	showHelp    bool
}

func parseArgs(argv []string) parsedArgs {
	var p parsedArgs
	known := map[string]bool{
		"release": true, "deploy": true, "clean": true, "backup": true,
		"analysis": true, "bundle": true, "version": true, "update": true, "init": true,
		"server": true, "log": true, "logs": true,
	}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			p.showHelp = true
		case a == "-V" || a == "--version":
			p.showVersion = true
		case a == "-f" || a == "--follow":
			p.follow = true
		case a == "-c" || a == "--clear":
			p.clear = true
		case a == "-s" || a == "--server":
			p.serverLog = true
		case a == "-n" || a == "--tail" || a == "--lines":
			if i+1 < len(argv) {
				i++
				if n, err := strconv.Atoi(argv[i]); err == nil {
					p.tail = n
				}
			}
		case strings.HasPrefix(a, "--tail="):
			if n, err := strconv.Atoi(strings.TrimPrefix(a, "--tail=")); err == nil {
				p.tail = n
			}
		case strings.HasPrefix(a, "--lines="):
			if n, err := strconv.Atoi(strings.TrimPrefix(a, "--lines=")); err == nil {
				p.tail = n
			}
		case a == "-d" || a == "--daemon":
			p.daemon = true
		case a == "--foreground":
			p.foreground = true
		case a == "--listen" || a == "--port":
			if i+1 < len(argv) {
				i++
				p.listen = argv[i]
			}
		case strings.HasPrefix(a, "--listen="):
			p.listen = strings.TrimPrefix(a, "--listen=")
		case strings.HasPrefix(a, "--port="):
			p.listen = strings.TrimPrefix(a, "--port=")
		case a == "--token":
			if i+1 < len(argv) {
				i++
				p.token = argv[i]
			}
		case strings.HasPrefix(a, "--token="):
			p.token = strings.TrimPrefix(a, "--token=")
		case a == "--force":
			p.force = true
		case a == "--detail":
			p.detail = true
		case a == "--json":
			p.json = true
		case a == "--sort":
			if i+1 < len(argv) {
				i++
				p.sort = argv[i]
			}
		case strings.HasPrefix(a, "--sort="):
			p.sort = strings.TrimPrefix(a, "--sort=")
		case a == "--bundles":
			p.bundles = true
		case a == "--old-releases":
			p.oldReleases = true
		case a == "--all":
			p.all = true
		case a == "--purge":
			p.purge = true
		case a == "--unregister":
			p.unregister = true
		case a == "--list":
			p.list = true
		case a == "--script":
			if i+1 < len(argv) {
				i++
				p.script = argv[i]
			}
		case strings.HasPrefix(a, "--script="):
			p.script = strings.TrimPrefix(a, "--script=")
		case a == "--prune":
			if i+1 < len(argv) {
				i++
				if n, err := strconv.Atoi(argv[i]); err == nil {
					p.prune = n
				}
			}
		case strings.HasPrefix(a, "--prune="):
			if n, err := strconv.Atoi(strings.TrimPrefix(a, "--prune=")); err == nil {
				p.prune = n
			}
		case a == "-y" || a == "--yes":
			p.yes = true
		case a == "--root":
			if i+1 < len(argv) {
				i++
				p.root = argv[i]
			}
		case strings.HasPrefix(a, "--root="):
			p.root = strings.TrimPrefix(a, "--root=")
		case p.cmd == "" && known[a]:
			p.cmd = a
		default:
			p.positional = append(p.positional, a)
		}
	}
	return p
}

func main() {
	platform.SetDefaultUmask()
	cmdStartedAt := time.Now()

	logCloser := log.EnableFileLog(filepath.Join(config.UserDataDir(), "logs"), os.Args)
	defer logCloser()

	if len(os.Args) == 2 && os.Args[1] == "__install-completion" {
		result, err := commands.InstallShellCompletion()
		if err != nil {
			log.Fail("命令补全安装失败: " + err.Error())
			os.Exit(1)
		}
		if !result.Supported {
			log.Info("检测到当前 shell: " + result.Shell + "（暂不支持自动安装 Tab 补全）")
			return
		}
		log.Info("检测到当前 shell: " + result.Shell)
		if len(result.ScriptPaths) > 1 {
			log.Ok("已安装 Tab 命令补全 (" + strings.Join(result.ScriptPaths, ", ") + ")（新开终端后生效）")
		} else {
			log.Ok("已安装 Tab 命令补全: " + result.ScriptPath + "（新开终端后生效）")
		}
		return
	}

	p := parseArgs(os.Args[1:])

	if p.showHelp {
		printHelp()
		return
	}
	if p.showVersion {
		fmt.Println("upy " + version.String())
		return
	}

	root := p.root
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			log.Fail("无法获取当前目录: " + err.Error())
			os.Exit(1)
		}
		root = cwd
	}

	var err error
	switch p.cmd {
	case "":
		err = commands.SelectProjectAndRelease(p.force, cmdStartedAt)
	case "release":
		target := ""
		if len(p.positional) > 0 {
			target = p.positional[0]
		}
		err = commands.Release(commands.ReleaseArgs{Target: target, Force: p.force, Root: root, StartedAt: cmdStartedAt})
	case "deploy":
		projectName := ""
		if len(p.positional) > 0 {
			projectName = p.positional[0]
		}
		err = commands.Deploy(projectName, p.force, cmdStartedAt)
	case "clean":
		projectName := ""
		if len(p.positional) > 0 {
			projectName = p.positional[0]
		}
		err = commands.Clean(commands.CleanArgs{
			ProjectName: projectName,
			Root:        p.root,
			Bundles:     p.bundles,
			OldReleases: p.oldReleases,
			All:         p.all,
			Purge:       p.purge,
			Unregister:  p.unregister,
			Yes:         p.yes,
		})
	case "backup":
		projectName := ""
		if len(p.positional) > 0 {
			projectName = p.positional[0]
		}
		err = commands.Backup(commands.BackupArgs{
			ProjectName: projectName,
			Root:        p.root,
			Script:      p.script,
			List:        p.list,
			Prune:       p.prune,
			All:         p.all,
			Yes:         p.yes,
		})
	case "analysis":
		projectName := ""
		if len(p.positional) > 0 {
			projectName = p.positional[0]
		}
		err = commands.Analysis(commands.AnalysisArgs{
			ProjectName: projectName,
			Root:        p.root,
			Detail:      p.detail,
			JSON:        p.json,
			Sort:        p.sort,
		})
	case "bundle":
		file := ""
		if len(p.positional) > 0 {
			file = p.positional[0]
		}
		err = commands.Bundle(commands.BundleArgs{File: file, Force: p.force, Root: root, StartedAt: cmdStartedAt})
	case "version":
		err = commands.VersionCmd()
	case "update":
		target := ""
		if len(p.positional) > 0 {
			target = p.positional[0]
		}
		err = commands.UpdateCmd(target, p.force)
	case "init":
		err = commands.Init(commands.InitArgs{Args: p.positional})
	case "server":
		action := ""
		if len(p.positional) > 0 {
			action = p.positional[0]
		}
		err = commands.Server(commands.ServerArgs{
			Action:     action,
			Daemon:     p.daemon,
			Foreground: p.foreground,
			Listen:     p.listen,
			Token:      p.token,
		})
	case "log", "logs":
		target := ""
		if len(p.positional) > 0 {
			target = p.positional[0]
		}
		err = commands.Log(commands.LogArgs{
			Target: target,
			Follow: p.follow,
			Tail:   p.tail,
			Clear:  p.clear,
			List:   p.list,
			Server: p.serverLog,
		})
	}

	if err != nil {
		log.Fail(err.Error())
		os.Exit(1)
	}
}
