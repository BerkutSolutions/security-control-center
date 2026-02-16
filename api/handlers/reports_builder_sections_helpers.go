package handlers

import (
	"context"
	"fmt"
	"strings"

	"berkut-scc/core/docs"
	"berkut-scc/core/store"
	"berkut-scc/tasks"
)

func (h *ReportsHandler) cachedUserName(cache map[int64]string, id int64) string {
	if id == 0 {
		return "-"
	}
	if val, ok := cache[id]; ok {
		return val
	}
	u, _, _ := h.users.Get(context.Background(), id)
	name := "-"
	if u != nil {
		if strings.TrimSpace(u.FullName) != "" {
			name = u.FullName
		} else {
			name = u.Username
		}
	}
	cache[id] = name
	return name
}

func taskAssignees(assignments []tasks.Assignment, cache map[int64]string, h *ReportsHandler) string {
	if len(assignments) == 0 {
		return "-"
	}
	var names []string
	for _, a := range assignments {
		names = append(names, h.cachedUserName(cache, a.UserID))
	}
	return strings.Join(names, ", ")
}

func assignmentIDs(assignments []tasks.Assignment) []int64 {
	var ids []int64
	for _, a := range assignments {
		ids = append(ids, a.UserID)
	}
	return ids
}

func escapePipes(val string) string {
	return strings.ReplaceAll(val, "|", "\\|")
}

func sectionTitle(sec store.ReportSection, fallback string) string {
	if strings.TrimSpace(sec.Title) != "" {
		return strings.TrimSpace(sec.Title)
	}
	return fallback
}

func (h *ReportsHandler) canViewIncidentByClassification(eff store.EffectiveAccess, level int, tags []string) bool {
	if h.cfg != nil && h.cfg.Security.TagsSubsetEnforced {
		return docs.HasClearance(docs.ClassificationLevel(eff.ClearanceLevel), eff.ClearanceTags, docs.ClassificationLevel(level), tags)
	}
	return eff.ClearanceLevel >= level
}

func taskBoardAllowed(user *store.User, roles []string, groups []store.Group, spaceACL []tasks.ACLRule, boardACL []tasks.ACLRule, required string) bool {
	if len(spaceACL) > 0 {
		return taskACLAllowed(user, roles, groups, spaceACL, required)
	}
	return taskACLAllowed(user, roles, groups, boardACL, required)
}

func taskACLAllowed(user *store.User, roles []string, groups []store.Group, acl []tasks.ACLRule, required string) bool {
	if user == nil {
		return false
	}
	req := strings.ToLower(strings.TrimSpace(required))
	if req == "" {
		return false
	}
	allowed := map[string]struct{}{req: {}}
	if req == "view" {
		allowed["manage"] = struct{}{}
	}
	for _, rule := range acl {
		if _, ok := allowed[strings.ToLower(rule.Permission)]; !ok {
			continue
		}
		switch strings.ToLower(rule.SubjectType) {
		case "all":
			return true
		case "user":
			if rule.SubjectID == user.Username || rule.SubjectID == fmt.Sprintf("%d", user.ID) {
				return true
			}
		case "role":
			for _, r := range roles {
				if r == rule.SubjectID {
					return true
				}
			}
		case "department":
			if strings.EqualFold(user.Department, rule.SubjectID) {
				return true
			}
		case "group":
			for _, g := range groups {
				if strings.EqualFold(g.Name, rule.SubjectID) || fmt.Sprintf("%d", g.ID) == rule.SubjectID {
					return true
				}
			}
		}
	}
	return false
}
