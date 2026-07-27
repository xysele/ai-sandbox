package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	"ai-sandbox-go/internal/config"
)

const sessionCookieName = "sandbox_session"
const sessionTTL = 1 * time.Hour

// sessions 存储活跃的会话令牌（令牌 -> 过期时间）。管理页会并发请求多个
// API，因此所有访问都必须持锁。
var (
	sessionsMu sync.Mutex
	sessions   = make(map[string]time.Time)
)

// UIHandler 提供 HTML 管理界面。
func UIHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if HasValidSession(r) {
			serveDashboard(w)
			return
		}

		// 未登录，显示登录表单
		serveLoginForm(w)
	}
}

// UIAuthHandler 处理登录请求。
func UIAuthHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		token := r.FormValue("token")
		if token == "" {
			http.Error(w, "Token required", http.StatusBadRequest)
			return
		}

		// 验证 token
		if subtle.ConstantTimeCompare([]byte(token), []byte(cfg.GatewayToken)) != 1 {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// 创建会话
		sessionToken, err := generateSessionToken()
		if err != nil {
			http.Error(w, "Failed to create session", http.StatusInternalServerError)
			return
		}
		sessionsMu.Lock()
		sessions[sessionToken] = time.Now().Add(sessionTTL)
		sessionsMu.Unlock()

		// 设置 cookie
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    sessionToken,
			Path:     "/",
			MaxAge:   int(sessionTTL.Seconds()),
			HttpOnly: true,
			Secure:   isHTTPSRequest(r),
			SameSite: http.SameSiteStrictMode,
		})

		http.Redirect(w, r, "/ui", http.StatusSeeOther)
	}
}

// UILogoutHandler 处理登出。
func UILogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(sessionCookieName); err == nil {
			sessionsMu.Lock()
			delete(sessions, cookie.Value)
			sessionsMu.Unlock()
		}

		http.SetCookie(w, &http.Cookie{
			Name:   sessionCookieName,
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})

		http.Redirect(w, r, "/ui", http.StatusSeeOther)
	}
}

// HasValidSession 判断请求是否携带有效的管理页会话 Cookie。API 鉴权
// 中间件也使用这个检查，使管理页登录后无需在 JavaScript 中保存原始 Token。
func HasValidSession(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	return isValidSession(cookie.Value)
}

// generateSessionToken 生成与原始 Gateway Token 无关的随机会话令牌。
func generateSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func isHTTPSRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	forwardedProto := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0])
	return strings.EqualFold(forwardedProto, "https")
}

// isValidSession 检查会话是否有效。
func isValidSession(sessionToken string) bool {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	expiry, exists := sessions[sessionToken]
	if !exists {
		return false
	}
	if time.Now().After(expiry) {
		delete(sessions, sessionToken)
		return false
	}
	return true
}

// serveLoginForm 返回登录页面 HTML。
func serveLoginForm(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(loginFormHTML))
}

// serveDashboard 返回管理界面 HTML。
func serveDashboard(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// 拼接所有 dashboard HTML 片段
	fullHTML := dashboardHTML + dashboardHTML2 + dashboardHTML3 + dashboardHTML4
	w.Write([]byte(fullHTML))
}
