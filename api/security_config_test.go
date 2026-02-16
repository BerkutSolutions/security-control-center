package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvExampleTrustedProxiesDefaultsAreEmpty(t *testing.T) {
	root := projectRoot(t)
	path := filepath.Join(root, ".env.example")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(raw)
	if !strings.Contains(text, "BERKUT_SECURITY_TRUSTED_PROXIES=") {
		t.Fatalf("missing BERKUT_SECURITY_TRUSTED_PROXIES setting in %s", path)
	}
	if !strings.Contains(text, "HTTPS_TRUSTED_PROXIES=") {
		t.Fatalf("missing HTTPS_TRUSTED_PROXIES setting in %s", path)
	}
	if strings.Contains(text, "BERKUT_SECURITY_TRUSTED_PROXIES=127.0.0.1,10.0.0.0/8") {
		t.Fatalf("broad trusted proxies default is not allowed in %s", path)
	}
	if strings.Contains(text, "HTTPS_TRUSTED_PROXIES=127.0.0.1,10.0.0.0/8") {
		t.Fatalf("broad https trusted proxies default is not allowed in %s", path)
	}
}

func TestDockerComposeHTTPSUsesForwardedScheme(t *testing.T) {
	root := projectRoot(t)
	path := filepath.Join(root, "docs", "ru", "docker-compose.https.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(raw)
	if !strings.Contains(text, "proxy_set_header X-Forwarded-Proto $$scheme;") {
		t.Fatalf("expected X-Forwarded-Proto $$scheme in %s", path)
	}
	if strings.Contains(text, "proxy_set_header X-Forwarded-Proto http;") {
		t.Fatalf("legacy hardcoded X-Forwarded-Proto http found in %s", path)
	}
}
