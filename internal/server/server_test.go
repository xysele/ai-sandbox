package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"ai-sandbox-go/internal/config"
)

const testGatewayToken = "test-gateway-token"

func testServer() http.Handler {
	cfg := &config.Config{
		Port:         "7860",
		Display:      ":99",
		GatewayToken: testGatewayToken,
	}
	srv := New(cfg)
	return srv.authMiddleware(srv.mux)
}

func TestAPIHeaderAuthentication(t *testing.T) {
	handler := testServer()

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
		t.Fatalf("authenticated API status = %d, want %d", authenticated.Code, http.StatusOK)
	}
}

func TestRemovedUIEndpoint(t *testing.T) {
	handler := testServer()

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ui", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /ui status = %d, want %d", response.Code, http.StatusUnauthorized)
	}

	authenticatedRequest := httptest.NewRequest(http.MethodGet, "/ui", nil)
	authenticatedRequest.Header.Set("X-Gateway-Token", testGatewayToken)
	authenticated := httptest.NewRecorder()
	handler.ServeHTTP(authenticated, authenticatedRequest)
	if authenticated.Code != http.StatusNotFound {
		t.Fatalf("authenticated /ui status = %d, want %d", authenticated.Code, http.StatusNotFound)
	}
}
