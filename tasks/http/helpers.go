package taskshttp

import "strings"

func sameTemplateID(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func attachmentDisposition(filename string) string {
	return `attachment; filename="` + sanitizeHeaderFilename(filename) + `"`
}

func sanitizeHeaderFilename(name string) string {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return "file"
	}
	replacer := strings.NewReplacer(
		"\r", "",
		"\n", "",
		"\"", "",
		";", "_",
		"/", "_",
		"\\", "_",
	)
	clean = replacer.Replace(clean)
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return "file"
	}
	return clean
}
