package api

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAppJobsRoutesHaveSessionAndPermissionGuards(t *testing.T) {
	root := projectRoot(t)
	path := filepath.Join(root, "api", "routes_shell_core.go")
	lines := readLines(t, path)

	cases := []struct {
		substr string
		perm   string
	}{
		{substr: "\"POST\", \"/app/jobs\"", perm: "requirePermission(\"app.compat.manage.partial\")"},
		{substr: "\"GET\", \"/app/jobs\"", perm: "requirePermission(\"app.compat.view\")"},
		{substr: "\"GET\", \"/app/jobs/{id}\"", perm: "requirePermission(\"app.compat.view\")"},
		{substr: "\"POST\", \"/app/jobs/{id}/cancel\"", perm: "requirePermission(\"app.compat.manage.partial\")"},
	}

	for _, tc := range cases {
		found := false
		for i, line := range lines {
			if !strings.Contains(line, "apiRouter.MethodFunc(") {
				continue
			}
			if !strings.Contains(line, tc.substr) {
				continue
			}
			found = true
			if !strings.Contains(line, "s.withSession(") {
				t.Fatalf("missing session guard in %s:%d -> %s", path, i+1, strings.TrimSpace(line))
			}
			if !strings.Contains(line, tc.perm) {
				t.Fatalf("missing perm guard %s in %s:%d -> %s", tc.perm, path, i+1, strings.TrimSpace(line))
			}
		}
		if !found {
			t.Fatalf("route not found (%s) in %s", tc.substr, path)
		}
	}
}
