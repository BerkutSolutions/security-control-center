package auth

import (
	"sort"
	"strings"

	"berkut-scc/core/rbac"
	"berkut-scc/core/store"
)

// CalculateEffectiveAccess aggregates roles, permissions and clearance from a user and their groups.
func CalculateEffectiveAccess(user *store.User, directRoles []string, groups []store.Group, policy *rbac.Policy) store.EffectiveAccess {
	roleSet := map[string]struct{}{}
	for _, r := range directRoles {
		roleSet[strings.ToLower(r)] = struct{}{}
	}
	for _, g := range groups {
		for _, r := range g.Roles {
			roleSet[strings.ToLower(r)] = struct{}{}
		}
	}
	effRoles := setToSortedSlice(roleSet)

	permSet := map[string]struct{}{}
	if policy != nil {
		for _, p := range policy.PermissionsForRoles(effRoles) {
			permSet[string(p)] = struct{}{}
		}
	}

	level := 0
	if user != nil {
		level = user.ClearanceLevel
	}
	tagSet := map[string]struct{}{}
	if user != nil {
		for _, t := range user.ClearanceTags {
			tagSet[strings.ToLower(strings.TrimSpace(t))] = struct{}{}
		}
	}
	menuSet := map[string]struct{}{}
	for _, g := range groups {
		if g.ClearanceLevel > level {
			level = g.ClearanceLevel
		}
		for _, t := range g.ClearanceTags {
			tagSet[strings.ToLower(strings.TrimSpace(t))] = struct{}{}
		}
		for _, m := range g.MenuPermissions {
			val := strings.ToLower(strings.TrimSpace(m))
			if val != "" {
				menuSet[val] = struct{}{}
			}
		}
	}

	return store.EffectiveAccess{
		Roles:             effRoles,
		Permissions:       setToSortedSlice(permSet),
		ClearanceLevel:    level,
		ClearanceTags:     setToSortedSlice(tagSet),
		MenuPermissions:   setToSortedSlice(menuSet),
	}
}

func setToSortedSlice(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for v := range set {
		if strings.TrimSpace(v) == "" {
			continue
		}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
