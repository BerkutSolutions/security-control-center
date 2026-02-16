package config

import (
	"path/filepath"
	"testing"
)

func TestLoadWithAliasEnv(t *testing.T) {
	t.Setenv("APP_CONFIG", "config/does-not-exist.yaml")
	t.Setenv("BERKUT_DB_DRIVER", "postgres")
	t.Setenv("BERKUT_DB_URL", "postgres://localhost/test")
	t.Setenv("BERKUT_LISTEN_ADDR", "127.0.0.1:8080")
	t.Setenv("PORT", "9090")
	t.Setenv("ENV", "dev")
	t.Setenv("DEPLOYMENT_MODE", "enterprise")
	t.Setenv("DATA_PATH", filepath.FromSlash("var/data"))
	t.Setenv("CSRF_KEY", "csrf")
	t.Setenv("PEPPER", "pepper")
	t.Setenv("DOCS_ENCRYPTION_KEY", "docskey")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if cfg.ListenAddr != "127.0.0.1:9090" {
		t.Fatalf("unexpected listen addr: %s", cfg.ListenAddr)
	}
	if cfg.DeploymentMode != "home" {
		t.Fatalf("expected home mode for ENV=dev, got %s", cfg.DeploymentMode)
	}
	if cfg.Docs.StorageDir != filepath.Join("var", "data", "docs") {
		t.Fatalf("unexpected docs storage dir: %s", cfg.Docs.StorageDir)
	}
	if cfg.Incidents.StorageDir != filepath.Join("var", "data", "incidents") {
		t.Fatalf("unexpected incidents storage dir: %s", cfg.Incidents.StorageDir)
	}
}
