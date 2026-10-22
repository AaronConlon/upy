// upy encrypt <输入文件> [输出文件] [--password <密码>]
package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/AaronConlon/upy/internal/crypto"
	"github.com/AaronConlon/upy/internal/i18n"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AlecAivazis/survey/v2"
	"golang.org/x/term"
)

// EncryptArgs encrypt 命令参数
type EncryptArgs struct {
	Input    string
	Output   string
	Password string
}

// Encrypt 处理 upy encrypt 命令
func Encrypt(args EncryptArgs) error {
	input := strings.TrimSpace(args.Input)
	if input == "" {
		return i18n.Error("encrypt 命令需要指定输入文件: upy encrypt <输入文件> [输出文件] --password <密码>")
	}
	if _, err := os.Stat(input); err != nil {
		return fmt.Errorf(i18n.T("未找到输入文件: %s"), input)
	}

	output := strings.TrimSpace(args.Output)
	if output == "" {
		output = input + ".enc"
	}

	password := strings.TrimSpace(args.Password)
	if password == "" {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return i18n.Error("未指定密码且当前非交互终端，请使用 --password 参数指定加密密码")
		}
		prompt := &survey.Password{
			Message: i18n.T("请输入加密密码:"),
		}
		if err := survey.AskOne(prompt, &password); err != nil {
			return err
		}
		password = strings.TrimSpace(password)
		if password == "" {
			return i18n.Error("加密密码不能为空")
		}
	}

	log.Step(fmt.Sprintf(i18n.T("正在加密 %s -> %s..."), log.Path(input), log.Path(output)))
	if err := crypto.EncryptFile(input, output, password); err != nil {
		return fmt.Errorf(i18n.T("加密失败: %w"), err)
	}
	log.Ok(fmt.Sprintf(i18n.T("加密完成: %s"), log.Path(output)))
	return nil
}
