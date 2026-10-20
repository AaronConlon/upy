// system 服务部署器: 负责基于 systemd 纳管 Go 二进制及原生服务的生命周期与版本发布
package deployer

import (
	"encoding/json"
	"fmt"
	"github.com/AaronConlon/upy/internal/i18n"
	"io"
	"net/http"
	"net/url"
	"os"
	userpkg "os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AaronConlon/upy/internal/cmdrun"
	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/log"
)

// SystemDeployer 系统级二进制服务部署器
type SystemDeployer struct {
	ctx         *DeployContext
	unitName    string
	unitDir     string
	user        string
	group       string
	backupUnit  []byte
	wasActive   bool
	deactivated bool
	activated   bool
}

// NewSystem 创建 system 部署器
func NewSystem(ctx *DeployContext) *SystemDeployer {
	unitName := fmt.Sprintf("upy-%s.service", Slug(ctx.ProjectName))
	unitDir := os.Getenv("UPY_SYSTEMD_DIR")
	if unitDir == "" {
		unitDir = "/etc/systemd/system"
	}
	user := os.Getenv("UPY_SERVICE_USER")
	if user == "" {
		user = Slug(ctx.ProjectName)
	}
	group := user
	if u, err := userpkg.Lookup(user); err == nil && u != nil {
		if _, err := userpkg.LookupGroup(group); err != nil {
			if g, err := userpkg.LookupGroupId(u.Gid); err == nil && g != nil {
				group = g.Name
			}
		}
	}

	// 读取服务器端配置（如果存在）
	if pcfg, err := config.ReadProjectConfig(ctx.Root); err == nil && pcfg != nil && pcfg.Service != nil {
		if strings.TrimSpace(pcfg.Service.Unit) != "" {
			unitName = strings.TrimSpace(pcfg.Service.Unit)
		}
		if strings.TrimSpace(pcfg.Service.User) != "" {
			user = strings.TrimSpace(pcfg.Service.User)
			group = user
		}
		if strings.TrimSpace(pcfg.Service.Group) != "" {
			group = strings.TrimSpace(pcfg.Service.Group)
		}
	}

	return &SystemDeployer{
		ctx:      ctx,
		unitName: unitName,
		unitDir:  unitDir,
		user:     user,
		group:    group,
	}
}

// UnitName 返回当前纳管的 systemd unit 名称
func (s *SystemDeployer) UnitName() string {
	return s.unitName
}

// UnitPath 返回 systemd unit 文件路径
func (s *SystemDeployer) UnitPath() string {
	return filepath.Join(s.unitDir, s.unitName)
}

func (s *SystemDeployer) parseCommand() ([]string, error) {
	if s.ctx.Manifest.System == nil {
		return nil, i18n.Error("缺少 system 配置块")
	}
	return config.ParseCommand(s.ctx.Manifest.System.Command)
}

func (s *SystemDeployer) executablePath(args []string) (string, error) {
	if len(args) == 0 {
		return "", i18n.Error("启动命令为空")
	}
	raw := args[0]
	var execPath string
	if filepath.IsAbs(raw) {
		execPath = raw
	} else {
		execPath = filepath.Clean(filepath.Join(s.ctx.ReleaseDir, strings.TrimPrefix(raw, "./")))
	}
	info, err := os.Stat(execPath)
	if err != nil {
		return "", fmt.Errorf(i18n.T("找不到可执行文件: %s"), execPath)
	}
	if info.IsDir() {
		return "", fmt.Errorf(i18n.T("目标可执行路径是一个目录: %s"), execPath)
	}
	_ = os.Chmod(execPath, 0o755)
	return execPath, nil
}

func (s *SystemDeployer) workingDirectory() string {
	if s.ctx.Manifest.System != nil && strings.TrimSpace(s.ctx.Manifest.System.WorkingDirectory) != "" {
		wd := strings.TrimSpace(s.ctx.Manifest.System.WorkingDirectory)
		if filepath.IsAbs(wd) {
			return wd
		}
		return filepath.Clean(filepath.Join(s.ctx.ReleaseDir, wd))
	}
	return s.ctx.ReleaseDir
}

func (s *SystemDeployer) generateUnitContent(execStart string) string {
	userLines := fmt.Sprintf("User=%s\nGroup=%s\n", s.user, s.group)

	envFile := SharedServiceEnvFile(s.ctx.Root)
	if envFile == "" {
		envFile = SharedServiceEnvPath(s.ctx.Root)
	}

	absRoot, _ := filepath.Abs(s.ctx.Root)

	return fmt.Sprintf(`# Managed by upy - DO NOT EDIT MANUALLY
# X-Upy-Project: %s
# X-Upy-Root: %s

[Unit]
Description=%s managed by upy
After=network.target
Wants=network-online.target

[Service]
Type=exec
%sWorkingDirectory=%s
ExecStart=%s
EnvironmentFile=-%s
Restart=on-failure
RestartSec=3s
TimeoutStopSec=30s
KillSignal=SIGTERM
StandardOutput=journal
StandardError=journal
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
`, s.ctx.ProjectName, absRoot, s.ctx.ProjectName, userLines, s.workingDirectory(), execStart, envFile)
}

func (s *SystemDeployer) verifyUnitOwnership(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	text := string(content)
	if !strings.Contains(text, "managed by upy") && !strings.Contains(text, "X-Upy-Root") {
		return fmt.Errorf(i18n.T("systemd 服务 %s 已存在但非 upy 纳管（缺少 upy 标记），拒绝接管或覆盖"), path)
	}
	absRoot, _ := filepath.Abs(s.ctx.Root)
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# X-Upy-Root:") {
			rootInUnit := strings.TrimSpace(strings.TrimPrefix(trimmed, "# X-Upy-Root:"))
			if rootInUnit != "" && filepath.Clean(rootInUnit) != filepath.Clean(absRoot) {
				return fmt.Errorf(i18n.T("systemd 服务 %s 已被路径 %s 的项目纳管，当前项目 (%s) 拒绝冲突覆盖"), s.unitName, rootInUnit, absRoot)
			}
		}
	}
	return nil
}

// Prepare 校验可执行文件、运行账号、目标架构与基础环境
func (s *SystemDeployer) Prepare() error {
	log.Step(i18n.T("正在校验系统服务配置与可执行文件..."))
	args, err := s.parseCommand()
	if err != nil {
		return err
	}
	if _, err := s.executablePath(args); err != nil {
		return err
	}

	// SYS-10: 验证运行用户与组是否存在，严禁静默降级为 root
	u, err := userpkg.Lookup(s.user)
	if err != nil {
		return fmt.Errorf(i18n.T("系统服务运行用户 %q 不存在，拒绝以 root 静默运行: %w"), s.user, err)
	}
	if s.group != "" {
		if _, err := userpkg.LookupGroup(s.group); err != nil {
			if u != nil {
				if g, gErr := userpkg.LookupGroupId(u.Gid); gErr == nil && (g.Name == s.group || s.group == s.user) {
					s.group = g.Name
				} else {
					return fmt.Errorf(i18n.T("系统服务运行用户组 %q 不存在: %w"), s.group, err)
				}
			} else {
				return fmt.Errorf(i18n.T("系统服务运行用户组 %q 不存在: %w"), s.group, err)
			}
		}
	}

	// SYS-001: 权限预检 - 确保 unitDir 存在且可写 (在停止旧服务前阻断缺权异常)
	if err := os.MkdirAll(s.unitDir, 0o755); err != nil {
		return fmt.Errorf(i18n.T("无法创建 systemd 目录 %s: %w"), s.unitDir, err)
	}
	testFile := filepath.Join(s.unitDir, fmt.Sprintf(".upy-perm-test-%d", time.Now().UnixNano()))
	if err := os.WriteFile(testFile, []byte(""), 0o644); err != nil {
		return fmt.Errorf(i18n.T("systemd 目录 %s 无写入权限（请配置 sudo 权限或以 root 运行）: %w"), s.unitDir, err)
	}
	_ = os.Remove(testFile)

	// SYS-006: 校验既有 unit 归属，杜绝覆盖外部服务或同名项目冲突
	if err := s.verifyUnitOwnership(s.UnitPath()); err != nil {
		return err
	}

	// 确保持久化数据目录 shared/data 存在
	dataDir := filepath.Join(s.ctx.Root, "shared", "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf(i18n.T("创建持久化数据目录失败: %v"), err)
	}
	log.Ok(i18n.T("系统服务配置校验通过"))
	return nil
}

// Deactivate 备份旧 unit，停掉旧版本服务
func (s *SystemDeployer) Deactivate() error {
	path := s.UnitPath()
	bakPath := filepath.Join(s.ctx.Root, ".deploy.unit.bak")

	if content, err := os.ReadFile(path); err == nil {
		if err := s.verifyUnitOwnership(path); err != nil {
			return err
		}
		s.backupUnit = content
		// SYS-003: 写入磁盘备份快照
		if err := os.WriteFile(bakPath, content, 0o644); err != nil {
			return fmt.Errorf(i18n.T("持久化旧 unit 备份快照失败: %w"), err)
		}

		// 查询服务当前是否处于活跃运行状态
		if err := cmdrun.Run("systemctl", []string{"is-active", "--quiet", s.unitName}, cmdrun.Options{}); err == nil {
			s.wasActive = true
			log.Step(fmt.Sprintf(i18n.T("正在停止上一版本系统服务 (%s)..."), s.unitName))
			if err := cmdrun.Run("systemctl", []string{"stop", s.unitName}, cmdrun.Options{}); err != nil {
				return fmt.Errorf(i18n.T("停止旧版本服务失败: %v"), err)
			}
			log.Ok(i18n.T("上一版本服务已停止"))
		}
	} else {
		// 首次部署，清理可能残留的旧备份
		_ = os.Remove(bakPath)
	}
	s.deactivated = true
	return nil
}

// Activate 写入新 unit 文件并启动服务
func (s *SystemDeployer) Activate() error {
	args, err := s.parseCommand()
	if err != nil {
		return err
	}
	execPath, err := s.executablePath(args)
	if err != nil {
		return err
	}

	// SYS-008: 规范转义 ExecStart 参数
	var execParts []string
	if strings.ContainsAny(execPath, " \t") {
		execParts = append(execParts, `"`+execPath+`"`)
	} else {
		execParts = append(execParts, execPath)
	}
	for _, arg := range args[1:] {
		escaped, err := config.EscapeSystemdArg(arg)
		if err != nil {
			return fmt.Errorf(i18n.T("参数转义失败: %w"), err)
		}
		execParts = append(execParts, escaped)
	}
	execStart := strings.Join(execParts, " ")

	content := s.generateUnitContent(execStart)

	// SYS-003: 同目录临时文件 + 原子 rename 写入 unit
	tmpUnit := filepath.Join(s.unitDir, fmt.Sprintf(".upy-%s.tmp.%d", s.unitName, time.Now().UnixNano()))
	if err := os.WriteFile(tmpUnit, []byte(content), 0o644); err != nil {
		return fmt.Errorf(i18n.T("写入 unit 临时文件失败: %v"), err)
	}
	if err := os.Rename(tmpUnit, s.UnitPath()); err != nil {
		_ = os.Remove(tmpUnit)
		return fmt.Errorf(i18n.T("原子激活 unit 文件失败: %v"), err)
	}

	log.Step(i18n.T("正在加载 systemd 配置并启动服务..."))
	if err := cmdrun.Run("systemctl", []string{"daemon-reload"}, cmdrun.Options{}); err != nil {
		return fmt.Errorf(i18n.T("systemctl daemon-reload 失败: %v"), err)
	}
	if err := cmdrun.Run("systemctl", []string{"start", s.unitName}, cmdrun.Options{}); err != nil {
		return fmt.Errorf(i18n.T("systemctl start %s 失败: %v"), s.unitName, err)
	}
	s.activated = true
	log.Ok(fmt.Sprintf(i18n.T("系统服务已启动 (%s)"), s.unitName))
	return nil
}

// HealthCheck 执行健康检查
func (s *SystemDeployer) HealthCheck() error {
	// SYS-007: 先确认目标 unit 处于 active 状态
	if err := cmdrun.Run("systemctl", []string{"is-active", "--quiet", s.unitName}, cmdrun.Options{}); err != nil {
		return i18n.Error("服务未能成功启动或已崩溃退出 (systemctl is-active 检查失败)")
	}

	// 获取 MainPID
	var mainPID int
	if out, err := cmdrun.Output("systemctl", []string{"show", s.unitName, "-p", "MainPID", "--value"}, cmdrun.Options{}); err == nil {
		mainPID, _ = strconv.Atoi(strings.TrimSpace(out))
	}

	hc := s.ctx.Manifest.Healthcheck
	if hc == nil && s.ctx.Manifest.System != nil {
		hc = s.ctx.Manifest.System.Healthcheck
	}

	if hc != nil && strings.TrimSpace(hc.URL) != "" {
		timeout := HealthcheckTimeoutSec(s.ctx.Manifest)
		log.Step(fmt.Sprintf(i18n.T("正在健康检查 %s (超时 %ds)..."), hc.URL, timeout))

		port, isLocal := extractPortFromURL(hc.URL)

		deadline := time.Now().Add(time.Duration(timeout) * time.Second)
		client := &http.Client{
			Timeout: 3 * time.Second,
		}
		var lastErr string
		passed := false
		for time.Now().Before(deadline) {
			// 在轮询间隙监控进程是否依然存活，避免进程崩溃后空等超时
			if err := cmdrun.Run("systemctl", []string{"is-active", "--quiet", s.unitName}, cmdrun.Options{}); err != nil {
				return i18n.Error("服务在等待健康检查期间崩溃退出 (systemctl is-active 失败)")
			}

			// 如果是本地探针且存在 /proc/net/tcp，验证 MainPID 是否已就绪监听该端口 (SYS-21)
			if isLocal && isProcAvailable() && mainPID > 0 {
				if !isPortListenedByPID(mainPID, port) {
					lastErr = fmt.Sprintf(i18n.T("端口 %d 尚未由服务进程 (PID %d) 监听"), port, mainPID)
					time.Sleep(500 * time.Millisecond)
					continue
				}
			}

			req, err := http.NewRequest("GET", hc.URL, nil)
			if err == nil {
				resp, err := client.Do(req)
				if err == nil {
					body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
					resp.Body.Close()

					if resp.StatusCode >= 200 && resp.StatusCode < 300 {
						// 校验响应内容中的身份 (防假探针 200 欺骗 - SYS-21)
						if isLocal && len(body) > 0 {
							var respData map[string]any
							if err := json.Unmarshal(body, &respData); err == nil && respData != nil {
								if v, ok := respData["version"].(string); ok && v != "" && v != s.ctx.Version {
									lastErr = fmt.Sprintf(i18n.T("探针响应版本 %q 与部署版本 %q 不符"), v, s.ctx.Version)
									time.Sleep(500 * time.Millisecond)
									continue
								}
								if p, ok := respData["pid"].(float64); ok && mainPID > 0 && int(p) != mainPID {
									lastErr = fmt.Sprintf(i18n.T("探针响应 PID %d 与服务 MainPID %d 不符"), int(p), mainPID)
									time.Sleep(500 * time.Millisecond)
									continue
								}
							}
						}
						passed = true
						break
					}
					lastErr = fmt.Sprintf("HTTP %d", resp.StatusCode)
				} else {
					lastErr = err.Error()
				}
			} else {
				lastErr = err.Error()
			}
			time.Sleep(500 * time.Millisecond)
		}
		if !passed {
			return fmt.Errorf(i18n.T("健康检查超时（%ds）: %s"), timeout, log.Redact(lastErr))
		}
		log.Ok(i18n.T("健康检查通过"))
	} else {
		log.Info(i18n.T("未配置 HTTP 健康检查，系统服务处于运行状态 (active)"))
	}

	// SYS-009: 首次成功发布后，启用开机自启 (enable)
	if s.backupUnit == nil {
		_ = cmdrun.Run("systemctl", []string{"enable", s.unitName}, cmdrun.Options{AllowNonZero: true})
	}
	// 成功提交，清理临时 unit 备份
	bakPath := filepath.Join(s.ctx.Root, ".deploy.unit.bak")
	_ = os.Remove(bakPath)

	return nil
}

// Rollback 回滚到上一版本 unit
func (s *SystemDeployer) Rollback() error {
	// SYS-07, SYS-11, SYS-12: 如果从未执行 Deactivate 或 Activate（例如在 Prepare 阶段失败），严禁触碰或停止服务，更不可删除 unit
	if !s.deactivated && !s.activated {
		return nil
	}

	log.Warn(fmt.Sprintf(i18n.T("正在回滚系统服务 %s..."), s.unitName))

	// SYS-002: 无论新服务是否存活，先显式停止新服务，避免新进程占有端口或死循环重启
	_ = cmdrun.Run("systemctl", []string{"stop", s.unitName}, cmdrun.Options{AllowNonZero: true})
	// 取消失败计数与重启频次限制
	_ = cmdrun.Run("systemctl", []string{"reset-failed", s.unitName}, cmdrun.Options{AllowNonZero: true})

	// 获取备份内容 (优先内存，其次磁盘备份)
	backupData := s.backupUnit
	bakPath := filepath.Join(s.ctx.Root, ".deploy.unit.bak")
	if len(backupData) == 0 {
		if data, err := os.ReadFile(bakPath); err == nil {
			backupData = data
		}
	}

	if len(backupData) > 0 {
		// 原子还原旧 unit
		tmpUnit := filepath.Join(s.unitDir, fmt.Sprintf(".upy-%s.rollback.%d", s.unitName, time.Now().UnixNano()))
		if err := os.WriteFile(tmpUnit, backupData, 0o644); err != nil {
			return fmt.Errorf(i18n.T("写入旧 unit 还原文件失败: %w"), err)
		}
		if err := os.Rename(tmpUnit, s.UnitPath()); err != nil {
			_ = os.Remove(tmpUnit)
			return fmt.Errorf(i18n.T("恢复旧 unit 失败: %w"), err)
		}
		_ = cmdrun.Run("systemctl", []string{"daemon-reload"}, cmdrun.Options{})

		// 仅在旧服务原本处于活跃状态且被本次停掉时，重新启动旧服务
		if s.wasActive {
			log.Step(fmt.Sprintf(i18n.T("正在重新启动上一版本系统服务 (%s)..."), s.ctx.PreviousVersion))
			if err := cmdrun.Run("systemctl", []string{"start", s.unitName}, cmdrun.Options{}); err != nil {
				return fmt.Errorf(i18n.T("恢复启动旧版本服务失败: %v"), err)
			}
			// 有界健康状态校验
			if err := cmdrun.Run("systemctl", []string{"is-active", "--quiet", s.unitName}, cmdrun.Options{}); err != nil {
				return fmt.Errorf(i18n.T("上一版本服务未能恢复 active 状态: %v"), err)
			}
			// SYS-15: 验证回滚后旧服务实际 HTTP 存活性
			hc := s.ctx.Manifest.Healthcheck
			if hc == nil && s.ctx.Manifest.System != nil {
				hc = s.ctx.Manifest.System.Healthcheck
			}
			if hc != nil && strings.TrimSpace(hc.URL) != "" {
				if err := PollHealth(hc.URL, 5); err != nil {
					return fmt.Errorf(i18n.T("上一版本服务未能恢复健康状态 (HTTP 探测失败): %w"), err)
				}
			}
			log.Ok(fmt.Sprintf(i18n.T("已成功恢复上一版本系统服务 (%s)"), s.ctx.PreviousVersion))
		} else {
			log.Info(i18n.T("上一版本服务在发布前处于停止状态，回滚后保持停止"))
		}
	} else {
		// 首次部署失败，清理新建的 unit 文件并禁用
		_ = cmdrun.Run("systemctl", []string{"disable", s.unitName}, cmdrun.Options{AllowNonZero: true})
		_ = os.Remove(s.UnitPath())
		_ = cmdrun.Run("systemctl", []string{"daemon-reload"}, cmdrun.Options{})
		_ = cmdrun.Run("systemctl", []string{"reset-failed", s.unitName}, cmdrun.Options{AllowNonZero: true})
		_ = os.Remove(bakPath)
		log.Ok(i18n.T("首次部署失败，已清理新生成的 systemd unit"))
	}
	return nil
}

func isProcAvailable() bool {
	_, err := os.Stat("/proc/net/tcp")
	return err == nil
}

func extractPortFromURL(rawURL string) (int, bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return 0, false
	}
	host := parsed.Hostname()
	if !isLocalHost(host) {
		return 0, false
	}
	portStr := parsed.Port()
	if portStr == "" {
		if parsed.Scheme == "https" {
			return 443, true
		}
		return 80, true
	}
	p, err := strconv.Atoi(portStr)
	if err != nil || p <= 0 {
		return 0, false
	}
	return p, true
}

func isLocalHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	return h == "127.0.0.1" || h == "localhost" || h == "::1" || h == "[::1]" || h == "0.0.0.0" || h == ""
}

// isPortListenedByPID 检查指定 pid (或其子进程) 是否在 Linux 上监听了目标端口
func isPortListenedByPID(pid int, port int) bool {
	if pid <= 0 || port <= 0 {
		return false
	}

	pids := []int{pid}
	if childrenData, err := os.ReadFile(fmt.Sprintf("/proc/%d/task/%d/children", pid, pid)); err == nil {
		for _, field := range strings.Fields(string(childrenData)) {
			if cpid, err := strconv.Atoi(field); err == nil && cpid > 0 {
				pids = append(pids, cpid)
			}
		}
	}

	socketInodes := make(map[uint64]bool)
	for _, p := range pids {
		fdDir := fmt.Sprintf("/proc/%d/fd", p)
		entries, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			link, err := os.Readlink(filepath.Join(fdDir, e.Name()))
			if err != nil {
				continue
			}
			if strings.HasPrefix(link, "socket:[") && strings.HasSuffix(link, "]") {
				inodeStr := strings.TrimPrefix(strings.TrimSuffix(link, "]"), "socket:[")
				if inode, err := strconv.ParseUint(inodeStr, 10, 64); err == nil {
					socketInodes[inode] = true
				}
			}
		}
	}

	if len(socketInodes) == 0 {
		return false
	}

	checkNetFile := func(path string) bool {
		data, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines[1:] {
			fields := strings.Fields(line)
			if len(fields) < 10 {
				continue
			}
			localAddr := fields[1]
			st := fields[3]
			inodeStr := fields[9]
			if st != "0A" { // 0A 代表 TCP_LISTEN
				continue
			}
			parts := strings.Split(localAddr, ":")
			if len(parts) != 2 {
				continue
			}
			portHex, err := strconv.ParseInt(parts[1], 16, 32)
			if err != nil || int(portHex) != port {
				continue
			}
			inode, err := strconv.ParseUint(inodeStr, 10, 64)
			if err != nil {
				continue
			}
			if socketInodes[inode] {
				return true
			}
		}
		return false
	}

	return checkNetFile("/proc/net/tcp") || checkNetFile("/proc/net/tcp6")
}
