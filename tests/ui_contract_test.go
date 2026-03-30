package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	reMenuOrderContract = regexp.MustCompile(`MENU_ORDER\s*=\s*\[([^\]]+)\]`)
	reMenuItemContract  = regexp.MustCompile(`'([^']+)'`)
	rePageFileEntry     = regexp.MustCompile(`"([a-z0-9_]+)"\s*:\s*"([a-z0-9_.-]+)"`)
)

func TestUIContract_MenuRoutesTemplatesAndSelectors(t *testing.T) {
	root := filepath.Clean("..")
	appJS := mustReadContract(t, filepath.Join(root, "gui", "static", "js", "app.js"))
	shellRoutes := mustReadContract(t, filepath.Join(root, "api", "routes_shell_tabs.go"))
	placeholderHandlers := mustReadContract(t, filepath.Join(root, "api", "handlers", "placeholders_handlers.go"))

	menu := extractMenuOrderContract(t, appJS)
	pageFiles := extractPageFilesContract(t, placeholderHandlers)

	requiredByMenu := map[string]struct {
		file      string
		selectors []string
		i18nAttrs []string
	}{
		"dashboard": {
			file: "dashboard.html",
			selectors: []string{
				`id="dashboard-page"`,
				`id="dashboard-edit-btn"`,
			},
			i18nAttrs: []string{
				`data-i18n="dashboard.edit"`,
			},
		},
		"tasks": {
			file: "tasks.html",
			selectors: []string{
				`id="tasks-page"`,
				`id="tasks-tabs"`,
				`id="tasks-space-create-btn"`,
			},
			i18nAttrs: []string{
				`data-i18n="tasks.actions.createSpace"`,
			},
		},
		"monitoring": {
			file: "monitoring.html",
			selectors: []string{
				`id="monitoring-page"`,
				`id="monitoring-tabs"`,
				`id="monitor-new-btn"`,
			},
			i18nAttrs: []string{
				`data-i18n="monitoring.newMonitor"`,
			},
		},
		"notifications": {
			file: "notifications.html",
			selectors: []string{
				`id="notifications-page"`,
				`id="notifications-tabs"`,
				`id="notifications-channel-new"`,
				`id="notifications-settings-save"`,
			},
			i18nAttrs: []string{
				`data-i18n="notifications.settings.title"`,
			},
		},
		"docs": {
			file: "docs.html",
			selectors: []string{
				`id="docs-page"`,
				`id="btn-new-doc"`,
				`id="docs-search"`,
			},
			i18nAttrs: []string{
				`data-i18n="docs.newMd"`,
			},
		},
		"approvals": {
			file: "approvals.html",
			selectors: []string{
				`id="approvals-page"`,
				`id="approvals-status"`,
				`id="approvals-refresh"`,
			},
			i18nAttrs: []string{
				`data-i18n="approvals.filter.status"`,
			},
		},
		"incidents": {
			file: "incidents.html",
			selectors: []string{
				`id="incidents-page"`,
				`id="incidents-tabs"`,
			},
			i18nAttrs: []string{
				`data-i18n="incidents.stage.addTitle"`,
			},
		},
		"registry": {
			file: "controls.html",
			selectors: []string{
				`id="controls-page"`,
				`id="controls-tabs"`,
				`id="controls-create-btn"`,
			},
			i18nAttrs: []string{
				`data-i18n="controls.tabs.controls"`,
			},
		},
		"reports": {
			file: "reports.html",
			selectors: []string{
				`id="reports-page"`,
				`id="reports-tabs"`,
				`id="reports-new-btn"`,
			},
			i18nAttrs: []string{
				`data-i18n="reports.new"`,
			},
		},
		"accounts": {
			file: "accounts.html",
			selectors: []string{
				`id="accounts-page"`,
				`id="accounts-dashboard"`,
				`id="open-group-create"`,
			},
			i18nAttrs: []string{
				`data-i18n="accounts.groups.create"`,
			},
		},
		"accesses": {
			file: "accesses.html",
			selectors: []string{
				`id="accesses-page"`,
				`id="accesses-open-create"`,
				`id="accesses-service-filter"`,
			},
			i18nAttrs: []string{
				`data-i18n="accesses.filters.service"`,
			},
		},
		"settings": {
			file: "settings.html",
			selectors: []string{
				`id="settings-page"`,
				`id="settings-tabs"`,
				`id="settings-alert"`,
			},
			i18nAttrs: []string{
				`data-i18n="settings.tabs.general"`,
			},
		},
		"backups": {
			file: "backups.html",
			selectors: []string{
				`id="backups-page"`,
				`id="backups-tabs"`,
				`id="backups-create-now"`,
			},
			i18nAttrs: []string{
				`data-i18n="backups.actions.createNow"`,
			},
		},
		"logs": {
			file: "logs.html",
			selectors: []string{
				`id="logs-page"`,
				`id="logs-table"`,
				`id="logs-refresh"`,
			},
			i18nAttrs: []string{
				`data-i18n="logs.table.time"`,
			},
		},
	}

	for _, tab := range menu {
		contract, ok := requiredByMenu[tab]
		if !ok {
			t.Fatalf("menu tab %q has no UI contract mapping in tests/ui_contract_test.go", tab)
		}

		routeNeedle := `"` + "/" + tab + `"`
		if !strings.Contains(shellRoutes, routeNeedle) {
			t.Fatalf("missing shell route for menu tab %q in api/routes_shell_tabs.go", tab)
		}

		if tab != "dashboard" && tab != "settings" && tab != "accounts" {
			pageFile, ok := pageFiles[tab]
			if !ok {
				t.Fatalf("placeholder page file is not registered for tab %q", tab)
			}
			if pageFile != contract.file {
				t.Fatalf("unexpected placeholder page file for tab %q: got %q want %q", tab, pageFile, contract.file)
			}
		}

		pageContent := mustReadContract(t, filepath.Join(root, "gui", "static", contract.file))
		for _, sel := range contract.selectors {
			if !strings.Contains(pageContent, sel) {
				t.Fatalf("ui selector contract broken for tab %q in %s: missing %s", tab, contract.file, sel)
			}
		}
		for _, key := range contract.i18nAttrs {
			if !strings.Contains(pageContent, key) {
				t.Fatalf("ui i18n marker contract broken for tab %q in %s: missing %s", tab, contract.file, key)
			}
		}
	}
}

func mustReadContract(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

func extractMenuOrderContract(t *testing.T, appJS string) []string {
	t.Helper()
	m := reMenuOrderContract.FindStringSubmatch(appJS)
	if len(m) < 2 {
		t.Fatalf("MENU_ORDER not found in gui/static/js/app.js")
	}
	items := reMenuItemContract.FindAllStringSubmatch(m[1], -1)
	if len(items) == 0 {
		t.Fatalf("MENU_ORDER is empty in gui/static/js/app.js")
	}
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, it := range items {
		if len(it) < 2 {
			continue
		}
		v := strings.TrimSpace(it[1])
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func extractPageFilesContract(t *testing.T, handlersSrc string) map[string]string {
	t.Helper()
	start := strings.Index(handlersSrc, "var pageFiles = map[string]string{")
	if start == -1 {
		t.Fatalf("pageFiles map not found in api/handlers/placeholders_handlers.go")
	}
	end := strings.Index(handlersSrc[start:], "}")
	if end == -1 {
		t.Fatalf("pageFiles map closing brace not found in api/handlers/placeholders_handlers.go")
	}
	chunk := handlersSrc[start : start+end+1]
	out := map[string]string{}
	for _, m := range rePageFileEntry.FindAllStringSubmatch(chunk, -1) {
		if len(m) < 3 {
			continue
		}
		out[m[1]] = m[2]
	}
	if len(out) == 0 {
		t.Fatalf("pageFiles map parsed empty from api/handlers/placeholders_handlers.go")
	}
	return out
}

