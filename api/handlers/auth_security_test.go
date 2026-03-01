package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"berkut-scc/config"
)

func TestIsSecureRequestWithTrustedProxyHTTPS(t *testing.T) {
	cfg := &config.AppConfig{
		Security: config.SecurityConfig{
			TrustedProxies: []string{"10.0.0.10"},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	req.RemoteAddr = "10.0.0.10:32100"
	req.Header.Set("X-Forwarded-Proto", "https")
	if !isSecureRequest(req, cfg) {
		t.Fatalf("expected secure request behind trusted https proxy")
	}
}

func TestIsSecureRequestIgnoresUntrustedProxyHeader(t *testing.T) {
	cfg := &config.AppConfig{
		Security: config.SecurityConfig{
			TrustedProxies: []string{"10.0.0.10"},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	req.RemoteAddr = "192.168.1.20:32100"
	req.Header.Set("X-Forwarded-Proto", "https")
	if isSecureRequest(req, cfg) {
		t.Fatalf("expected insecure request for untrusted proxy source")
	}
}

func TestIsSecureRequestIgnoresBroadTrustedProxyCIDR(t *testing.T) {
	cfg := &config.AppConfig{
		Security: config.SecurityConfig{
			TrustedProxies: []string{"0.0.0.0/0"},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	req.RemoteAddr = "10.0.0.10:32100"
	req.Header.Set("X-Forwarded-Proto", "https")
	if isSecureRequest(req, cfg) {
		t.Fatalf("expected broad trusted proxy cidr to be ignored")
	}
}
