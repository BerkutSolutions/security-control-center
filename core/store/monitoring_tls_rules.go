package store

import "sort"

func DefaultTLSExpiringRules() []TLSExpiringRule {
	return []TLSExpiringRule{
		{Days: 30, Enabled: true},
		{Days: 14, Enabled: true},
		{Days: 7, Enabled: true},
	}
}

func NormalizeTLSExpiringRules(input []TLSExpiringRule, fallbackDays int) []TLSExpiringRule {
	seen := map[int]bool{}
	out := make([]TLSExpiringRule, 0, len(input)+3)
	for _, item := range input {
		if item.Days <= 0 {
			continue
		}
		if _, ok := seen[item.Days]; ok {
			continue
		}
		seen[item.Days] = true
		out = append(out, TLSExpiringRule{Days: item.Days, Enabled: item.Enabled})
	}
	if len(out) == 0 {
		if fallbackDays > 0 {
			out = append(out, TLSExpiringRule{Days: fallbackDays, Enabled: true})
		}
		for _, item := range DefaultTLSExpiringRules() {
			if _, ok := seen[item.Days]; ok {
				continue
			}
			seen[item.Days] = true
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Days > out[j].Days
	})
	return out
}

func EnabledTLSExpiringThresholds(rules []TLSExpiringRule, fallbackDays int) []int {
	normalized := NormalizeTLSExpiringRules(rules, fallbackDays)
	out := make([]int, 0, len(normalized))
	for _, item := range normalized {
		if item.Enabled && item.Days > 0 {
			out = append(out, item.Days)
		}
	}
	sort.Ints(out)
	return out
}

func MaxEnabledTLSExpiringDay(rules []TLSExpiringRule, fallbackDays int) int {
	enabled := EnabledTLSExpiringThresholds(rules, fallbackDays)
	if len(enabled) > 0 {
		return enabled[len(enabled)-1]
	}
	if fallbackDays > 0 {
		return fallbackDays
	}
	return 30
}
