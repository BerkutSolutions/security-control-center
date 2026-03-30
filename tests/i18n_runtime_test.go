package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	reMenuOrderRuntime = regexp.MustCompile(`MENU_ORDER\s*=\s*\[([^\]]+)\]`)
	reMenuItemRuntime  = regexp.MustCompile(`'([^']+)'`)
	reDescReturnKey    = regexp.MustCompile(`return\s+BerkutI18n\.t\('([^']+)'\);`)
)

func TestI18NRuntime_MenuAndPageKeysResolveForRUAndEN(t *testing.T) {
	ru := mustLoadLang(t, filepath.Join("..", "gui", "static", "i18n", "ru.json"))
	en := mustLoadLang(t, filepath.Join("..", "gui", "static", "i18n", "en.json"))

	appJS := mustReadFileRuntime(t, filepath.Join("..", "gui", "static", "js", "app.js"))
	menu := extractRuntimeMenuItems(t, appJS)

	for _, item := range menu {
		key := "nav." + item
		if item == "registry" {
			key = "nav.controls"
		}
		assertRuntimeKeyResolved(t, "ru", key, ru)
		assertRuntimeKeyResolved(t, "en", key, en)
	}

	// Registry title uses nav.controls explicitly.
	assertRuntimeKeyResolved(t, "ru", "nav.controls", ru)
	assertRuntimeKeyResolved(t, "en", "nav.controls", en)

	// Runtime fallback-sensitive keys used by app shell controls.
	for _, key := range []string{
		"common.confirm",
		"common.cancel",
		"common.accessDenied",
		"settings.updates.available",
		"profile.sessionStarted",
		"profile.sessionExpires",
		"auth.stepup.lockedFor",
		"auth.stepup.secondFactor",
		"auth.stepup.subtitle",
	} {
		assertRuntimeKeyResolved(t, "ru", key, ru)
		assertRuntimeKeyResolved(t, "en", key, en)
	}

	// Page subtitle keys from descriptionFor(path) must always resolve.
	for _, key := range extractDescriptionKeys(t, appJS) {
		assertRuntimeKeyResolved(t, "ru", key, ru)
		assertRuntimeKeyResolved(t, "en", key, en)
	}
}

func TestI18NRuntime_LanguageSwitchReloadPathExists(t *testing.T) {
	appJS := mustReadFileRuntime(t, filepath.Join("..", "gui", "static", "js", "app.js"))
	required := []string{
		"async function handlePreferencesChange(",
		"await BerkutI18n.load(",
		"BerkutI18n.apply();",
		"renderMenu(menu, currentPath);",
		"await loadPage(currentPath);",
	}
	for _, marker := range required {
		if !strings.Contains(appJS, marker) {
			t.Fatalf("missing i18n runtime switch marker in app.js: %q", marker)
		}
	}
}

func mustReadFileRuntime(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

func extractRuntimeMenuItems(t *testing.T, appJS string) []string {
	t.Helper()
	m := reMenuOrderRuntime.FindStringSubmatch(appJS)
	if len(m) < 2 {
		t.Fatalf("MENU_ORDER not found in app.js")
	}
	matches := reMenuItemRuntime.FindAllStringSubmatch(m[1], -1)
	if len(matches) == 0 {
		t.Fatalf("MENU_ORDER is empty in app.js")
	}
	out := make([]string, 0, len(matches))
	seen := map[string]struct{}{}
	for _, v := range matches {
		if len(v) < 2 {
			continue
		}
		key := strings.TrimSpace(v[1])
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func extractDescriptionKeys(t *testing.T, appJS string) []string {
	t.Helper()
	idx := strings.Index(appJS, "function descriptionFor(path)")
	if idx < 0 {
		t.Fatalf("descriptionFor(path) not found in app.js")
	}
	block := appJS[idx:]
	endIdx := strings.Index(block, "function bindProfileShortcut()")
	if endIdx > 0 {
		block = block[:endIdx]
	}
	matches := reDescReturnKey.FindAllStringSubmatch(block, -1)
	if len(matches) == 0 {
		t.Fatalf("no BerkutI18n description keys found in descriptionFor(path)")
	}
	out := make([]string, 0, len(matches))
	seen := map[string]struct{}{}
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		key := strings.TrimSpace(m[1])
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func assertRuntimeKeyResolved(t *testing.T, lang, key string, dict map[string]string) {
	t.Helper()
	val, ok := dict[key]
	if !ok {
		t.Fatalf("%s i18n key missing: %q", lang, key)
	}
	if strings.TrimSpace(val) == "" {
		t.Fatalf("%s i18n key has empty value: %q", lang, key)
	}
	if strings.TrimSpace(val) == key {
		t.Fatalf("%s i18n key resolves to raw key text (runtime risk): %q", lang, key)
	}
}
