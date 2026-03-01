package config

import "testing"

func TestValidateRejectsDefaultSecretsInProd(t *testing.T) {
	cfg := &AppConfig{
		DBDriver:   "postgres",
		DBURL:      "postgres://localhost/test",
		AppEnv:     "prod",
		CSRFKey:    defaultCSRFKey,
		Pepper:     defaultPepper,
		TLSEnabled: true,
		Docs: DocsConfig{
			EncryptionKey: defaultDocsEncryptionKey,
		},
		Backups: BackupsConfig{
			EncryptionKey: "backup-key-test-value-1234567890",
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatalf("expected error for default secrets in prod")
	}
}

func TestValidateRejectsTLSDisabledInProd(t *testing.T) {
	cfg := &AppConfig{
		DBDriver:   "postgres",
		DBURL:      "postgres://localhost/test",
		AppEnv:     "prod",
		CSRFKey:    "csrf",
		Pepper:     "pepper",
		TLSEnabled: false,
		Docs: DocsConfig{
			EncryptionKey: "docskey",
		},
		Backups: BackupsConfig{
			EncryptionKey: "backup-key-test-value-1234567890",
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatalf("expected error for tls_disabled in prod")
	}
}

func TestValidateAllowsDevDefaults(t *testing.T) {
	cfg := &AppConfig{
		DBDriver:   "postgres",
		DBURL:      "postgres://localhost/test",
		AppEnv:     "dev",
		CSRFKey:    defaultCSRFKey,
		Pepper:     defaultPepper,
		TLSEnabled: false,
		Docs: DocsConfig{
			EncryptionKey: defaultDocsEncryptionKey,
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("unexpected error for dev defaults: %v", err)
	}
}

func TestValidateAllowsHomeModeWithoutTLS(t *testing.T) {
	cfg := &AppConfig{
		DBDriver:       "postgres",
		DBURL:          "postgres://localhost/test",
		AppEnv:         "prod",
		DeploymentMode: "home",
		CSRFKey:        defaultCSRFKey,
		Pepper:         defaultPepper,
		TLSEnabled:     false,
		Docs:           DocsConfig{EncryptionKey: defaultDocsEncryptionKey},
		Backups: BackupsConfig{
			EncryptionKey: "backup-key-test-value-1234567890",
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("unexpected error for home mode: %v", err)
	}
}

func TestValidateRequiresBackupKeyOutsideDev(t *testing.T) {
	cfg := &AppConfig{
		DBDriver:   "postgres",
		DBURL:      "postgres://localhost/test",
		AppEnv:     "prod",
		CSRFKey:    "csrf",
		Pepper:     "pepper",
		TLSEnabled: true,
		Docs: DocsConfig{
			EncryptionKey: "docskey",
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatalf("expected error for missing backups key")
	}
}

func TestValidateOnlyOfficeRequiresPublicURLAndSecret(t *testing.T) {
	cfg := &AppConfig{
		DBDriver:   "postgres",
		DBURL:      "postgres://localhost/test",
		AppEnv:     "dev",
		CSRFKey:    "csrf",
		Pepper:     "pepper",
		TLSEnabled: false,
		Docs: DocsConfig{
			EncryptionKey: "docskey",
			OnlyOffice: OnlyOfficeConfig{
				Enabled: true,
			},
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatalf("expected onlyoffice config validation error")
	}
	cfg.Docs.OnlyOffice.PublicURL = "https://scc.local/office/"
	cfg.Docs.OnlyOffice.AppInternalURL = "http://berkut:8080"
	cfg.Docs.OnlyOffice.JWTSecret = "test-secret-0123456789abcdef012345"
	if err := Validate(cfg); err != nil {
		t.Fatalf("unexpected error for valid onlyoffice config: %v", err)
	}
}
