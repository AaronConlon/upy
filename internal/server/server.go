package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/log"
)

type Server struct {
	mu         sync.Mutex
	cfg        config.ServerConfig
	taskMgr    *TaskManager
	worker     *Worker
	httpServer *http.Server
	listener   net.Listener
	startedAt  time.Time
	serverOut  io.Writer
	totpMgr    *TOTPManager
	sessions   *SessionStore
}

func NewServer(cfg config.ServerConfig, serverOut io.Writer) *Server {
	if serverOut == nil {
		serverOut = os.Stderr
	}
	taskMgr := NewTaskManager()
	worker := NewWorker(taskMgr, serverOut, cfg.EffectiveMaxLogHistory())

	totpMgr, _ := NewTOTPManager("", cfg.TOTPSecret)
	sessions := NewSessionStore()

	s := &Server{
		cfg:       cfg,
		taskMgr:   taskMgr,
		worker:    worker,
		startedAt: time.Now(),
		serverOut: serverOut,
		totpMgr:   totpMgr,
		sessions:  sessions,
	}
	return s
}

// SetCustomBinary 允许在测试中注入 mock 执行二进制
func (s *Server) SetCustomBinary(path string) {
	s.worker.customBinary = path
}

// SetTOTPManager 允许在测试中注入自定义的 TOTPManager
func (s *Server) SetTOTPManager(tm *TOTPManager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totpMgr = tm
}

// Start 启动 HTTP 服务 (阻塞直到收到 Shutdown 或错误)
func (s *Server) Start() error {
	listenAddr := s.cfg.EffectiveListen()

	// 安全告警: 如果监听公网 (0.0.0.0 或非 127.0.0.1) 且没有配置 token，必须给出安全警告
	if strings.TrimSpace(s.cfg.Token) == "" {
		if strings.HasPrefix(listenAddr, "0.0.0.0") || (!strings.HasPrefix(listenAddr, "127.0.0.1") && !strings.HasPrefix(listenAddr, "localhost")) {
			log.Warn("安全警告: upy server 正在监听开放接口但未配置 server.token 密钥！")
			log.Warn("建议在 ~/.upy/config.yaml 中配置 server.token，避免受到未授权探测")
		}
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("绑定监听地址 %s 失败: %w", listenAddr, err)
	}

	s.mu.Lock()
	s.listener = ln
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.httpServer = &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	s.mu.Unlock()

	return s.httpServer.Serve(ln)
}

// Addr 返回当前实际监听地址
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.cfg.EffectiveListen()
}

// Shutdown 优雅停止 HTTP 服务
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	srv := s.httpServer
	s.mu.Unlock()

	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}
