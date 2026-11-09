package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// handleConsoleLogStream 处理 /logs/{id}/stream 的 SSE 长连接请求
func (s *Server) handleConsoleLogStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validLogID(id) {
		http.NotFound(w, r)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	d, path, err := s.consoleLogDetail(id)
	if err != nil {
		if os.IsNotExist(err) && !d.Live {
			http.NotFound(w, r)
			return
		}
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// 解析客户端传入的起始偏移量
	lastEventID := strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	if lastEventID == "" {
		lastEventID = strings.TrimSpace(r.URL.Query().Get("last_event_id"))
	}

	var currentOffset int64 = -1
	if lastEventID != "" {
		if parsed, perr := strconv.ParseInt(lastEventID, 10, 64); perr == nil && parsed >= 0 {
			currentOffset = parsed
		}
	}

	// 等待文件创建 (针对刚创建排队的任务，文件可能稍后生成)
	var f *os.File
	for i := 0; i < 20; i++ {
		if path != "" {
			if openF, openErr := os.Open(path); openErr == nil {
				f = openF
				break
			}
		}
		if !d.Live {
			break
		}
		select {
		case <-r.Context().Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
		if path == "" {
			d, path, _ = s.consoleLogDetail(id)
		}
	}

	if f == nil && path != "" {
		if openF, openErr := os.Open(path); openErr == nil {
			f = openF
		}
	}

	if f != nil {
		defer f.Close()
	}

	// 初始状态与追赶读取
	if f != nil {
		st, statErr := f.Stat()
		if statErr == nil {
			fileSize := st.Size()
			startOffset := int64(0)
			skipFirstLine := false

			if currentOffset < 0 {
				if fileSize > maxConsoleLogBytes {
					startOffset = fileSize - maxConsoleLogBytes
					skipFirstLine = true
				}
			} else {
				if currentOffset <= fileSize {
					startOffset = currentOffset
				}
			}

			if _, err := f.Seek(startOffset, io.SeekStart); err == nil {
				data, _ := io.ReadAll(io.LimitReader(f, maxConsoleLogBytes))
				if skipFirstLine {
					if idx := strings.IndexByte(string(data), '\n'); idx >= 0 {
						data = data[idx+1:]
					}
				}
				currentOffset = fileSize
				if len(data) > 0 {
					text := strings.ToValidUTF8(string(data), "")
					htmlChunk := RenderAnsiToHTML(text)
					_ = sendSSEEvent(w, flusher, "log", strconv.FormatInt(currentOffset, 10), map[string]any{
						"html":   htmlChunk,
						"offset": currentOffset,
					})
				}
			}
		}
	}

	// 若任务已非 active，推送最终状态与结束事件后返回
	if !d.Live {
		sendSSEStatusAndEnd(w, flusher, d)
		return
	}

	// 活跃任务或实时系统日志流：持续监听文件增量
	buf := make([]byte, 32*1024)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	pingTicker := time.NewTicker(15 * time.Second)
	defer pingTicker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return

		case <-pingTicker.C:
			// 发送 SSE 注释行作为心跳保活
			_, _ = fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()

		case <-ticker.C:
			// 若尚未打开文件，尝试重新打开
			if f == nil && path != "" {
				if openF, openErr := os.Open(path); openErr == nil {
					f = openF
					defer f.Close()
					if currentOffset > 0 {
						_, _ = f.Seek(currentOffset, io.SeekStart)
					}
				}
			}

			// 读取增量
			if f != nil {
				st, statErr := f.Stat()
				if statErr == nil && currentOffset > st.Size() {
					// 文件被清空/截断
					currentOffset = 0
					_, _ = f.Seek(0, io.SeekStart)
				}

				for {
					n, rerr := f.Read(buf)
					if n > 0 {
						currentOffset += int64(n)
						text := strings.ToValidUTF8(string(buf[:n]), "")
						htmlChunk := RenderAnsiToHTML(text)
						_ = sendSSEEvent(w, flusher, "log", strconv.FormatInt(currentOffset, 10), map[string]any{
							"html":   htmlChunk,
							"offset": currentOffset,
						})
					}
					if n == 0 || rerr != nil {
						break
					}
				}
			}

			// 检查任务生命周期是否结束
			if id != "server" && id != "upy" {
				if task, ok := s.taskMgr.Get(id); ok {
					if task.Status != StatusRunning && task.Status != StatusQueued {
						// 任务已完成，最后一次完整排空剩余未读内容
						if f != nil {
							for {
								n, _ := f.Read(buf)
								if n <= 0 {
									break
								}
								currentOffset += int64(n)
								text := strings.ToValidUTF8(string(buf[:n]), "")
								htmlChunk := RenderAnsiToHTML(text)
								_ = sendSSEEvent(w, flusher, "log", strconv.FormatInt(currentOffset, 10), map[string]any{
									"html":   htmlChunk,
									"offset": currentOffset,
								})
							}
						}

						d.Status = string(task.Status)
						d.ExitCode = task.ExitCode
						d.Duration = task.Duration
						d.Live = false
						sendSSEStatusAndEnd(w, flusher, d)
						return
					}
				}
			}
		}
	}
}

func sendSSEEvent(w io.Writer, flusher http.Flusher, event, id string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if event != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
			return err
		}
	}
	if id != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", id); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func sendSSEStatusAndEnd(w io.Writer, flusher http.Flusher, d LogDetailView) {
	_ = sendSSEEvent(w, flusher, "status", "", map[string]any{
		"status":       d.Status,
		"status_class": statusClass(d.Status),
		"status_label": statusLabel(d.Status),
		"status_html":  string(consoleIcon(statusIcon(d.Status))) + template.HTMLEscapeString(statusLabel(d.Status)),
		"duration":     d.Duration,
		"exit_label":   d.ExitLabel(),
		"live":         false,
	})
	_ = sendSSEEvent(w, flusher, "end", "", map[string]any{})
}
