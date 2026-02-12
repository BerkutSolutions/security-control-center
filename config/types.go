package config

import "time"

type AppConfig struct {
	DBPath     string        `yaml:"db_path"`
	ListenAddr string        `yaml:"listen_addr"`
	SessionTTL time.Duration `yaml:"session_ttl"`
	AppEnv     string        `yaml:"app_env"`
	DeploymentMode string    `yaml:"deployment_mode"`
	CSRFKey    string        `yaml:"csrf_key"`
	Pepper     string        `yaml:"pepper"`
	TLSEnabled bool          `yaml:"tls_enabled"`
	TLSCert    string        `yaml:"tls_cert"`
	TLSKey     string        `yaml:"tls_key"`
	Scheduler  SchedulerConfig `yaml:"scheduler"`
	Docs       DocsConfig    `yaml:"docs"`
	Security   SecurityConfig `yaml:"security"`
	Incidents  IncidentsConfig `yaml:"incidents"`
}

func (c *AppConfig) IsHomeMode() bool {
	if c == nil {
		return false
	}
	return c.DeploymentMode == "home"
}

type DocsConfig struct {
	StoragePath        string            `yaml:"storage_path"`
	StorageDir         string            `yaml:"storage_dir"`
	EncryptionKey      string            `yaml:"encryption_key"`
	EncryptionKeyID    string            `yaml:"encryption_key_id"`
	RegTemplate        string            `yaml:"reg_template"`
	PerFolderSequence  bool              `yaml:"per_folder_sequence"`
	VersionLimit       int               `yaml:"version_limit"`
	Watermark          WatermarkConfig   `yaml:"watermark"`
	Converters         ConvertersConfig  `yaml:"converters"`
	AllowDowngrade     bool              `yaml:"allow_downgrade"`
	WatermarkMinLevel  string            `yaml:"watermark_min_level"` // deprecated; kept for compatibility
	ClassificationTags map[string]string `yaml:"classification_tags"` // optional mapping of tag codes to descriptions
}

type WatermarkConfig struct {
	Enabled      bool   `yaml:"enabled"`
	MinLevel     string `yaml:"min_level"`
	TextTemplate string `yaml:"text_template"`
	Placement    string `yaml:"placement"`
}

type ConvertersConfig struct {
	Enabled     bool   `yaml:"enabled"`
	PandocPath  string `yaml:"pandoc_path"`
	SofficePath string `yaml:"soffice_path"`
	TimeoutSec  int    `yaml:"timeout_sec"`
	TempDir     string `yaml:"temp_dir"`
}

type SecurityConfig struct {
	TagsSubsetEnforced bool `yaml:"tags_subset_enforced"`
	OnlineWindowSec    int  `yaml:"online_window_sec"`
	LegacyImportEnabled bool `yaml:"legacy_import_enabled"`
	TrustedProxies     []string `yaml:"trusted_proxies"`
}

type IncidentsConfig struct {
	RegNoFormat string `yaml:"reg_no_format"`
	StorageDir  string `yaml:"storage_dir"`
	TimelineExportLimit int `yaml:"timeline_export_limit"`
}

type SchedulerConfig struct {
	Enabled        bool `yaml:"enabled"`
	IntervalSeconds int `yaml:"interval_seconds"`
	MaxJobsPerTick int `yaml:"max_jobs_per_tick"`
}
