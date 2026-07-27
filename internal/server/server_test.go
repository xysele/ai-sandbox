package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"ai-sandbox-go/internal/config"
)

const testGatewayToken = "test-gateway-token"

func testServer() (*Server, http.Handler) {
	cfg := &config.Config{
		Port:         "7860",
		Display:      ":99",
		GatewayToken: testGatewayToken,
	}
	srv := New(cfg)
	return srv, srv.authMiddleware(srv.mux)
}

func TestUIAuthenticationFlow(t *testing.T) {
	_, handler := testServer()

	loginPage := httptest.NewRecorder()
	handler.ServeHTTP(loginPage, httptest.NewRequest(http.MethodGet, "/ui", nil))
	if loginPage.Code != http.StatusOK {
		t.Fatalf("GET /ui status = %d, want %d", loginPage.Code, http.StatusOK)
	}
	if !strings.Contains(loginPage.Body.String(), `action="/ui/auth"`) {
		t.Fatal("GET /ui did not return the login form")
	}

	form := url.Values{"token": {testGatewayToken}}
	loginRequest := httptest.NewRequest(
		http.MethodPost,
		"/ui/auth",
		strings.NewReader(form.Encode()),
	)
	loginRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRequest.Header.Set("X-Forwarded-Proto", "https")
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusSeeOther {
		t.Fatalf("POST /ui/auth status = %d, want %d", loginResponse.Code, http.StatusSeeOther)
	}

	var sessionCookie *http.Cookie
	for _, cookie := range loginResponse.Result().Cookies() {
		if cookie.Name == "sandbox_session" {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatal("POST /ui/auth did not set a session cookie")
	}
	if !sessionCookie.HttpOnly || !sessionCookie.Secure || sessionCookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("session cookie is missing its security attributes")
	}

	dashboardRequest := httptest.NewRequest(http.MethodGet, "/ui", nil)
	dashboardRequest.AddCookie(sessionCookie)
	dashboardResponse := httptest.NewRecorder()
	handler.ServeHTTP(dashboardResponse, dashboardRequest)
	if dashboardResponse.Code != http.StatusOK {
		t.Fatalf("authenticated GET /ui status = %d, want %d", dashboardResponse.Code, http.StatusOK)
	}
	if !strings.Contains(dashboardResponse.Body.String(), `id="activeTasks"`) {
		t.Fatal("authenticated GET /ui did not return the dashboard")
	}

	apiRequest := httptest.NewRequest(http.MethodGet, "/system/info", nil)
	apiRequest.AddCookie(sessionCookie)
	apiResponse := httptest.NewRecorder()
	handler.ServeHTTP(apiResponse, apiRequest)
	if apiResponse.Code != http.StatusOK {
		t.Fatalf("cookie-authenticated API status = %d, want %d", apiResponse.Code, http.StatusOK)
	}
	if !strings.Contains(apiResponse.Body.String(), `"success":true`) {
		t.Fatalf("cookie-authenticated API response = %s", apiResponse.Body.String())
	}

	logoutRequest := httptest.NewRequest(http.MethodGet, "/ui/logout", nil)
	logoutRequest.AddCookie(sessionCookie)
	logoutResponse := httptest.NewRecorder()
	handler.ServeHTTP(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusSeeOther {
		t.Fatalf("GET /ui/logout status = %d, want %d", logoutResponse.Code, http.StatusSeeOther)
	}

	apiAfterLogout := httptest.NewRequest(http.MethodGet, "/system/info", nil)
	apiAfterLogout.AddCookie(sessionCookie)
	apiAfterLogoutResponse := httptest.NewRecorder()
	handler.ServeHTTP(apiAfterLogoutResponse, apiAfterLogout)
	if apiAfterLogoutResponse.Code != http.StatusUnauthorized {
		t.Fatalf("API status after logout = %d, want %d", apiAfterLogoutResponse.Code, http.StatusUnauthorized)
	}
}

func TestAPIHeaderAuthenticationRemainsSupported(t *testing.T) {
	_, handler := testServer()

	unauthenticated := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/system/info", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated API status = %d, want %d", unauthenticated.Code, http.StatusUnauthorized)
	}

	authenticatedRequest := httptest.NewRequest(http.MethodGet, "/system/info", nil)
	authenticatedRequest.Header.Set("X-Gateway-Token", testGatewayToken)
	authenticated := httptest.NewRecorder()
	handler.ServeHTTP(authenticated, authenticatedRequest)
	if authenticated.Code != http.StatusOK {
		t.Fatalf("header-authenticated API status = %d, want %d", authenticated.Code, http.StatusOK)
	}
}

func TestUIRejectsInvalidToken(t *testing.T) {
	_, handler := testServer()

	form := url.Values{"token": {"wrong-token"}}
	request := httptest.NewRequest(http.MethodPost, "/ui/auth", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("invalid login status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == "sandbox_session" && cookie.Value != "" {
			t.Fatal("invalid login set a session cookie")
		}
	}
}
