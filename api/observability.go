package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var processStartedAt = time.Now().UTC()

func (s *Server) registerObservabilityRoutes() {
	s.router.MethodFunc("GET", "/healthz", s.healthz)
	s.router.MethodFunc("GET", "/readyz", s.readyz)

	if s.cfg != nil && s.cfg.Observability.MetricsEnabled {
		reg := prometheus.NewRegistry()
		_ = reg.Register(collectors.NewGoCollector())
		_ = reg.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
		reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "berkut_uptime_seconds",
			Help: "Process uptime in seconds.",
		}, func() float64 {
			return time.Since(processStartedAt).Seconds()
		}))

		reg.MustRegister(newAppJobsMetricsCollector(s.db))
		reg.MustRegister(newBackupsMetricsCollector(s.db))
		reg.MustRegister(newMonitoringMetricsCollector(s.monitoringEngine))
		reg.MustRegister(newWorkersMetricsCollector(s.tasksScheduler, s.backupsScheduler, s.appJobsWorker, s.monitoringEngine))

		handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
		s.router.Method("GET", "/metrics", s.requireMetricsAuth(handler))
	}
}

func (s *Server) requireMetricsAuth(next http.Handler) http.Handler {
	if s == nil || s.cfg == nil {
		return next
	}
	token := strings.TrimSpace(s.cfg.Observability.MetricsToken)
	if token == "" {
		allowUnauth := s.cfg.IsHomeMode() && s.cfg.Observability.MetricsAllowUnauthInHome
		if allowUnauth {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		})
	}
	expected := "Bearer " + token
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != expected {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	deploymentMode := ""
	appEnv := ""
	if s != nil && s.cfg != nil {
		deploymentMode = s.cfg.DeploymentMode
		appEnv = s.cfg.AppEnv
	}
	writeJSONPlain(w, http.StatusOK, map[string]any{
		"ok":              true,
		"now":             time.Now().UTC().Format(time.RFC3339Nano),
		"uptime_sec":      int64(time.Since(processStartedAt).Seconds()),
		"deployment_mode": deploymentMode,
		"app_env":         appEnv,
	})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
	defer cancel()
	if s == nil || s.db == nil {
		writeJSONPlain(w, http.StatusServiceUnavailable, map[string]any{"ok": false})
		return
	}
	if err := s.db.PingContext(ctx); err != nil {
		writeJSONPlain(w, http.StatusServiceUnavailable, map[string]any{"ok": false})
		return
	}
	writeJSONPlain(w, http.StatusOK, map[string]any{"ok": true})
}

func writeJSONPlain(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
