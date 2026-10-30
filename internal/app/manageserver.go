package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"clip/internal/webui"
)

// manageBroadcastInterval 是网页管理轮询数据版本、推送 SSE 的间隔。
var manageBroadcastInterval = 200 * time.Millisecond

// ManageServer 提供浏览器管理界面：仅监听本机的 HTTP 服务 + SSE 数据变更推送。
type ManageServer struct {
	app  *App
	srv  *http.Server
	ln   net.Listener
	url  string
	port int
	done chan struct{}

	sseMu      sync.Mutex
	sseClients map[chan string]bool

	stopOnce sync.Once
}

// NewManageServer 绑定本机随机端口并构造网页管理（不启动）。
func NewManageServer(app *App) (*ManageServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	port := ln.Addr().(*net.TCPAddr).Port
	server := &ManageServer{
		app:        app,
		ln:         ln,
		url:        fmt.Sprintf("http://127.0.0.1:%d", port),
		port:       port,
		done:       make(chan struct{}),
		sseClients: make(map[chan string]bool),
	}
	server.srv = &http.Server{
		Handler:           server.newManageHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return server, nil
}

// URL 返回管理页面地址。
func (s *ManageServer) URL() string {
	return s.url
}

// Start 启动 HTTP 服务与数据变更广播。
func (s *ManageServer) Start() {
	go func() {
		if err := s.srv.Serve(s.ln); err != nil && err != http.ErrServerClosed {
			global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("网页管理已停止: %v", err)}
		}
	}()
	go s.broadcastLoop()

	global_log_channel <- LogEntry{Kind: KindInfo, Content: fmt.Sprintf("网页管理已启动: %s", s.url)}
}

// Stop 停止广播、断开 SSE 客户端并关闭 HTTP 服务；重复调用安全。
func (s *ManageServer) Stop() {
	s.stopOnce.Do(func() {
		close(s.done)

		s.sseMu.Lock()
		for ch := range s.sseClients {
			close(ch)
		}
		s.sseClients = make(map[chan string]bool)
		s.sseMu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.srv.Shutdown(ctx)

		global_log_channel <- LogEntry{Kind: KindInfo, Content: "网页管理已关闭"}
	})
}

// toggleManageServer 启停网页管理；开启时自动用浏览器打开管理页面。
func (a *App) toggleManageServer() {
	a.manageServerMu.RLock()
	current := a.manageServer
	a.manageServerMu.RUnlock()

	if current != nil {
		current.Stop()
		a.manageServerMu.Lock()
		a.manageServer = nil
		a.manageServerMu.Unlock()
		return
	}

	server, err := NewManageServer(a)
	if err != nil {
		global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("启动网页管理失败: %v", err)}
		return
	}
	a.manageServerMu.Lock()
	a.manageServer = server
	a.manageServerMu.Unlock()
	server.Start()

	if err := openURL(server.URL()); err != nil {
		global_log_channel <- LogEntry{Kind: KindError, Content: fmt.Sprintf("打开管理页面失败: %v", err)}
	}
}

// broadcastLoop 轮询数据版本，变化时向所有 SSE 客户端推送。
func (s *ManageServer) broadcastLoop() {
	last := s.app.dataVersion.Load()
	ticker := time.NewTicker(manageBroadcastInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			current := s.app.dataVersion.Load()
			if current == last {
				continue
			}
			last = current
			s.broadcast(fmt.Sprintf(`{"version":%d}`, current))
		}
	}
}

func (s *ManageServer) broadcast(payload string) {
	s.sseMu.Lock()
	defer s.sseMu.Unlock()

	for ch := range s.sseClients {
		select {
		case ch <- payload:
		default:
		}
	}
}

// newManageHandler 构建网页管理路由：Host 校验 + 静态资源 + API。
func (s *ManageServer) newManageHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(webui.FS())))
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/image", s.handleImage)
	mux.HandleFunc("/api/items/copy", s.handleItemCopy)
	mux.HandleFunc("/api/items/delete", s.handleItemDelete)
	mux.HandleFunc("/api/items/rename", s.handleItemRename)
	mux.HandleFunc("/api/items/add-to-group", s.handleItemAddToGroup)
	mux.HandleFunc("/api/clipboard/text", s.handleClipboardText)
	mux.HandleFunc("/api/history/clear", s.handleHistoryClear)
	mux.HandleFunc("/api/groups/create", s.handleGroupCreate)
	mux.HandleFunc("/api/groups/rename", s.handleGroupRename)
	mux.HandleFunc("/api/groups/delete", s.handleGroupDelete)
	mux.HandleFunc("/api/groups/toggle", s.handleGroupToggle)
	mux.HandleFunc("/api/config", s.handleConfig)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.validHost(r.Host) {
			writeError(w, http.StatusForbidden, "仅允许本机访问")
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// validHost 校验请求 Host 仅为本机地址，防御 DNS rebinding。
func (s *ManageServer) validHost(host string) bool {
	return host == fmt.Sprintf("127.0.0.1:%d", s.port) || host == fmt.Sprintf("localhost:%d", s.port)
}

// handleEvents 以 SSE 推送数据版本变化，驱动前端自动刷新。
func (s *ManageServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "当前环境不支持流式响应")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	ch := make(chan string, 4)

	s.sseMu.Lock()
	select {
	case <-s.done:
		s.sseMu.Unlock()
		return
	default:
	}
	s.sseClients[ch] = true
	s.sseMu.Unlock()

	defer func() {
		s.sseMu.Lock()
		delete(s.sseClients, ch)
		s.sseMu.Unlock()
	}()

	fmt.Fprintf(w, "data: {\"version\":%d}\n\n", s.app.dataVersion.Load())
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.done:
			return
		case payload, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
