package format

import "time"

type Manifest struct {
	FormatVersion  string `json:"format_version"`
	CreatedAt      string `json:"created_at"`
	AppVersion     string `json:"app_version"`
	DBEngine       string `json:"db_engine"`
	GooseDBVersion int64  `json:"goose_db_version"`
	IncludesFiles  bool   `json:"includes_files"`
	Notes          string `json:"notes,omitempty"`
}

func NewManifest(appVersion string, gooseVersion int64, now time.Time) Manifest {
	return Manifest{
		FormatVersion:  "1",
		CreatedAt:      now.UTC().Format(time.RFC3339),
		AppVersion:     appVersion,
		DBEngine:       "postgres",
		GooseDBVersion: gooseVersion,
		IncludesFiles:  false,
	}
}
