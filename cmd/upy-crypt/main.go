// upy-crypt: 超轻量 Bundle 加密/解密 CLI 工具 (专为 CI/CD 设计)
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/AaronConlon/upy/internal/crypto"
	"golang.org/x/term"
)

var version = "dev"

func printUsage() {
	fmt.Fprintf(os.Stderr, `upy-crypt %s — 超轻量 Bundle 加密与解密工具 (AES-256-GCM)

用法:
  upy-crypt [选项] <输入文件> [输出文件]

选项:
  -p, --password <密码>   指定密码 (优先于环境变量; 也可用 UPY_CRYPT_PASSWORD 环境变量)
  -d, --decrypt           解密模式 (默认是加密模式)
  -v, --version           查看版本
  -h, --help              查看帮助

示例:
  # 加密 bundle (默认输出为 bundle.zip.enc)
  upy-crypt bundle.zip -p "my-secret"

  # 指定输出路径
  upy-crypt bundle.zip dist/bundle.zip.enc -p "my-secret"

  # 通过环境变量读取密码 (CI/CD 推荐)
  UPY_CRYPT_PASSWORD="my-secret" upy-crypt bundle.zip

  # 解密验证
  upy-crypt -d bundle.zip.enc restored.zip -p "my-secret"
`, version)
}

func main() {
	var (
		password    string
		decryptMode bool
		showVer     bool
		showHelp    bool
	)

	flag.StringVar(&password, "p", "", "加密或解密密码")
	flag.StringVar(&password, "password", "", "加密或解密密码")
	flag.BoolVar(&decryptMode, "d", false, "解密模式")
	flag.BoolVar(&decryptMode, "decrypt", false, "解密模式")
	flag.BoolVar(&showVer, "v", false, "查看版本")
	flag.BoolVar(&showVer, "version", false, "查看版本")
	flag.BoolVar(&showHelp, "h", false, "查看帮助")
	flag.BoolVar(&showHelp, "help", false, "查看帮助")

	flag.Usage = printUsage
	flag.Parse()

	if showHelp {
		printUsage()
		os.Exit(0)
	}

	if showVer {
		fmt.Printf("upy-crypt %s\n", version)
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	input := args[0]
	var output string
	if len(args) > 1 {
		output = args[1]
	} else {
		if decryptMode {
			if strings.HasSuffix(input, ".enc") {
				output = strings.TrimSuffix(input, ".enc")
			} else {
				output = input + ".dec"
			}
		} else {
			output = input + ".enc"
		}
	}

	// 解析密码：优先命令行参数 -> 环境变量 -> 终端交互
	pwd := strings.TrimSpace(password)
	if pwd == "" {
		for _, envKey := range []string{"UPY_CRYPT_PASSWORD", "UPY_PASSWORD", "DEPLOY_SECRET"} {
			if val := strings.TrimSpace(os.Getenv(envKey)); val != "" {
				pwd = val
				break
			}
		}
	}

	if pwd == "" {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprintln(os.Stderr, "错误: 未指定密码且当前非交互终端。请使用 -p 参数或设置 UPY_CRYPT_PASSWORD 环境变量。")
			os.Exit(1)
		}
		prompt := "请输入加密密码: "
		if decryptMode {
			prompt = "请输入解密密码: "
		}
		fmt.Fprint(os.Stderr, prompt)
		passBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: 读取密码失败: %v\n", err)
			os.Exit(1)
		}
		pwd = strings.TrimSpace(string(passBytes))
		if pwd == "" {
			fmt.Fprintln(os.Stderr, "错误: 密码不能为空")
			os.Exit(1)
		}
	}

	if decryptMode {
		if err := crypto.DecryptFile(input, output, pwd); err != nil {
			fmt.Fprintf(os.Stderr, "解密失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ 解密完成: %s -> %s\n", input, output)
	} else {
		if err := crypto.EncryptFile(input, output, pwd); err != nil {
			fmt.Fprintf(os.Stderr, "加密失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ 加密完成: %s -> %s\n", input, output)
	}
}

// 确保引用 io 避免未用报错
var _ = io.EOF
