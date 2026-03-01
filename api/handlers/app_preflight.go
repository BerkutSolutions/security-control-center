package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/appmeta"
	"berkut-scc/core/netguard"
	"berkut-scc/core/store"
)

type PreflightHandler struct {
	cfg *config.AppConfig
	db  *sql.DB
}

type PreflightCheck struct {
	ID      string         `json:"id"`
	Status  string         `json:"status"` // ok|needs_attention|failed
	I18NKey string         `json:"i18n_key"`
	Details map[string]any `json:"details,omitempty"`
}

type PreflightReport struct {
	NowUTC    time.Time             `json:"now_utc"`
	OK        bool                  `json:"ok"`
	AppVer    string                `json:"app_version"`
	RunMode   string                `json:"run_mode"`
	Checks    []PreflightCheck      `json:"checks"`
	Migration store.MigrationStatus `json:"migration"`
}

func NewPreflightHandler(cfg *config.AppConfig, db *sql.DB) *PreflightHandler {
	return &PreflightHandler{cfg: cfg, db: db}
}

func (h *PreflightHandler) Report(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	report := PreflightReport{
		NowUTC:  time.Now().UTC(),
		AppVer:  appmeta.AppVersion,
		RunMode: "",
		Checks:  []PreflightCheck{},
	}
	if h.cfg != nil {
		report.RunMode = h.cfg.RunMode
	}

	report.Checks = append(report.Checks, h.checkDB(ctx))
	report.Migration = h.checkMigrations(ctx, &report.Checks)
	report.Checks = append(report.Checks, h.checkStorage())
	report.Checks = append(report.Checks, h.checkRunMode())
	report.Checks = append(report.Checks, h.checkTrustedProxies())
	report.Checks = append(report.Checks, h.checkMetrics())
	report.Checks = append(report.Checks, h.checkOnlyOffice(ctx))

	ok := true
	for _, c := range report.Checks {
		if strings.EqualFold(c.Status, "failed") {
			ok = false
			break
		}
	}
	report.OK = ok
	writeJSON(w, http.StatusOK, report)
}

func (h *PreflightHandler) checkDB(ctx context.Context) PreflightCheck {
	if h == nil || h.db == nil {
		return PreflightCheck{ID: "db", Status: "failed", I18NKey: "preflight.db.missing"}
	}
	pingCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	if err := h.db.PingContext(pingCtx); err != nil {
		return PreflightCheck{ID: "db", Status: "failed", I18NKey: "preflight.db.failed"}
	}
	return PreflightCheck{ID: "db", Status: "ok", I18NKey: "preflight.db.ok"}
}

func (h *PreflightHandler) checkMigrations(ctx context.Context, out *[]PreflightCheck) store.MigrationStatus {
	if h == nil || h.db == nil {
		if out != nil {
			*out = append(*out, PreflightCheck{ID: "migrations", Status: "failed", I18NKey: "preflight.migrations.db_missing"})
		}
		return store.MigrationStatus{NowUTC: time.Now().UTC()}
	}
	status, err := store.GetMigrationStatus(ctx, h.db)
	if err != nil {
		if out != nil {
			*out = append(*out, PreflightCheck{ID: "migrations", Status: "failed", I18NKey: "preflight.migrations.failed"})
		}
		return store.MigrationStatus{NowUTC: time.Now().UTC()}
	}
	if status.LegacyDatabase {
		if out != nil {
			*out = append(*out, PreflightCheck{ID: "migrations", Status: "failed", I18NKey: "preflight.migrations.legacy"})
		}
		return status
	}
	if status.HasPending {
		if out != nil {
			*out = append(*out, PreflightCheck{
				ID:      "migrations",
				Status:  "needs_attention",
				I18NKey: "preflight.migrations.pending",
				Details: map[string]any{
					"current": status.CurrentVersion,
					"latest":  status.LatestVersion,
				},
			})
		}
		return status
	}
	if out != nil {
		*out = append(*out, PreflightCheck{ID: "migrations", Status: "ok", I18NKey: "preflight.migrations.ok"})
	}
	return status
}

func (h *PreflightHandler) checkStorage() PreflightCheck {
	if h == nil || h.cfg == nil {
		return PreflightCheck{ID: "storage", Status: "needs_attention", I18NKey: "preflight.storage.unknown"}
	}
	type item struct {
		id   string
		path string
	}
	items := []item{
		{id: "docs", path: h.cfg.Docs.StorageDir},
		{id: "incidents", path: h.cfg.Incidents.StorageDir},
		{id: "backups", path: h.cfg.Backups.Path},
	}
	missing := []string{}
	for _, it := range items {
		p := strings.TrimSpace(it.path)
		if p == "" {
			missing = append(missing, it.id)
			continue
		}
		if st, err := os.Stat(p); err != nil || st == nil || !st.IsDir() {
			missing = append(missing, it.id)
			continue
		}
	}
	if len(missing) > 0 {
		return PreflightCheck{
			ID:      "storage",
			Status:  "failed",
			I18NKey: "preflight.storage.missing",
			Details: map[string]any{"missing": missing},
		}
	}
	return PreflightCheck{ID: "storage", Status: "ok", I18NKey: "preflight.storage.ok"}
}

func (h *PreflightHandler) checkRunMode() PreflightCheck {
	if h == nil || h.cfg == nil {
		return PreflightCheck{ID: "run_mode", Status: "needs_attention", I18NKey: "preflight.run_mode.unknown"}
	}
	if strings.EqualFold(h.cfg.RunMode, "api") {
		return PreflightCheck{ID: "run_mode", Status: "needs_attention", I18NKey: "preflight.run_mode.api"}
	}
	if strings.EqualFold(h.cfg.RunMode, "worker") {
		return PreflightCheck{ID: "run_mode", Status: "ok", I18NKey: "preflight.run_mode.worker"}
	}
	return PreflightCheck{ID: "run_mode", Status: "ok", I18NKey: "preflight.run_mode.all"}
}

func (h *PreflightHandler) checkOnlyOffice(ctx context.Context) PreflightCheck {
	if h == nil || h.cfg == nil || !h.cfg.Docs.OnlyOffice.Enabled {
		return PreflightCheck{ID: "onlyoffice", Status: "ok", I18NKey: "preflight.onlyoffice.disabled"}
	}
	raw := strings.TrimSpace(h.cfg.Docs.OnlyOffice.InternalURL)
	if raw == "" {
		return PreflightCheck{ID: "onlyoffice", Status: "needs_attention", I18NKey: "preflight.onlyoffice.missing_url"}
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return PreflightCheck{ID: "onlyoffice", Status: "needs_attention", I18NKey: "preflight.onlyoffice.bad_url"}
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "http" && scheme != "https" {
		return PreflightCheck{ID: "onlyoffice", Status: "needs_attention", I18NKey: "preflight.onlyoffice.bad_url"}
	}
	host := u.Hostname()
	if err := netguard.ValidateHost(ctx, host, netguard.Policy{AllowPrivate: true, AllowLoopback: true}); err != nil {
		if errors.Is(err, netguard.ErrRestrictedTarget) || errors.Is(err, netguard.ErrPrivateNetworkBlocked) {
			return PreflightCheck{ID: "onlyoffice", Status: "needs_attention", I18NKey: "preflight.onlyoffice.restricted_url", Details: map[string]any{"host": host}}
		}
		return PreflightCheck{ID: "onlyoffice", Status: "needs_attention", I18NKey: "preflight.onlyoffice.bad_url"}
	}
	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "https":
			port = "443"
		default:
			port = "80"
		}
	}
	addr := net.JoinHostPort(host, port)
	d := net.Dialer{Timeout: 800 * time.Millisecond}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return PreflightCheck{ID: "onlyoffice", Status: "needs_attention", I18NKey: "preflight.onlyoffice.unreachable", Details: map[string]any{"addr": addr}}
	}
	_ = conn.Close()
	return PreflightCheck{ID: "onlyoffice", Status: "ok", I18NKey: "preflight.onlyoffice.ok", Details: map[string]any{"addr": addr, "path": filepath.Clean(u.Path)}}
}

func (h *PreflightHandler) checkTrustedProxies() PreflightCheck {
	if h == nil || h.cfg == nil {
		return PreflightCheck{ID: "trusted_proxies", Status: "needs_attention", I18NKey: "preflight.trusted_proxies.unknown"}
	}
	proxies := append([]string(nil), h.cfg.Security.TrustedProxies...)
	if len(proxies) == 0 {
		return PreflightCheck{ID: "trusted_proxies", Status: "ok", I18NKey: "preflight.trusted_proxies.empty"}
	}
	broad := false
	for _, proxy := range proxies {
		if isBroadTrustedProxyCIDR(proxy) {
			broad = true
			break
		}
	}
	if broad {
		return PreflightCheck{ID: "trusted_proxies", Status: "needs_attention", I18NKey: "preflight.trusted_proxies.broad", Details: map[string]any{"count": len(proxies)}}
	}
	return PreflightCheck{ID: "trusted_proxies", Status: "ok", I18NKey: "preflight.trusted_proxies.ok", Details: map[string]any{"count": len(proxies)}}
}

func (h *PreflightHandler) checkMetrics() PreflightCheck {
	if h == nil || h.cfg == nil || !h.cfg.Observability.MetricsEnabled {
		return PreflightCheck{ID: "metrics", Status: "ok", I18NKey: "preflight.metrics.disabled"}
	}
	if strings.TrimSpace(h.cfg.Observability.MetricsToken) == "" {
		if h.cfg.IsHomeMode() && h.cfg.Observability.MetricsAllowUnauthInHome {
			return PreflightCheck{ID: "metrics", Status: "ok", I18NKey: "preflight.metrics.unauth_home"}
		}
		return PreflightCheck{ID: "metrics", Status: "needs_attention", I18NKey: "preflight.metrics.no_token"}
	}
	return PreflightCheck{ID: "metrics", Status: "ok", I18NKey: "preflight.metrics.ok"}
}

func isBroadTrustedProxyCIDR(raw string) bool {
	val := strings.TrimSpace(raw)
	if val == "" || !strings.Contains(val, "/") {
		return false
	}
	_, block, err := net.ParseCIDR(val)
	if err != nil || block == nil {
		return false
	}
	ones, bits := block.Mask.Size()
	if bits <= 0 {
		return false
	}
	is4 := block.IP != nil && block.IP.To4() != nil
	if is4 {
		return ones <= 16
	}
	return ones <= 48
}
