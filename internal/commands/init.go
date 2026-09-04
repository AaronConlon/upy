// upy init: 初始化或添加 GitHub token 配置
package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AlecAivazis/survey/v2"
	"golang.org/x/term"
)

type InitArgs struct {
	Args []string
}

// Init 处理 upy init 命令
func Init(args InitArgs) error {
	var owner, token string

	// 解析传入参数
	raw := args.Args
	for i := 0; i < len(raw); i++ {
		a := raw[i]
		switch {
		case a == "--owner" || a == "-o":
			if i+1 < len(raw) {
				i++
				owner = raw[i]
			}
		case strings.HasPrefix(a, "--owner="):
			owner = strings.TrimPrefix(a, "--owner=")
		case a == "--token" || a == "-t":
			if i+1 < len(raw) {
				i++
				token = raw[i]
			}
		case strings.HasPrefix(a, "--token="):
			token = strings.TrimPrefix(a, "--token=")
		default:
			if owner == "" && token == "" {
				// 检查是否形如 owner:token
				if idx := strings.Index(a, ":"); idx > 0 {
					owner = a[:idx]
					token = a[idx+1:]
				} else {
					token = a
				}
			} else if owner == "" && token != "" {
				// 原先 token 占了第 1 个位置，说明形式是: upy init <owner> <token>
				owner = token
				token = a
			}
		}
	}

	// 若未传 token 且处于交互终端中，发起友好交互提示
	if strings.TrimSpace(token) == "" {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return fmt.Errorf("请提供 GitHub token。\n用法:\n  upy init <token>              # 设置全局默认 token\n  upy init <owner> <token>      # 为组织或个人仓库设置专属 token\n  upy init <owner>:<token>      # 冒号简写形式")
		}

		if strings.TrimSpace(owner) == "" {
			ownerPrompt := &survey.Input{
				Message: "归属组织或个人用户名 (留空则作为全局默认 token):",
			}
			if err := survey.AskOne(ownerPrompt, &owner); err != nil {
				return err
			}
		}

		tokenPrompt := &survey.Password{
			Message: "GitHub Token:",
		}
		if err := survey.AskOne(tokenPrompt, &token); err != nil {
			return err
		}
	}

	token = strings.TrimSpace(token)
	owner = strings.TrimSpace(owner)
	if token == "" {
		return fmt.Errorf("GitHub token 不能为空")
	}

	path, err := config.SaveGitHubToken(owner, token)
	if err != nil {
		return err
	}

	targetDesc := "全局默认 (default)"
	if owner != "" && !strings.EqualFold(owner, "default") {
		targetDesc = fmt.Sprintf("组织/个人 [%s]", owner)
	}

	log.Ok(fmt.Sprintf("GitHub token 已成功保存到 %s", path))
	log.Info(fmt.Sprintf("归属范围: %s", targetDesc))
	return nil
}
