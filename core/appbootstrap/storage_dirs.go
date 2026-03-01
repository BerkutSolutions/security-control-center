package appbootstrap

import (
	"os"
	"strings"

	"berkut-scc/config"
	"berkut-scc/core/utils"
)

func ensureStorageDirs(cfg *config.AppConfig, logger *utils.Logger) error {
	if cfg == nil {
		return nil
	}
	type item struct {
		name string
		path string
	}
	items := []item{
		{name: "docs", path: cfg.Docs.StorageDir},
		{name: "incidents", path: cfg.Incidents.StorageDir},
		{name: "backups", path: cfg.Backups.Path},
	}
	for _, it := range items {
		p := strings.TrimSpace(it.path)
		if p == "" {
			continue
		}
		if err := os.MkdirAll(p, 0o700); err != nil {
			if logger != nil {
				logger.Errorf("storage dir init failed name=%s path=%s: %v", it.name, p, err)
			}
			return err
		}
	}
	return nil
}
