package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

const (
	defaultConfigPath = "config/app.yaml"
	envPrefix         = "BERKUT_"
)

func Load() (*AppConfig, error) {
	cfg := &AppConfig{}
	cfgPath := resolveConfigPath()
	if st, err := os.Stat(cfgPath); err == nil && !st.IsDir() {
		if err := cleanenv.ReadConfig(cfgPath, cfg); err != nil {
			return nil, err
		}
	}
	if err := cleanenv.ReadEnv(cfg); err != nil {
		return nil, err
	}
	applyEnvAliases(cfg)
	normalizeConfig(cfg)
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyEnvAliases(cfg *AppConfig) {
	if cfg == nil {
		return
	}
	if v := getEnv("CSRF_KEY"); v != "" {
		cfg.CSRFKey = strings.TrimSpace(v)
	}
	if v := getEnv("PEPPER"); v != "" {
		cfg.Pepper = strings.TrimSpace(v)
	}
	if v := getEnv("DOCS_ENCRYPTION_KEY"); v != "" {
		cfg.Docs.EncryptionKey = strings.TrimSpace(v)
	}
	if v := getEnv("ENV", "APP_ENV"); v != "" {
		cfg.AppEnv = strings.TrimSpace(v)
	}
	if v := getEnv("DEPLOYMENT_MODE"); v != "" {
		cfg.DeploymentMode = strings.ToLower(strings.TrimSpace(v))
	}
	if v := getEnv("PORT", envPrefix+"PORT"); v != "" {
		cfg.ListenAddr = listenAddrWithPort(cfg.ListenAddr, v)
	}
	if v := getEnv("DATA_PATH", envPrefix+"DATA_PATH"); v != "" {
		base := strings.TrimSpace(v)
		cfg.Docs.StoragePath = filepathJoin(base, "docs")
		cfg.Docs.StorageDir = filepathJoin(base, "docs")
		cfg.Incidents.StorageDir = filepathJoin(base, "incidents")
		cfg.Backups.Path = filepathJoin(base, "backups")
	}
	if v := getEnv("DOCS_STORAGE_PATH"); v != "" {
		cfg.Docs.StoragePath = strings.TrimSpace(v)
		cfg.Docs.StorageDir = strings.TrimSpace(v)
	}
	if v := getEnv("DOCS_STORAGE_DIR"); v != "" {
		cfg.Docs.StorageDir = strings.TrimSpace(v)
	}
	if v := getEnv("INCIDENTS_STORAGE_DIR"); v != "" {
		cfg.Incidents.StorageDir = strings.TrimSpace(v)
	}
	if v := getEnv("BACKUP_PATH"); v != "" {
		cfg.Backups.Path = strings.TrimSpace(v)
	}
	if v := getEnv("BACKUP_ENCRYPTION_KEY"); v != "" {
		cfg.Backups.EncryptionKey = strings.TrimSpace(v)
	}
	if v := getEnv("BACKUP_PGDUMP_BIN"); v != "" {
		cfg.Backups.PGDumpBin = strings.TrimSpace(v)
	}
	if v := getEnv("BACKUP_MAX_PARALLEL"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			cfg.Backups.MaxParallel = n
		}
	}
	if v := getEnv("BACKUP_UPLOAD_MAX_BYTES"); v != "" {
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			cfg.Backups.UploadMaxBytes = n
		}
	}
}

func normalizeConfig(cfg *AppConfig) {
	if cfg == nil {
		return
	}
	cfg.DBDriver = strings.ToLower(strings.TrimSpace(cfg.DBDriver))
	cfg.DBURL = strings.TrimSpace(cfg.DBURL)
	cfg.ListenAddr = strings.TrimSpace(cfg.ListenAddr)
	cfg.AppEnv = strings.ToLower(strings.TrimSpace(cfg.AppEnv))
	cfg.DeploymentMode = strings.ToLower(strings.TrimSpace(cfg.DeploymentMode))
	cfg.CSRFKey = strings.TrimSpace(cfg.CSRFKey)
	cfg.Pepper = strings.TrimSpace(cfg.Pepper)
	cfg.Docs.EncryptionKey = strings.TrimSpace(cfg.Docs.EncryptionKey)
	cfg.Docs.EncryptionKeyID = strings.TrimSpace(cfg.Docs.EncryptionKeyID)
	cfg.Docs.StoragePath = strings.TrimSpace(cfg.Docs.StoragePath)
	cfg.Docs.StorageDir = strings.TrimSpace(cfg.Docs.StorageDir)
	cfg.Incidents.StorageDir = strings.TrimSpace(cfg.Incidents.StorageDir)
	cfg.Backups.Path = strings.TrimSpace(cfg.Backups.Path)
	cfg.Backups.EncryptionKey = strings.TrimSpace(cfg.Backups.EncryptionKey)
	cfg.Backups.PGDumpBin = strings.TrimSpace(cfg.Backups.PGDumpBin)
	cfg.Docs.OnlyOffice.PublicURL = strings.TrimSpace(cfg.Docs.OnlyOffice.PublicURL)
	cfg.Docs.OnlyOffice.InternalURL = strings.TrimSpace(cfg.Docs.OnlyOffice.InternalURL)
	cfg.Docs.OnlyOffice.AppInternalURL = strings.TrimSpace(cfg.Docs.OnlyOffice.AppInternalURL)
	cfg.Docs.OnlyOffice.JWTSecret = strings.TrimSpace(cfg.Docs.OnlyOffice.JWTSecret)
	cfg.Docs.OnlyOffice.JWTHeader = strings.TrimSpace(cfg.Docs.OnlyOffice.JWTHeader)
	cfg.Docs.OnlyOffice.JWTIssuer = strings.TrimSpace(cfg.Docs.OnlyOffice.JWTIssuer)
	cfg.Docs.OnlyOffice.JWTAudience = strings.TrimSpace(cfg.Docs.OnlyOffice.JWTAudience)
	if cfg.Backups.PGDumpBin == "" {
		cfg.Backups.PGDumpBin = "pg_dump"
	}
	if cfg.Backups.MaxParallel <= 0 {
		cfg.Backups.MaxParallel = 1
	}
	if cfg.Backups.UploadMaxBytes <= 0 {
		cfg.Backups.UploadMaxBytes = 512 * 1024 * 1024
	}
	if cfg.Docs.StorageDir == "" && cfg.Docs.StoragePath != "" {
		cfg.Docs.StorageDir = cfg.Docs.StoragePath
	}
	if cfg.Docs.StoragePath == "" && cfg.Docs.StorageDir != "" {
		cfg.Docs.StoragePath = cfg.Docs.StorageDir
	}
	if cfg.Docs.Watermark.MinLevel == "" && cfg.Docs.WatermarkMinLevel != "" {
		cfg.Docs.Watermark.MinLevel = cfg.Docs.WatermarkMinLevel
	}
	if cfg.Docs.Watermark.MinLevel != "" {
		cfg.Docs.WatermarkMinLevel = cfg.Docs.Watermark.MinLevel
	}
	if cfg.Docs.OnlyOffice.InternalURL == "" {
		cfg.Docs.OnlyOffice.InternalURL = "http://onlyoffice/"
	}
	if cfg.Docs.OnlyOffice.AppInternalURL == "" {
		cfg.Docs.OnlyOffice.AppInternalURL = "http://berkut:8080"
	}
	if cfg.Docs.OnlyOffice.JWTHeader == "" {
		cfg.Docs.OnlyOffice.JWTHeader = "Authorization"
	}
	if cfg.Docs.OnlyOffice.JWTIssuer == "" {
		cfg.Docs.OnlyOffice.JWTIssuer = "berkut-scc"
	}
	if cfg.Docs.OnlyOffice.JWTAudience == "" {
		cfg.Docs.OnlyOffice.JWTAudience = "onlyoffice-document-server"
	}
	if cfg.Docs.OnlyOffice.RequestTimeout <= 0 {
		cfg.Docs.OnlyOffice.RequestTimeout = 20
	}
	if cfg.DeploymentMode == "" {
		cfg.DeploymentMode = "enterprise"
	}
	if cfg.DeploymentMode != "home" && cfg.DeploymentMode != "enterprise" {
		cfg.DeploymentMode = "enterprise"
	}
	if cfg.DeploymentMode == "enterprise" && cfg.AppEnv == "dev" {
		cfg.DeploymentMode = "home"
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
