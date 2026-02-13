package config

import (
	"fmt"
	"strings"
)

const (
	defaultCSRFKey           = "FWgaRnHOh8Nep_kGLCTiBXIB2k72_G2Ch1Q7HOM0zIo"
	defaultPepper            = "BPY89KfAWweJM5p2Vh0Zwg_-nm7wSlS8La8DxPWFAlg"
	defaultDocsEncryptionKey = "337d0c35c87c92c9dfe05f4251de8ad18fcb363d4be33bcc5394fb013dc22daf"
)

func Validate(cfg *AppConfig) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	driver := strings.ToLower(strings.TrimSpace(cfg.DBDriver))
	if driver == "" {
		driver = "postgres"
	}
	if driver != "postgres" && driver != "pg" {
		return fmt.Errorf("unsupported db_driver: %s", cfg.DBDriver)
	}
	if strings.TrimSpace(cfg.DBURL) == "" {
		return fmt.Errorf("db_url must be set for postgres driver")
	}
	appEnv := strings.ToLower(strings.TrimSpace(cfg.AppEnv))
	csrk := strings.TrimSpace(cfg.CSRFKey)
	pep := strings.TrimSpace(cfg.Pepper)
	docKey := strings.TrimSpace(cfg.Docs.EncryptionKey)
	if csrk == "" || pep == "" || docKey == "" {
		return fmt.Errorf("csrf_key, pepper, and docs.encryption_key must be set via env")
	}
	if cfg.IsHomeMode() {
		if appEnv != "dev" {
			key := strings.TrimSpace(cfg.Backups.EncryptionKey)
			if key == "" {
				return fmt.Errorf("backups.encryption_key must be set outside APP_ENV=dev")
			}
			if len(key) < 32 {
				return fmt.Errorf("backups.encryption_key must be at least 32 characters")
			}
		}
		return nil
	}
	if appEnv != "dev" {
		if isDefaultSecret(csrk) || isDefaultSecret(pep) || isDefaultSecret(docKey) {
			return fmt.Errorf("default secrets are not allowed outside APP_ENV=dev")
		}
		if !cfg.TLSEnabled {
			return fmt.Errorf("tls_enabled=false is only allowed in APP_ENV=dev")
		}
		key := strings.TrimSpace(cfg.Backups.EncryptionKey)
		if key == "" {
			return fmt.Errorf("backups.encryption_key must be set outside APP_ENV=dev")
		}
		if len(key) < 32 {
			return fmt.Errorf("backups.encryption_key must be at least 32 characters")
		}
	}
	return nil
}

func isDefaultSecret(val string) bool {
	switch val {
	case defaultCSRFKey, defaultPepper, defaultDocsEncryptionKey:
		return true
	default:
		return false
	}
}
