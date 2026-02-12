package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath = "config/app.yaml"
	envPrefix         = "BERKUT_"
)

func Load() (*AppConfig, error) {
	cfg := &AppConfig{
		DBPath:     "data/berkut.db",
		ListenAddr: "0.0.0.0:8080",
		SessionTTL: 24 * time.Hour,
		DeploymentMode: "enterprise",
		Security: SecurityConfig{
			TagsSubsetEnforced: true,
			OnlineWindowSec:    300,
			LegacyImportEnabled: false,
		},
		Incidents: IncidentsConfig{
			RegNoFormat: "INC-{year}-{seq:05}",
		},
		Docs: DocsConfig{
			StoragePath:       "data/docs",
			StorageDir:        "data/docs",
			RegTemplate:       "{level}.{year}.{seq}",
			PerFolderSequence: false,
			VersionLimit:      10,
			Watermark: WatermarkConfig{
				Enabled:      true,
				MinLevel:     "CONFIDENTIAL",
				TextTemplate: "Berkut SCC • {classification} • {username} • {timestamp} • DocNo {reg_no}",
				Placement:    "header",
			},
			Converters: ConvertersConfig{
				Enabled:     false,
				PandocPath:  "pandoc",
				SofficePath: "soffice",
				TimeoutSec:  20,
				TempDir:     "",
			},
		},
		Scheduler: SchedulerConfig{
			Enabled:        true,
			IntervalSeconds: 60,
			MaxJobsPerTick: 20,
		},
	}
	if raw, err := os.ReadFile(resolveConfigPath()); err == nil {
		if err := yaml.Unmarshal(raw, cfg); err != nil {
			return nil, err
		}
	}
	overrideFromEnv(cfg)
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func overrideFromEnv(cfg *AppConfig) {
	if v := getEnv("DATA_PATH", envPrefix+"DATA_PATH"); v != "" {
		base := strings.TrimSpace(v)
		cfg.DBPath = filepathJoin(base, "berkut.db")
		cfg.Docs.StoragePath = filepathJoin(base, "docs")
		cfg.Docs.StorageDir = filepathJoin(base, "docs")
		cfg.Incidents.StorageDir = filepathJoin(base, "incidents")
	}
	if v := os.Getenv(envPrefix + "DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv(envPrefix + "LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := getEnv("PORT", envPrefix+"PORT"); v != "" {
		cfg.ListenAddr = listenAddrWithPort(cfg.ListenAddr, v)
	}
	if v := os.Getenv(envPrefix + "SESSION_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.SessionTTL = d
		}
	}
	if v := getEnv("CSRF_KEY", envPrefix+"CSRF_KEY"); v != "" {
		cfg.CSRFKey = v
	}
	if v := getEnv("PEPPER", envPrefix+"PEPPER"); v != "" {
		cfg.Pepper = v
	}
	if v := getEnv("ENV", "APP_ENV", envPrefix+"APP_ENV"); v != "" {
		cfg.AppEnv = v
	}
	if v := getEnv("DEPLOYMENT_MODE", envPrefix+"DEPLOYMENT_MODE"); v != "" {
		mode := strings.ToLower(strings.TrimSpace(v))
		if mode == "home" || mode == "enterprise" {
			cfg.DeploymentMode = mode
		}
	}
	if v := os.Getenv(envPrefix + "TLS_ENABLED"); v != "" {
		cfg.TLSEnabled = v == "1" || v == "true" || v == "TRUE"
	}
	if v := os.Getenv(envPrefix + "TLS_CERT"); v != "" {
		cfg.TLSCert = v
	}
	if v := os.Getenv(envPrefix + "TLS_KEY"); v != "" {
		cfg.TLSKey = v
	}
	if v := os.Getenv(envPrefix + "DOCS_STORAGE_PATH"); v != "" {
		cfg.Docs.StoragePath = v
		cfg.Docs.StorageDir = v
	}
	if v := os.Getenv(envPrefix + "DOCS_STORAGE_DIR"); v != "" {
		cfg.Docs.StorageDir = v
	}
	if v := getEnv("DOCS_ENCRYPTION_KEY", envPrefix+"DOCS_ENCRYPTION_KEY"); v != "" {
		cfg.Docs.EncryptionKey = v
	}
	if v := os.Getenv(envPrefix + "DOCS_ENCRYPTION_KEY_ID"); v != "" {
		cfg.Docs.EncryptionKeyID = v
	}
	if v := os.Getenv(envPrefix + "DOCS_REG_TEMPLATE"); v != "" {
		cfg.Docs.RegTemplate = v
	}
	if v := os.Getenv(envPrefix + "DOCS_PER_FOLDER_SEQUENCE"); v != "" {
		cfg.Docs.PerFolderSequence = v == "1" || strings.ToLower(v) == "true"
	}
	if v := os.Getenv(envPrefix + "DOCS_VERSION_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Docs.VersionLimit = n
		}
	}
	if v := os.Getenv(envPrefix + "DOCS_WATERMARK_ENABLED"); v != "" {
		cfg.Docs.Watermark.Enabled = v == "1" || strings.ToLower(v) == "true"
	}
	if v := os.Getenv(envPrefix + "DOCS_WATERMARK_MIN_LEVEL"); v != "" {
		cfg.Docs.Watermark.MinLevel = v
		cfg.Docs.WatermarkMinLevel = v
	}
	if v := os.Getenv(envPrefix + "DOCS_WATERMARK_TEMPLATE"); v != "" {
		cfg.Docs.Watermark.TextTemplate = v
	}
	if v := os.Getenv(envPrefix + "DOCS_WATERMARK_PLACEMENT"); v != "" {
		cfg.Docs.Watermark.Placement = v
	}
	if v := os.Getenv(envPrefix + "DOCS_CONVERTERS_ENABLED"); v != "" {
		cfg.Docs.Converters.Enabled = v == "1" || strings.ToLower(v) == "true"
	}
	if v := os.Getenv(envPrefix + "DOCS_CONVERTERS_PANDOC_PATH"); v != "" {
		cfg.Docs.Converters.PandocPath = v
	}
	if v := os.Getenv(envPrefix + "DOCS_CONVERTERS_SOFFICE_PATH"); v != "" {
		cfg.Docs.Converters.SofficePath = v
	}
	if v := os.Getenv(envPrefix + "DOCS_CONVERTERS_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Docs.Converters.TimeoutSec = n
		}
	}
	if v := os.Getenv(envPrefix + "DOCS_CONVERTERS_TEMP_DIR"); v != "" {
		cfg.Docs.Converters.TempDir = v
	}
	if v := os.Getenv(envPrefix + "SECURITY_TAGS_SUBSET"); v != "" {
		cfg.Security.TagsSubsetEnforced = v == "1" || strings.ToLower(v) == "true"
	}
	if v := os.Getenv(envPrefix + "SECURITY_ONLINE_WINDOW_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Security.OnlineWindowSec = n
		}
	}
	if v := os.Getenv(envPrefix + "SECURITY_LEGACY_IMPORT_ENABLED"); v != "" {
		cfg.Security.LegacyImportEnabled = v == "1" || strings.ToLower(v) == "true"
	}
	if v := os.Getenv(envPrefix + "SECURITY_TRUSTED_PROXIES"); v != "" {
		cfg.Security.TrustedProxies = splitList(v)
	}
	if v := os.Getenv(envPrefix + "INCIDENTS_REG_NO_FORMAT"); v != "" {
		cfg.Incidents.RegNoFormat = v
	}
	if v := os.Getenv(envPrefix + "INCIDENTS_STORAGE_DIR"); v != "" {
		cfg.Incidents.StorageDir = v
	}
	if v := os.Getenv(envPrefix + "SCHEDULER_ENABLED"); v != "" {
		cfg.Scheduler.Enabled = v == "1" || strings.ToLower(v) == "true"
	}
	if v := os.Getenv(envPrefix + "SCHEDULER_INTERVAL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Scheduler.IntervalSeconds = n
		}
	}
	if v := os.Getenv(envPrefix + "SCHEDULER_MAX_JOBS_PER_TICK"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Scheduler.MaxJobsPerTick = n
		}
	}
	if cfg.DeploymentMode == "" {
		cfg.DeploymentMode = "enterprise"
	}
	if cfg.DeploymentMode != "home" && cfg.DeploymentMode != "enterprise" {
		cfg.DeploymentMode = "enterprise"
	}
	if cfg.DeploymentMode == "enterprise" {
		appEnv := strings.ToLower(strings.TrimSpace(cfg.AppEnv))
		if appEnv == "dev" {
			cfg.DeploymentMode = "home"
		}
	}
	if cfg.DeploymentMode == "home" {
		cfg.TLSEnabled = false
	}
}

func getEnv(keys ...string) string {
	for _, key := range keys {
		if key == "" {
			continue
		}
		if val := os.Getenv(key); val != "" {
			return val
		}
	}
	return ""
}

func splitList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		val := strings.TrimSpace(part)
		if val != "" {
			out = append(out, val)
		}
	}
	return out
}

func resolveConfigPath() string {
	if v := getEnv("APP_CONFIG", envPrefix+"APP_CONFIG"); v != "" {
		return strings.TrimSpace(v)
	}
	return defaultConfigPath
}

func listenAddrWithPort(currentAddr, portRaw string) string {
	port := strings.TrimSpace(portRaw)
	if port == "" {
		return currentAddr
	}
	if _, err := strconv.Atoi(port); err != nil {
		return currentAddr
	}
	host := "0.0.0.0"
	parts := strings.Split(strings.TrimSpace(currentAddr), ":")
	if len(parts) > 1 {
		host = strings.Join(parts[:len(parts)-1], ":")
	}
	if host == "" {
		host = "0.0.0.0"
	}
	return host + ":" + port
}

func filepathJoin(base, leaf string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return leaf
	}
	base = strings.TrimRight(base, "/\\")
	return base + string(os.PathSeparator) + leaf
}
