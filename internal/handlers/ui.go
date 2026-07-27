package handlers

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"time"

	"ai-sandbox-go/internal/config"
)

const sessionCookieName = "sandbox_session"
const sessionTTL = 1 * time.Hour

// sessions 存储活跃的会话令牌（令牌 -> 过期时间）
var sessions = make(map[string]time.Time)

// UIHandler 提供 HTML 管理界面。
func UIHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 检查是否已登录
		if cookie, err := r.Cookie(sessionCookieName); err == nil {
			if isValidSession(cookie.Value) {
				// 已登录，显示主界面
				serveDashboard(w)
				return
			}
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
		sessionToken := generateSessionToken(token)
		sessions[sessionToken] = time.Now().Add(sessionTTL)

		// 设置 cookie
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    sessionToken,
			Path:     "/",
			MaxAge:   int(sessionTTL.Seconds()),
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})

		http.Redirect(w, r, "/ui", http.StatusSeeOther)
	}
}

// UILogoutHandler 处理登出。
func UILogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(sessionCookieName); err == nil {
			delete(sessions, cookie.Value)
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

// generateSessionToken 根据用户 token 生成会话令牌（不存储原始 token）。
func generateSessionToken(userToken string) string {
	hash := sha256.Sum256([]byte(userToken + time.Now().String()))
	return hex.EncodeToString(hash[:])
}

// isValidSession 检查会话是否有效。
func isValidSession(sessionToken string) bool {
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
