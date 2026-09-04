// upy: 轻量级 Web 项目部署 CLI (Go 实现)
// 中文 CLI: 除名字外全部使用中文介绍
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/AaronConlon/upy/internal/commands"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AaronConlon/upy/internal/version"
)

func printHelp() {
	lines := []string{
		"upy " + version.String() + " — 轻量级 Web 项目部署工具",
		"",
		"用法: upy <命令> [选项]",
		"",
		"命令:",
		"  release [list|latest|<版本>]   从 GitHub Release 获取并部署 bundle",
		"                                 (不带参数时交互选择版本)",
		"  bundle <文件>                  直接部署本地 bundle.zip (不访问 GitHub)",
		"  version                        查看当前版本并检查更新",
		"  update                         自更新到最新版本",
		"  init [owner] <token>           初始化或添加 GitHub 访问 token (支持按组织/用户归属)",
		"",
		"选项:",
		"  --force                        跳过 '当前版本与目标一致' 的跳过逻辑，并强制重新下载",
		"  --root <目录>                  项目目录 (默认当前目录)",
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
		"  upy bundle ./bundle.zip       部署本地 bundle",
		"  upy init ghp_xxx              设置默认全局 GitHub token",
		"  upy init WeiaiHealth ghp_xxx  为指定组织设置专属 GitHub token",
	}
	fmt.Println(strings.Join(lines, "\n"))
}

type parsedArgs struct {
	cmd         string
	positional  []string
	force       bool
	root        string
	showVersion bool
	showHelp    bool
}

func parseArgs(argv []string) parsedArgs {
	var p parsedArgs
	known := map[string]bool{"release": true, "bundle": true, "version": true, "update": true, "init": true}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			p.showHelp = true
		case a == "-V" || a == "--version":
			p.showVersion = true
		case a == "--force":
			p.force = true
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
		printHelp()
		return
	case "release":
		target := ""
		if len(p.positional) > 0 {
			target = p.positional[0]
		}
		err = commands.Release(commands.ReleaseArgs{Target: target, Force: p.force, Root: root})
	case "bundle":
		file := ""
		if len(p.positional) > 0 {
			file = p.positional[0]
		}
		err = commands.Bundle(commands.BundleArgs{File: file, Force: p.force, Root: root})
	case "version":
		err = commands.VersionCmd()
	case "update":
		err = commands.UpdateCmd()
	case "init":
		err = commands.Init(commands.InitArgs{Args: p.positional})
	}

	if err != nil {
		log.Fail(err.Error())
		os.Exit(1)
	}
}
