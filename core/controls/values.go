package controls

import "strings"

const (
	ControlTypeOrganizational = "organizational"
	ControlTypeTechnical      = "technical"
	ControlTypeProcedural     = "procedural"

	StatusImplemented    = "implemented"
	StatusPartial        = "partial"
	StatusNotImplemented = "not_implemented"
	StatusNotApplicable  = "not_applicable"

	RiskLow      = "low"
	RiskMedium   = "medium"
	RiskHigh     = "high"
	RiskCritical = "critical"

	FrequencyManual     = "manual"
	FrequencyDaily      = "daily"
	FrequencyWeekly     = "weekly"
	FrequencyMonthly    = "monthly"
	FrequencyQuarterly  = "quarterly"
	FrequencySemiannual = "semiannual"
	FrequencyAnnual     = "annual"

	CheckPass          = "pass"
	CheckPartial       = "partial"
	CheckFail          = "fail"
	CheckNotApplicable = "not_applicable"
)

var (
	ControlTypes = []string{ControlTypeOrganizational, ControlTypeTechnical, ControlTypeProcedural}
	Statuses     = []string{StatusImplemented, StatusPartial, StatusNotImplemented, StatusNotApplicable}
	RiskLevels   = []string{RiskLow, RiskMedium, RiskHigh, RiskCritical}
	Frequencies  = []string{FrequencyManual, FrequencyDaily, FrequencyWeekly, FrequencyMonthly, FrequencyQuarterly, FrequencySemiannual, FrequencyAnnual}
	CheckResults = []string{CheckPass, CheckPartial, CheckFail, CheckNotApplicable}
)

func ViolationSeverities() []string {
	return RiskLevels
}

func NormalizeInList(value string, list []string) (string, bool) {
	val := strings.ToLower(strings.TrimSpace(value))
	if val == "" {
		return "", false
	}
	for _, item := range list {
		if val == item {
			return val, true
		}
	}
	return val, false
}
