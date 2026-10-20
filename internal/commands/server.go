package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/i18n"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AaronConlon/upy/internal/server"
	"golang.org/x/term"
)

type ServerArgs struct {
	Action     string // "start", "stop", "restart", "status", "auth"
	Daemon     bool   // -d, --daemon
	Foreground bool   // --foreground
	Listen     string // --listen, --port
	Token      string // --token
}

func Server(args ServerArgs) error {
	action := strings.ToLower(strings.TrimSpace(args.Action))
	if action == "" {
		action = "start"
	}

	userCfg, err := config.LoadUserConfig()
	if err != nil {
		return err
	}
	serverCfg := userCfg.Server
	if args.Listen != "" {
		serverCfg.Listen = args.Listen
	}
	if args.Token != "" {
		serverCfg.Token = args.Token
	}

	switch action {
	case "start":
		if args.Daemon && !args.Foreground {
			return runDaemonStart(serverCfg)
		}
		return runForegroundServer(serverCfg)

	case "stop":
		return runDaemonStop()

	case "restart":
		log.Step(i18n.T("正在停止已有 server 进程..."))
		_ = runDaemonStop()
		time.Sleep(200 * time.Millisecond)
		if args.Daemon && !args.Foreground {
			return runDaemonStart(serverCfg)
		}
		return runForegroundServer(serverCfg)

	case "auth":
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			return i18n.Error("请在交互终端执行 upy server auth；二维码不会写入管道或日志")
		}
		return renderServerAuth(serverCfg, os.Stdout)
	case "status":
		return runDaemonStatus(serverCfg.EffectiveListen())

	default:
		return fmt.Errorf(i18n.T("未知操作 %q，支持的操作为: start, stop, restart, status, auth"), action)
	}
}

func runDaemonStart(cfg config.ServerConfig) error {
	log.Step(i18n.T("正在启动 upy server 后台静默进程..."))
	pid, err := server.StartDaemon(server.DaemonFlags{
		Listen: cfg.EffectiveListen(),
		Token:  cfg.Token,
	})
	if err != nil {
		return err
	}

	log.Ok(fmt.Sprintf(i18n.T("upy server 已在后台静默运行 (PID: %d)"), pid))
	log.Info(i18n.T("监听地址: ") + log.Sky(cfg.EffectiveListen()))
	log.Info(i18n.T("运行日志: ") + log.Path(server.ServerLogPath()))
	log.Info(i18n.T("查看状态: ") + log.Cyan("upy server status"))
	log.Info(i18n.T("绑定认证器: ") + log.Cyan("upy server auth") + i18n.T("（在终端显示二维码）"))
	log.Info(i18n.T("停止命令: ") + log.Cyan("upy server stop"))
	return nil
}

func runForegroundServer(cfg config.ServerConfig) error {
	listenAddr := cfg.EffectiveListen()
	log.Step(i18n.T("正在启动 upy server (前台模式)..."))
	log.Info(i18n.T("监听地址: ") + log.Sky(listenAddr))
	if strings.TrimSpace(cfg.Token) != "" {
		log.RegisterSecret(cfg.Token)
		log.Info(i18n.T("全局 Token: 已启用 (***)"))
	} else {
		log.Warn(i18n.T("全局 Token: 未配置 (建议在 ~/.upy/config.yaml 中配置 server.token)"))
	}

	srv := server.NewServer(cfg, os.Stderr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errChan := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
		close(errChan)
	}()

	log.Ok(i18n.T("upy server 就绪，按 Ctrl+C 可停止"))
	log.Info(i18n.T("绑定认证器: 请在另一个终端运行 ") + log.Cyan("upy server auth"))

	select {
	case err := <-errChan:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		log.Step(i18n.T("接收到退出信号，正在优雅关闭服务..."))
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Warn(i18n.T("优雅关闭出错: ") + err.Error())
		}
		log.Ok(i18n.T("upy server 已安全退出"))
	}

	return nil
}

func runDaemonStop() error {
	pid, err := server.StopDaemon()
	if err != nil {
		return err
	}
	log.Ok(fmt.Sprintf(i18n.T("upy server (PID: %d) 已成功停止"), pid))
	return nil
}

func runDaemonStatus(configuredListen string) error {
	st, err := server.CheckDaemonStatus(configuredListen)
	if err != nil {
		return err
	}
	if !st.Running {
		log.Info(i18n.T("upy server 当前未在运行"))
		return nil
	}

	log.Ok(fmt.Sprintf(i18n.T("upy server 正在运行 (PID: %s)"), log.Green(strconv.Itoa(st.PID))))
	log.Info(i18n.T("监听地址: ") + log.Sky(st.Listen))
	if st.Uptime != "" {
		log.Info(i18n.T("运行时长: ") + st.Uptime)
	}
	if st.Version != "" {
		log.Info(i18n.T("版本信息: ") + st.Version)
	}
	log.Info(i18n.T("日志文件: ") + log.Path(server.ServerLogPath()))
	return nil
}

// 认证器绑定材料直接写终端，不能经过 log.Info 等会落盘的函数。
func renderServerAuth(cfg config.ServerConfig, out io.Writer) error {
	tm, err := server.NewTOTPManager("", cfg.TOTPSecret)
	if err != nil {
		return err
	}
	maxViews := cfg.EffectiveTOTPMaxViews()
	remaining, allowed, err := tm.RecordView(maxViews)
	if err != nil {
		return fmt.Errorf(i18n.T("读取绑定次数失败: %w"), err)
	}
	if !allowed {
		return fmt.Errorf(i18n.T("绑定二维码查看次数已达上限 (%d/%d 次)，已禁止再次渲染"), maxViews, maxViews)
	}
	host, _ := os.Hostname()
	account := "admin"
	if host != "" {
		account += "@" + host
	}
	qr, err := tm.TerminalQRCode(account)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, i18n.T("upy · 认证器绑定\n\n请用 Google Authenticator、1Password 等扫描下方二维码：\n\n%s\n绑定后访问控制台首页，输入 6 位动态验证码登录。\n此命令复用现有密钥（剩余展示次数：%d / %d 次）。\n"), qr, remaining, maxViews)
	return err
}
