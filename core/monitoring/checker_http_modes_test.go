package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"berkut-scc/core/store"
)

func TestCheckMonitorHTTPKeywordFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("service status: HEALTHY"))
	}))
	defer srv.Close()

	res := CheckMonitor(context.Background(), store.Monitor{
		Type:          "http_keyword",
		URL:           srv.URL,
		Method:        "GET",
		RequestBody:   "HEALTHY",
		AllowedStatus: []string{"200-299"},
		TimeoutSec:    3,
	}, store.MonitorSettings{AllowPrivateNetworks: true, DefaultTimeoutSec: 3})

	if !res.OK {
		t.Fatalf("expected ok, got error=%s status=%v", res.Error, res.StatusCode)
	}
}

func TestCheckMonitorHTTPKeywordMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("all good"))
	}))
	defer srv.Close()

	res := CheckMonitor(context.Background(), store.Monitor{
		Type:          "http_keyword",
		URL:           srv.URL,
		Method:        "GET",
		RequestBody:   "MUST_EXIST",
		AllowedStatus: []string{"200-299"},
		TimeoutSec:    3,
	}, store.MonitorSettings{AllowPrivateNetworks: true, DefaultTimeoutSec: 3})

	if res.OK {
		t.Fatalf("expected not ok")
	}
	if res.Error != "monitoring.error.keywordNotFound" {
		t.Fatalf("expected keywordNotFound, got %q", res.Error)
	}
}

func TestCheckMonitorHTTPJSONValidation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer srv.Close()

	okRes := CheckMonitor(context.Background(), store.Monitor{
		Type:          "http_json",
		URL:           fmt.Sprintf("%s/ok", srv.URL),
		Method:        "GET",
		AllowedStatus: []string{"200-299"},
		TimeoutSec:    3,
	}, store.MonitorSettings{AllowPrivateNetworks: true, DefaultTimeoutSec: 3})
	if !okRes.OK {
		t.Fatalf("expected ok for json endpoint, got error=%s", okRes.Error)
	}

	badRes := CheckMonitor(context.Background(), store.Monitor{
		Type:          "http_json",
		URL:           fmt.Sprintf("%s/bad", srv.URL),
		Method:        "GET",
		AllowedStatus: []string{"200-299"},
		TimeoutSec:    3,
	}, store.MonitorSettings{AllowPrivateNetworks: true, DefaultTimeoutSec: 3})
	if badRes.OK {
		t.Fatalf("expected not ok for invalid json")
	}
	if badRes.Error != "monitoring.error.invalidJsonResponse" {
		t.Fatalf("expected invalidJsonResponse, got %q", badRes.Error)
	}
}

func TestCheckMonitorHTTPRedirectFollowDefault(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`ok`))
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusMovedPermanently)
	}))
	defer redirector.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res := CheckMonitor(ctx, store.Monitor{
		Type:          "http",
		URL:           redirector.URL,
		Method:        "GET",
		AllowedStatus: []string{"200-299"},
		TimeoutSec:    3,
	}, store.MonitorSettings{AllowPrivateNetworks: true, DefaultTimeoutSec: 3})

	if !res.OK {
		t.Fatalf("expected redirect-follow success, got err=%s status=%v", res.Error, res.StatusCode)
	}
}

func TestCheckMonitorPingByHost(t *testing.T) {
	res := CheckMonitor(context.Background(), store.Monitor{
		Type:       TypePing,
		Host:       "localhost",
		TimeoutSec: 2,
	}, store.MonitorSettings{AllowPrivateNetworks: true, DefaultTimeoutSec: 2})
	if !res.OK {
		t.Fatalf("expected ping-like check ok, got error=%s", res.Error)
	}
}

func TestCheckMonitorPingWithHostPort(t *testing.T) {
	res := CheckMonitor(context.Background(), store.Monitor{
		Type:       TypePing,
		Host:       "127.0.0.1:80",
		TimeoutSec: 2,
	}, store.MonitorSettings{AllowPrivateNetworks: true, DefaultTimeoutSec: 2})
	if res.OK && res.Error != "" {
		t.Fatalf("expected clean result, got error=%s", res.Error)
	}
}

func TestCheckMonitorPingWithHostPortConnectionRefused(t *testing.T) {
	res := CheckMonitor(context.Background(), store.Monitor{
		Type:       TypePing,
		Host:       "127.0.0.1:1",
		TimeoutSec: 2,
	}, store.MonitorSettings{AllowPrivateNetworks: true, DefaultTimeoutSec: 2})
	if res.OK {
		t.Fatalf("expected failed ping-like check for closed port")
	}
}
