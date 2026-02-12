package config

import "testing"

func TestValidateRejectsDefaultSecretsInProd(t *testing.T) {
	cfg := &AppConfig{
		AppEnv:     "prod",
		CSRFKey:    defaultCSRFKey,
		Pepper:     defaultPepper,
		TLSEnabled: true,
		Docs: DocsConfig{
			EncryptionKey: defaultDocsEncryptionKey,
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatalf("expected error for default secrets in prod")
	}
}

func TestValidateRejectsTLSDisabledInProd(t *testing.T) {
	cfg := &AppConfig{
		AppEnv:     "prod",
		CSRFKey:    "csrf",
		Pepper:     "pepper",
		TLSEnabled: false,
		Docs: DocsConfig{
			EncryptionKey: "docskey",
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatalf("expected error for tls_disabled in prod")
	}
}

func TestValidateAllowsDevDefaults(t *testing.T) {
	cfg := &AppConfig{
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
		AppEnv:          "prod",
		DeploymentMode:  "home",
		CSRFKey:         defaultCSRFKey,
		Pepper:          defaultPepper,
		TLSEnabled:      false,
		Docs: DocsConfig{EncryptionKey: defaultDocsEncryptionKey},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("unexpected error for home mode: %v", err)
	}
}
