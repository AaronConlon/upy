// 外部命令执行包装: 一律使用 args 数组, 不拼 shell 字符串
package cmdrun

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/AaronConlon/upy/internal/log"
)

// Options 执行选项
type Options struct {
	Cwd          string
	Env          []string
	AllowNonZero bool
}

// Call 一次命令调用的记录 (测试辅助)
type Call struct {
	Cmd  string
	Args []string
}

// Run 执行命令并透传 stdio, 非零退出码返回错误。
// 保留为变量以便测试中替换为桩函数; 恢复时赋回 RealRun。
var Run = RealRun

// Output 执行命令并返回 stdout。用于查询 Docker Compose 状态，不透传标准输出。
// 保留为变量以便测试中替换为桩函数; 恢复时赋回 RealOutput。
var Output = RealOutput

// RealRun Run 的真实实现
func RealRun(cmd string, args []string, opts Options) error {
	c := exec.Command(cmd, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	if opts.Cwd != "" {
		c.Dir = opts.Cwd
	}
	c.Env = append(os.Environ(), opts.Env...)
	err := c.Run()
	if err == nil {
		return nil
	}
	if opts.AllowNonZero {
		return nil
	}
	code := 1
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	}
	msg := fmt.Sprintf("命令执行失败（退出码 %d）: %s %s", code, cmd, strings.Join(args, " "))
	log.Fail(msg)
	return fmt.Errorf("%s", log.Redact(msg))
}

// RealOutput 执行命令并捕获 stdout；stderr 仍透传，便于定位查询命令的配置错误。
func RealOutput(cmd string, args []string, opts Options) (string, error) {
	c := exec.Command(cmd, args...)
	c.Stderr = os.Stderr
	if opts.Cwd != "" {
		c.Dir = opts.Cwd
	}
	c.Env = append(os.Environ(), opts.Env...)
	out, err := c.Output()
	if err == nil {
		return string(out), nil
	}
	code := 1
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	}
	msg := fmt.Sprintf("命令执行失败（退出码 %d）: %s %s", code, cmd, strings.Join(args, " "))
	return "", fmt.Errorf("%s", log.Redact(msg))
}
