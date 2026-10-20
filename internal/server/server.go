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
		Handler:           s.loggingMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	s.mu.Unlock()

	s.logf("服务就绪，监听地址: %s", listenAddr)
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
	s.logf("接收到停止信号，正在优雅关闭 HTTP 服务...")
	err := srv.Shutdown(ctx)
	s.logf("HTTP 服务已安全关闭")
	return err
}

func (s *Server) logf(format string, args ...any) {
	if s.serverOut == nil {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(s.serverOut, "[%s] [SERVER] %s\n", ts, msg)
}

type responseWriterRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriterRecorder) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriterRecorder) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseWriterRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rec, r)
		duration := time.Since(start).Round(time.Millisecond)

		clientIP := r.RemoteAddr
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			clientIP = strings.TrimSpace(strings.Split(xff, ",")[0])
		}

		s.logf("%s - %s %s -> %d (%s)", clientIP, r.Method, r.URL.Path, rec.statusCode, duration)
	})
}
