package api

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAppCompatRouteRequiresAppViewPermission(t *testing.T) {
	root := projectRoot(t)
	path := filepath.Join(root, "api", "routes_shell_core.go")
	lines := readLines(t, path)

	found := false
	for i, line := range lines {
		if !strings.Contains(line, "apiRouter.MethodFunc(") || !strings.Contains(line, "\"/app/compat\"") {
			continue
		}
		found = true
		if !strings.Contains(line, "s.withSession(") {
			t.Fatalf("missing session guard in %s:%d -> %s", path, i+1, strings.TrimSpace(line))
		}
		if !strings.Contains(line, "requirePermission(\"app.compat.view\")") {
			t.Fatalf("missing app.compat.view permission guard in %s:%d -> %s", path, i+1, strings.TrimSpace(line))
		}
	}
	if !found {
		t.Fatalf("route /app/compat not found in %s", path)
	}
}
