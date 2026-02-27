package tests

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode"
)

var (
	reManyQuestionMarks = regexp.MustCompile(`\?{3,}`)
	reHasCyrillic       = regexp.MustCompile(`[\p{Cyrillic}]`)
	reHasLatin          = regexp.MustCompile(`[A-Za-z]`)
)

// TestI18N_NoArtifacts checks for obvious i18n text corruption artifacts:
// - Unicode replacement char (�)
// - control characters
// - "???" sequences
// - common mojibake characters (e.g. CP1251/UTF-8 mismatch) in RU strings
func TestI18N_NoArtifacts(t *testing.T) {
	ru := mustLoadLang(t, filepath.Join("..", "gui", "static", "i18n", "ru.json"))
	en := mustLoadLang(t, filepath.Join("..", "gui", "static", "i18n", "en.json"))

	var issues []string
	check := func(lang, key, value string) {
		if strings.ContainsRune(value, unicode.ReplacementChar) || strings.Contains(value, "\uFFFD") {
			issues = append(issues, lang+":"+key+": contains replacement char (�)")
		}
		if hasSuspiciousControlChars(value) {
			issues = append(issues, lang+":"+key+": contains control chars")
		}
		if reManyQuestionMarks.MatchString(value) {
			issues = append(issues, lang+":"+key+": contains \"???\"-like sequence")
		}

		// Common mojibake signatures when UTF-8 bytes are interpreted with a wrong encoding.
		// For RU locale these are almost always a bug (e.g. "РџСЂРё..." instead of "При...").
		if lang == "ru" && containsForbiddenRuCyrillic(value) {
			issues = append(issues, lang+":"+key+": contains suspicious Cyrillic letters (likely mojibake)")
		}

		// EN strings should not contain Cyrillic at all.
		if lang == "en" && reHasCyrillic.MatchString(value) {
			issues = append(issues, lang+":"+key+": contains Cyrillic characters")
		}
	}

	for k, v := range ru {
		check("ru", k, v)
	}
	for k, v := range en {
		check("en", k, v)
	}

	if len(issues) > 0 {
		sort.Strings(issues)
		t.Fatalf("i18n artifacts found: %v", sample(issues))
	}
}

// TestI18N_NoNewUntranslatedPhrases tries to detect obviously-untranslated phrases and prevents regressions.
// It allows the current known set (to keep the suite green) and fails only if *new* issues appear.
func TestI18N_NoNewUntranslatedPhrases(t *testing.T) {
	ru := mustLoadLang(t, filepath.Join("..", "gui", "static", "i18n", "ru.json"))
	en := mustLoadLang(t, filepath.Join("..", "gui", "static", "i18n", "en.json"))

	knownRU := map[string]struct{}{
		"docs.tag.pci_dss":                        {},
		"login.title":                             {},
		"monitoring.notifications.chat":           {},
		"monitoring.notifications.thread":         {},
		"monitoring.notify.footer":                {},
		"monitoring.placeholder.headers":          {},
		"monitoring.type.httpJson":                {},
		"monitoring.type.kafkaProducer":           {},
		"monitoring.type.mssql":                   {},
		"reports.sections.filters.eventsLimit":    {},
		"reports.sections.filters.includeCurrent": {},
		"reports.sections.filters.onlyViolations": {},
		"reports.sections.filters.slaPeriod":      {},
		"reports.sections.slaSummary":             {},
		"settings.about.name":                     {},
		"settings.about.profileLink":              {},
		"settings.https.mode.builtin":             {},
		"settings.https.mode.proxy":               {},
		"settings.https.proxy.nginx":              {},
		"settings.https.proxy.traefik":            {},
		"settings.https.proxy.traefikNginx":       {},
		"settings.https.trustedProxies":           {},
	}

	var newIssues []string
	for k, v := range ru {
		if looksUntranslatedForRU(v) {
			if _, ok := knownRU[k]; !ok {
				newIssues = append(newIssues, "ru:"+k)
			}
		}
	}
	for k, v := range en {
		if looksUntranslatedForEN(v) {
			newIssues = append(newIssues, "en:"+k)
		}
	}

	if len(newIssues) > 0 {
		sort.Strings(newIssues)
		t.Fatalf("new untranslated i18n phrases detected (update translations or extend allowlist): %v", sample(newIssues))
	}
}

func looksUntranslatedForRU(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	if reHasCyrillic.MatchString(value) {
		return false
	}
	if !reHasLatin.MatchString(value) {
		return false
	}
	clean := stripI18NPlaceholders(value)
	clean = strings.TrimSpace(clean)
	return strings.Contains(clean, " ")
}

func looksUntranslatedForEN(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	// If EN contains Cyrillic, it's almost always a missed translation.
	return reHasCyrillic.MatchString(value)
}

func stripI18NPlaceholders(value string) string {
	// Reuse placeholder regexes from i18n_keys_test.go.
	out := reNamedPlaceholder.ReplaceAllString(value, "")
	out = rePrintfPlaceholder.ReplaceAllString(out, "")
	return out
}

func hasSuspiciousControlChars(s string) bool {
	for _, r := range s {
		if !unicode.IsControl(r) {
			continue
		}
		// Allow common whitespace controls.
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		return true
	}
	return false
}

func containsForbiddenRuCyrillic(s string) bool {
	// These letters are not used in Russian and are common in mojibake like "РџСЂРё...".
	// (Russian typically uses: А-Я, а-я, Ё/ё, plus basic punctuation/quotes.)
	forbidden := map[rune]struct{}{
		'Ѐ': {}, 'ѐ': {},
		'Ѓ': {}, 'ѓ': {},
		'Є': {}, 'є': {},
		'Ѕ': {}, 'ѕ': {},
		'І': {}, 'і': {},
		'Ї': {}, 'ї': {},
		'Ј': {}, 'ј': {},
		'Љ': {}, 'љ': {},
		'Њ': {}, 'њ': {},
		'Ћ': {}, 'ћ': {},
		'Ќ': {}, 'ќ': {},
		'Ѝ': {}, 'ѝ': {},
		'Ў': {}, 'ў': {},
		'Џ': {}, 'џ': {},
		'Ґ': {}, 'ґ': {},
	}
	for _, r := range s {
		if _, ok := forbidden[r]; ok {
			return true
		}
	}
	return false
}
