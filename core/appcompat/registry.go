package appcompat

type Status string

const (
	StatusOK            Status = "ok"
	StatusNeedsAttention Status = "needs_attention"
	StatusNeedsReinit   Status = "needs_reinit"
	StatusBroken        Status = "broken"
)

type ModuleSpec struct {
	ModuleID string

	// Stable i18n keys for UI rendering.
	TitleI18nKey   string
	DetailsI18nKey string

	ExpectedSchemaVersion   int
	ExpectedBehaviorVersion int

	HasPartialAdapt bool
	HasFullReset    bool

	DangerLevel string // "low" | "medium" | "high" | "critical"
}

func DefaultRegistry() []ModuleSpec {
	// Stage 1 baseline: expected versions are 0, so nothing prompts users until a module bumps its expected version.
	return []ModuleSpec{
		{ModuleID: "dashboard", TitleI18nKey: "compat.module.dashboard", DetailsI18nKey: "compat.details.dashboard", ExpectedSchemaVersion: 0, ExpectedBehaviorVersion: 0, HasPartialAdapt: true, HasFullReset: true, DangerLevel: "low"},
		{ModuleID: "tasks", TitleI18nKey: "compat.module.tasks", DetailsI18nKey: "compat.details.tasks", ExpectedSchemaVersion: 0, ExpectedBehaviorVersion: 0, HasPartialAdapt: true, HasFullReset: true, DangerLevel: "high"},
		{ModuleID: "monitoring", TitleI18nKey: "compat.module.monitoring", DetailsI18nKey: "compat.details.monitoring", ExpectedSchemaVersion: 0, ExpectedBehaviorVersion: 0, HasPartialAdapt: true, HasFullReset: true, DangerLevel: "medium"},
		{ModuleID: "docs", TitleI18nKey: "compat.module.docs", DetailsI18nKey: "compat.details.docs", ExpectedSchemaVersion: 0, ExpectedBehaviorVersion: 0, HasPartialAdapt: true, HasFullReset: true, DangerLevel: "critical"},
		{ModuleID: "approvals", TitleI18nKey: "compat.module.approvals", DetailsI18nKey: "compat.details.approvals", ExpectedSchemaVersion: 0, ExpectedBehaviorVersion: 0, HasPartialAdapt: true, HasFullReset: true, DangerLevel: "high"},
		{ModuleID: "incidents", TitleI18nKey: "compat.module.incidents", DetailsI18nKey: "compat.details.incidents", ExpectedSchemaVersion: 0, ExpectedBehaviorVersion: 0, HasPartialAdapt: true, HasFullReset: true, DangerLevel: "critical"},
		{ModuleID: "registry.controls", TitleI18nKey: "compat.module.registryControls", DetailsI18nKey: "compat.details.registryControls", ExpectedSchemaVersion: 0, ExpectedBehaviorVersion: 0, HasPartialAdapt: true, HasFullReset: true, DangerLevel: "high"},
		{ModuleID: "registry.assets", TitleI18nKey: "compat.module.registryAssets", DetailsI18nKey: "compat.details.registryAssets", ExpectedSchemaVersion: 0, ExpectedBehaviorVersion: 0, HasPartialAdapt: true, HasFullReset: true, DangerLevel: "high"},
		{ModuleID: "registry.software", TitleI18nKey: "compat.module.registrySoftware", DetailsI18nKey: "compat.details.registrySoftware", ExpectedSchemaVersion: 0, ExpectedBehaviorVersion: 0, HasPartialAdapt: true, HasFullReset: true, DangerLevel: "high"},
		{ModuleID: "registry.findings", TitleI18nKey: "compat.module.registryFindings", DetailsI18nKey: "compat.details.registryFindings", ExpectedSchemaVersion: 0, ExpectedBehaviorVersion: 0, HasPartialAdapt: true, HasFullReset: true, DangerLevel: "high"},
		{ModuleID: "reports", TitleI18nKey: "compat.module.reports", DetailsI18nKey: "compat.details.reports", ExpectedSchemaVersion: 0, ExpectedBehaviorVersion: 0, HasPartialAdapt: true, HasFullReset: true, DangerLevel: "high"},
		{ModuleID: "accounts", TitleI18nKey: "compat.module.accounts", DetailsI18nKey: "compat.details.accounts", ExpectedSchemaVersion: 0, ExpectedBehaviorVersion: 0, HasPartialAdapt: true, HasFullReset: false, DangerLevel: "critical"},
		{ModuleID: "settings", TitleI18nKey: "compat.module.settings", DetailsI18nKey: "compat.details.settings", ExpectedSchemaVersion: 0, ExpectedBehaviorVersion: 0, HasPartialAdapt: true, HasFullReset: true, DangerLevel: "medium"},
		{ModuleID: "backups", TitleI18nKey: "compat.module.backups", DetailsI18nKey: "compat.details.backups", ExpectedSchemaVersion: 0, ExpectedBehaviorVersion: 0, HasPartialAdapt: true, HasFullReset: true, DangerLevel: "high"},
		{ModuleID: "logs", TitleI18nKey: "compat.module.logs", DetailsI18nKey: "compat.details.logs", ExpectedSchemaVersion: 0, ExpectedBehaviorVersion: 0, HasPartialAdapt: true, HasFullReset: true, DangerLevel: "critical"},
	}
}

