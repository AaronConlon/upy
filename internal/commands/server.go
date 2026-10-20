package commands

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AaronConlon/upy/internal/server"
)

type ServerArgs struct {
	Action     string // "start", "stop", "restart", "status"
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
		log.Step("正在停止已有 server 进程...")
		_ = runDaemonStop()
		time.Sleep(200 * time.Millisecond)
		if args.Daemon && !args.Foreground {
			return runDaemonStart(serverCfg)
		}
		return runForegroundServer(serverCfg)

	case "status":
		return runDaemonStatus(serverCfg.EffectiveListen())

	default:
		return fmt.Errorf("未知操作 %q，支持的操作为: start, stop, restart, status", action)
	}
}

func runDaemonStart(cfg config.ServerConfig) error {
	log.Step("正在启动 upy server 后台静默进程...")
	pid, err := server.StartDaemon(server.DaemonFlags{
		Listen: cfg.EffectiveListen(),
		Token:  cfg.Token,
	})
	if err != nil {
		return err
	}

	log.Ok(fmt.Sprintf("upy server 已在后台静默运行 (PID: %d)", pid))
	log.Info("监听地址: " + log.Sky(cfg.EffectiveListen()))
	log.Info("运行日志: " + log.Path(server.ServerLogPath()))
	log.Info("查看状态: " + log.Cyan("upy server status"))
	log.Info("停止命令: " + log.Cyan("upy server stop"))
	return nil
}

func runForegroundServer(cfg config.ServerConfig) error {
	listenAddr := cfg.EffectiveListen()
	log.Step("正在启动 upy server (前台模式)...")
	log.Info("监听地址: " + log.Sky(listenAddr))
	if strings.TrimSpace(cfg.Token) != "" {
		log.RegisterSecret(cfg.Token)
		log.Info("全局 Token: 已启用 (***)")
	} else {
		log.Warn("全局 Token: 未配置 (建议在 ~/.upy/config.yaml 中配置 server.token)")
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

	log.Ok("upy server 就绪，按 Ctrl+C 可停止")

	select {
	case err := <-errChan:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		log.Step("接收到退出信号，正在优雅关闭服务...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Warn("优雅关闭出错: " + err.Error())
		}
		log.Ok("upy server 已安全退出")
	}

	return nil
}

func runDaemonStop() error {
	pid, err := server.StopDaemon()
	if err != nil {
		return err
	}
	log.Ok(fmt.Sprintf("upy server (PID: %d) 已成功停止", pid))
	return nil
}

func runDaemonStatus(configuredListen string) error {
	st, err := server.CheckDaemonStatus(configuredListen)
	if err != nil {
		return err
	}
	if !st.Running {
		log.Info("upy server 当前未在运行")
		return nil
	}

	log.Ok(fmt.Sprintf("upy server 正在运行 (PID: %s)", log.Green(strconv.Itoa(st.PID))))
	log.Info("监听地址: " + log.Sky(st.Listen))
	if st.Uptime != "" {
		log.Info("运行时长: " + st.Uptime)
	}
	if st.Version != "" {
		log.Info("版本信息: " + st.Version)
	}
	log.Info("日志文件: " + log.Path(server.ServerLogPath()))
	return nil
}
