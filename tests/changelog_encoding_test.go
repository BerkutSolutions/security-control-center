package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChangelog_NoMojibakeArtifacts(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "CHANGELOG.md"))
	if err != nil {
		t.Fatalf("read changelog: %v", err)
	}
	text := string(raw)

	badMarkers := []string{
		"РѕСЂРёРЅРі",
		"Р–СѓСЂРЅР°Р»",
		"вЂ”",
	}

	var hits []string
	for _, m := range badMarkers {
		if strings.Contains(text, m) {
			hits = append(hits, m)
		}
	}
	if len(hits) > 0 {
		t.Fatalf("changelog contains mojibake markers: %v", hits)
	}
}

