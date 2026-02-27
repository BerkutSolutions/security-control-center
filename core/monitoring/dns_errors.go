package monitoring

import "strings"

// isDNSErrorText tries to detect name resolution problems across common runtimes.
// We intentionally keep it strict to avoid classifying application-level DNS checks
// like "monitoring.error.dnsNoAnswer" as a DNS outage.
func isDNSErrorText(errText string) bool {
	msg := strings.ToLower(strings.TrimSpace(errText))
	if msg == "" {
		return false
	}
	// Don't treat internal i18n keys as DNS failures.
	if strings.HasPrefix(msg, "monitoring.error.") {
		return false
	}
	// Go / net errors.
	if strings.Contains(msg, "no such host") {
		return true
	}
	if strings.Contains(msg, "temporary failure in name resolution") {
		return true
	}
	if strings.Contains(msg, "server misbehaving") {
		return true
	}
	if strings.Contains(msg, "nxdomain") {
		return true
	}
	if strings.Contains(msg, "servfail") {
		return true
	}
	// Node / getaddrinfo errors.
	if strings.Contains(msg, "getaddrinfo") && strings.Contains(msg, "enotfound") {
		return true
	}
	if strings.Contains(msg, "enotfound") {
		return true
	}
	// Some systems return "name or service not known".
	if strings.Contains(msg, "name or service not known") {
		return true
	}
	return false
}

