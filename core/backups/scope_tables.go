package backups

import (
	"sort"
	"strings"
)

var scopeTables = map[string][]string{
	"docs": {
		"entity_links",
		"doc_acl",
		"doc_versions",
		"docs_fts",
		"docs",
		"folder_acl",
		"doc_templates",
		"doc_folders",
		"doc_reg_counters",
	},
	"tasks": {
		"task_tag_links",
		"task_files",
		"task_blocks",
		"task_comments",
		"task_assignments",
		"task_archive_entries",
		"tasks",
		"task_subcolumns",
		"task_columns",
		"task_board_acl",
		"task_recurring_instances",
		"task_recurring_rules",
		"task_templates",
		"task_board_layouts",
		"task_boards",
		"task_space_acl",
		"task_spaces",
		"task_tags",
	},
	"incidents": {
		"incident_artifact_files",
		"incident_timeline",
		"incident_attachments",
		"incident_links",
		"incident_acl",
		"incident_stage_entries",
		"incident_stages",
		"incident_participants",
		"incident_reg_counters",
		"incidents",
	},
	"reports": {
		"report_snapshot_items",
		"report_snapshots",
		"report_charts",
		"report_sections",
		"report_meta",
		"docs_fts",
		"entity_links",
		"doc_acl",
		"doc_versions",
		"docs",
		"report_templates",
		"report_settings",
	},
	"monitoring": {
		"monitor_notification_deliveries",
		"monitor_notifications",
		"monitor_notification_state",
		"notification_channels",
		"monitor_maintenance",
		"monitor_tls",
		"monitor_events",
		"monitor_metrics",
		"monitor_assets",
		"monitor_state",
		"monitor_sla_period_results",
		"monitor_sla_policies",
		"monitors",
		"monitoring_settings",
	},
	"controls": {
		"control_framework_map",
		"control_framework_items",
		"control_frameworks",
		"control_violations",
		"control_comments",
		"control_checks",
		"controls",
		"control_types",
	},
	"approvals": {
		"approval_comments",
		"approval_participants",
		"doc_export_approvals",
		"approvals",
	},
	"accounts": {
		"user_role_links",
		"group_user_links",
		"group_role_links",
		"role_permissions",
		"sessions",
		"users",
		"groups",
		"roles",
	},
}

func backupScopeIsAll(scope []string) bool {
	norm := normalizedBackupScope(scope)
	return len(norm) == 1 && norm[0] == "ALL"
}

func backupTablesForScope(scope []string) []string {
	if backupScopeIsAll(scope) {
		return nil
	}
	normalized := normalizedBackupScope(scope)
	seen := map[string]struct{}{}
	out := make([]string, 0, 64)
	for _, item := range normalized {
		key := strings.ToLower(strings.TrimSpace(item))
		tables, ok := scopeTables[key]
		if !ok {
			continue
		}
		for _, table := range tables {
			name := strings.TrimSpace(table)
			if name == "" {
				continue
			}
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	return out
}

func normalizeManifestScope(scope []string) []string {
	norm := normalizedBackupScope(scope)
	if len(norm) == 0 {
		return []string{"ALL"}
	}
	if len(norm) == 1 && strings.EqualFold(norm[0], "ALL") {
		return []string{"ALL"}
	}
	sort.Strings(norm)
	return norm
}
