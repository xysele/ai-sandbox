package server

import (
	"crypto/subtle"
	"log"
	"net/http"
	"time"

	"ai-sandbox-go/internal/config"
	"ai-sandbox-go/internal/handlers"
)

// Server 持有配置与路由表。
type Server struct {
	config *config.Config
	mux    *http.ServeMux
}

func New(cfg *config.Config) *Server {
	s := &Server{
		config: cfg,
		mux:    http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// Start 启动 HTTP 服务。读写超时设得比最长命令超时更宽，
// 否则跑满 300 秒的 /task 会被 http 层提前掐断。
func (s *Server) Start() error {
	addr := "0.0.0.0:" + s.config.Port
	log.Printf("[server] listening on %s", addr)

	srv := &http.Server{
		Addr:              addr,
		Handler:           s.authMiddleware(s.mux),
		ReadHeaderTimeout: 15 * time.Second,
		WriteTimeout:      10 * time.Minute,
	}
	return srv.ListenAndServe()
}

// registerRoutes 挂载全部端点。
//
// 设计原则：凡是一行 shell 就能做到的事（git、进程管理、DNS、压缩解压、
// 代码格式化）都不单独占端点，交给 /exec。端点只保留 shell 做不到或做起来
// 很别扭的能力：二进制传输、GUI 控制、跨请求的任务状态、需要解析响应头的
// HTTP 调用、批量操作、浏览器自动化。
func (s *Server) registerRoutes() {
	// 公开端点，无需鉴权
	s.mux.HandleFunc("/", handlers.Root)
	s.mux.HandleFunc("/health", handlers.Health(s.config))

	// UI 管理界面（需要 cookie 认证，但不是 token header）
	s.mux.HandleFunc("/ui", handlers.UIHandler(s.config))
	s.mux.HandleFunc("/ui/auth", handlers.UIAuthHandler(s.config))
	s.mux.HandleFunc("/ui/logout", handlers.UILogoutHandler())

	// 命令与代码执行
	s.mux.HandleFunc("/exec", handlers.ExecShell)
	s.mux.HandleFunc("/run_code", handlers.RunCode)

	// 批量操作（新增）
	s.mux.HandleFunc("/batch", handlers.Batch(s.mux))

	// 异步任务：超过一次 HTTP 请求生命周期的长命令走这里
	s.mux.HandleFunc("/task/create", handlers.TaskCreate)
	s.mux.HandleFunc("/task/status", handlers.TaskStatus)
	s.mux.HandleFunc("/task/cancel", handlers.TaskCancel)
	s.mux.HandleFunc("/task/list", handlers.TaskList)

	// 文件系统
	s.mux.HandleFunc("/read_file", handlers.ReadFile)
	s.mux.HandleFunc("/write_file", handlers.WriteFile)
	s.mux.HandleFunc("/write_files", handlers.WriteFiles) // 批量写入（新增）
	s.mux.HandleFunc("/delete_file", handlers.DeleteFile)
	s.mux.HandleFunc("/list_dir", handlers.ListDir)
	s.mux.HandleFunc("/search_files", handlers.SearchFiles)
	s.mux.HandleFunc("/upload", handlers.Upload)
	s.mux.HandleFunc("/download", handlers.Download)

	// GUI 自动化
	s.mux.HandleFunc("/screenshot", handlers.Screenshot(s.config))
	s.mux.HandleFunc("/click", handlers.Click(s.config))
	s.mux.HandleFunc("/type_text", handlers.TypeText(s.config))
	s.mux.HandleFunc("/press_key", handlers.PressKey(s.config))
	s.mux.HandleFunc("/move_mouse", handlers.MoveMouse(s.config))
	s.mux.HandleFunc("/gui_status", handlers.GuiStatus(s.config))

	// 浏览器自动化（新增）
	s.mux.HandleFunc("/browser/status", handlers.BrowserStatus)
	s.mux.HandleFunc("/browser/screenshot", handlers.BrowserScreenshot)
	s.mux.HandleFunc("/browser/click", handlers.BrowserClick)
	s.mux.HandleFunc("/browser/type", handlers.BrowserType)
	s.mux.HandleFunc("/browser/evaluate", handlers.BrowserEvaluate)
	s.mux.HandleFunc("/browser/wait_for", handlers.BrowserWaitFor)
	s.mux.HandleFunc("/browser/navigate", handlers.BrowserNavigate)

	// 环境信息与外部 HTTP
	s.mux.HandleFunc("/system/info", handlers.SystemInfo)
	s.mux.HandleFunc("/http_fetch", handlers.HTTPFetch)
}

// authMiddleware 校验 X-Gateway-Token。
//
// 用自定义头而不是标准 Authorization，是因为魔搭创空间的反向代理会剥掉
// Authorization 头，token 传不进来。
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	publicPaths := map[string]bool{
		"/":            true,
		"/health":      true,
		"/favicon.ico": true,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if publicPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		if !s.checkAuth(r.Header.Get("X-Gateway-Token")) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"success":false,"error":"invalid or missing X-Gateway-Token"}` + "\n"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) checkAuth(token string) bool {
	if token == "" {
		return false
	}
	// 定长比较，避免通过响应时间差逐字节猜出 token。
	return subtle.ConstantTimeCompare([]byte(token), []byte(s.config.GatewayToken)) == 1
}
