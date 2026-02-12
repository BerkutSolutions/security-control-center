package config

import (
	"fmt"
	"strings"
)

const (
	defaultCSRFKey          = "FWgaRnHOh8Nep_kGLCTiBXIB2k72_G2Ch1Q7HOM0zIo"
	defaultPepper           = "BPY89KfAWweJM5p2Vh0Zwg_-nm7wSlS8La8DxPWFAlg"
	defaultDocsEncryptionKey = "337d0c35c87c92c9dfe05f4251de8ad18fcb363d4be33bcc5394fb013dc22daf"
)

func Validate(cfg *AppConfig) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	appEnv := strings.ToLower(strings.TrimSpace(cfg.AppEnv))
	csrk := strings.TrimSpace(cfg.CSRFKey)
	pep := strings.TrimSpace(cfg.Pepper)
	docKey := strings.TrimSpace(cfg.Docs.EncryptionKey)
	if csrk == "" || pep == "" || docKey == "" {
		return fmt.Errorf("csrf_key, pepper, and docs.encryption_key must be set via env")
	}
	if cfg.IsHomeMode() {
		return nil
	}
	if appEnv != "dev" {
		if isDefaultSecret(csrk) || isDefaultSecret(pep) || isDefaultSecret(docKey) {
			return fmt.Errorf("default secrets are not allowed outside APP_ENV=dev")
		}
		if !cfg.TLSEnabled {
			return fmt.Errorf("tls_enabled=false is only allowed in APP_ENV=dev")
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
