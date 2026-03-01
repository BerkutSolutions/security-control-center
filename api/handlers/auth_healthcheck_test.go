package handlers

// Healthcheck access tests.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/auth"
	"berkut-scc/core/store"
)

func TestHealthcheckPageRequiresCookieAndConsumesIt(t *testing.T) {
	h := NewAuthHandler(&config.AppConfig{}, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// Missing cookie -> redirect.
	req := httptest.NewRequest(http.MethodGet, "/healthcheck", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.SessionContextKey, &store.SessionRecord{Username: "u1"}))
	rr := httptest.NewRecorder()
	h.HealthcheckPage(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/dashboard" {
		t.Fatalf("expected redirect to /dashboard, got %q", loc)
	}

	// With cookie -> 200 and cookie cleared.
	req2 := httptest.NewRequest(http.MethodGet, "/healthcheck", nil)
	req2.AddCookie(&http.Cookie{Name: HealthcheckCookieName, Value: "v1:" + strconv.FormatInt(time.Now().UTC().UnixMilli(), 10)})
	req2 = req2.WithContext(context.WithValue(req2.Context(), auth.SessionContextKey, &store.SessionRecord{Username: "u1"}))
	rr2 := httptest.NewRecorder()
	h.HealthcheckPage(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr2.Code)
	}
	if rr2.Body.Len() == 0 {
		t.Fatalf("expected body")
	}
	setCookie := rr2.Header().Values("Set-Cookie")
	joined := strings.Join(setCookie, "\n")
	if !strings.Contains(joined, HealthcheckCookieName+"=") || !strings.Contains(strings.ToLower(joined), "max-age=0") {
		t.Fatalf("expected healthcheck cookie reset, got: %s", joined)
	}
}
