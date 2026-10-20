package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AaronConlon/upy/internal/config"
)

type DaemonFlags struct {
	Listen string
	Token  string
}

type DaemonStatus struct {
	Running bool   `json:"running"`
	PID     int    `json:"pid"`
	Listen  string `json:"listen"`
	Uptime  string `json:"uptime,omitempty"`
	Version string `json:"version,omitempty"`
}

func PIDFilePath() string {
	return filepath.Join(config.UserDataDir(), "server.pid")
}

func ServerLogPath() string {
	return filepath.Join(config.UserDataDir(), "logs", "server.log")
}

type PIDInfo struct {
	PID    int
	Listen string
}

func ReadPIDInfo() (*PIDInfo, error) {
	data, err := os.ReadFile(PIDFilePath())
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return nil, errors.New("empty pid file")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return nil, fmt.Errorf("无效的 PID 内容 %q: %w", lines[0], err)
	}
	listen := ""
	if len(lines) > 1 {
		listen = strings.TrimSpace(lines[1])
	}
	return &PIDInfo{PID: pid, Listen: listen}, nil
}

func ReadPID() (int, error) {
	info, err := ReadPIDInfo()
	if err != nil {
		return 0, err
	}
	return info.PID, nil
}

func WritePID(pid int, listen ...string) error {
	dir := filepath.Dir(PIDFilePath())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	content := strconv.Itoa(pid) + "\n"
	if len(listen) > 0 && strings.TrimSpace(listen[0]) != "" {
		content += strings.TrimSpace(listen[0]) + "\n"
	}
	return os.WriteFile(PIDFilePath(), []byte(content), 0o644)
}

func RemovePID() error {
	err := os.Remove(PIDFilePath())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// StartDaemon 以后台静默子进程形式拉起 upy server
func StartDaemon(flags DaemonFlags) (int, error) {
	if pid, err := ReadPID(); err == nil && isProcessAlive(pid) {
		return pid, fmt.Errorf("upy server 已经在运行中 (PID: %d)", pid)
	}

	exe, err := os.Executable()
	if err != nil {
		exe = "upy"
	}

	// 保证日志目录存在
	logPath := ServerLogPath()
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return 0, fmt.Errorf("创建 server 日志目录失败: %w", err)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, fmt.Errorf("打开 server 日志文件失败: %w", err)
	}
	defer logFile.Close()

	// 组装参数
	args := []string{"server", "--foreground"}
	if flags.Listen != "" {
		args = append(args, "--listen", flags.Listen)
	}
	if flags.Token != "" {
		args = append(args, "--token", flags.Token)
	}

	cmd := exec.Command(exe, args...)
	devNull, err := os.Open(os.DevNull)
	if err == nil {
		cmd.Stdin = devNull
		defer devNull.Close()
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// 平台特定的脱离终端会话设置 (Setsid / CREATE_NEW_PROCESS_GROUP)
	setDetachedSysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("启动后台静默进程失败: %w", err)
	}

	pid := cmd.Process.Pid
	if err := WritePID(pid, flags.Listen); err != nil {
		_ = terminateProcess(pid)
		return 0, fmt.Errorf("记录 PID 文件失败: %w", err)
	}

	// 探测服务是否成功启动
	listenAddr := flags.Listen
	if listenAddr == "" {
		userCfg, _ := config.LoadUserConfig()
		if userCfg != nil {
			listenAddr = userCfg.Server.EffectiveListen()
		} else {
			listenAddr = "127.0.0.1:8999"
		}
	}
	probeHost := listenAddr
	if strings.HasPrefix(probeHost, ":") {
		probeHost = "127.0.0.1" + probeHost
	} else if strings.HasPrefix(probeHost, "0.0.0.0:") {
		probeHost = "127.0.0.1" + strings.TrimPrefix(probeHost, "0.0.0.0")
	}
	healthURL := fmt.Sprintf("http://%s/api/v1/health", probeHost)

	client := &http.Client{Timeout: 300 * time.Millisecond}
	started := false
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		// 检查子进程是否已中途崩溃
		if !isProcessAlive(pid) {
			break
		}
		resp, err := client.Get(healthURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			started = true
			break
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
	}

	if !started {
		_ = RemovePID()
		return 0, fmt.Errorf("服务进程已启动 (PID %d) 但健康检查探测失败，请查看日志: %s", pid, logPath)
	}

	return pid, nil
}

// StopDaemon 停止正在运行的后台静默服务
func StopDaemon() (int, error) {
	pid, err := ReadPID()
	if err != nil || !isProcessAlive(pid) {
		_ = RemovePID()
		return 0, errors.New("upy server 未在运行")
	}

	// 发送 SIGTERM
	if err := terminateProcess(pid); err != nil {
		return pid, fmt.Errorf("向 PID %d 发送终止信号失败: %w", pid, err)
	}

	// 轮询等待最多 5 秒进程退出
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if !isProcessAlive(pid) {
			_ = RemovePID()
			return pid, nil
		}
	}

	// 若未退出，强制 kill
	_ = killProcess(pid)
	_ = RemovePID()
	return pid, nil
}

// CheckDaemonStatus 检查后台服务运行状态
func CheckDaemonStatus(configuredListen string) (*DaemonStatus, error) {
	info, err := ReadPIDInfo()
	if err != nil || !isProcessAlive(info.PID) {
		return &DaemonStatus{Running: false}, nil
	}

	listenAddr := configuredListen
	if info.Listen != "" {
		listenAddr = info.Listen
	} else if listenAddr == "" {
		listenAddr = "127.0.0.1:8999"
	}
	probeHost := listenAddr
	if strings.HasPrefix(probeHost, ":") {
		probeHost = "127.0.0.1" + probeHost
	} else if strings.HasPrefix(probeHost, "0.0.0.0:") {
		probeHost = "127.0.0.1" + strings.TrimPrefix(probeHost, "0.0.0.0")
	}

	status := &DaemonStatus{
		Running: true,
		PID:     info.PID,
		Listen:  listenAddr,
	}

	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://%s/api/v1/health", probeHost))
	if err == nil && resp.StatusCode == http.StatusOK {
		var body struct {
			Version string `json:"version"`
			Uptime  string `json:"uptime"`
		}
		if decErr := json.NewDecoder(resp.Body).Decode(&body); decErr == nil {
			status.Version = body.Version
			status.Uptime = body.Uptime
		}
		_ = resp.Body.Close()
	} else if resp != nil {
		_ = resp.Body.Close()
	}

	return status, nil
}
