package rbac

import (
	"sort"
	"strings"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

type Policy struct {
	mu       sync.RWMutex
	enforcer *casbin.Enforcer
}

func NewPolicy(roles []Role) *Policy {
	p := &Policy{}
	p.Replace(roles)
	return p
}

func (p *Policy) Allowed(userRoles []string, perm Permission) bool {
	p.mu.RLock()
	e := p.enforcer
	p.mu.RUnlock()
	if e == nil {
		return false
	}
	for _, r := range userRoles {
		roleName := strings.ToLower(strings.TrimSpace(r))
		if roleName == "" {
			continue
		}
		ok, err := e.Enforce(roleName, string(perm))
		if err == nil && ok {
			return true
		}
	}
	return false
}

// For menu building.
func (p *Policy) Roles() []string {
	p.mu.RLock()
	e := p.enforcer
	p.mu.RUnlock()
	if e == nil {
		return nil
	}
	subjects, err := e.GetAllSubjects()
	if err != nil {
		return nil
	}
	sort.Strings(subjects)
	return subjects
}

// PermissionsForRoles returns the union of permissions for the provided roles.
func (p *Policy) PermissionsForRoles(roles []string) []Permission {
	p.mu.RLock()
	e := p.enforcer
	p.mu.RUnlock()
	if e == nil {
		return nil
	}
	set := map[Permission]struct{}{}
	for _, r := range roles {
		roleName := strings.ToLower(strings.TrimSpace(r))
		if roleName == "" {
			continue
		}
		rules, err := e.GetFilteredPolicy(0, roleName)
		if err != nil {
			continue
		}
		for _, rule := range rules {
			if len(rule) < 2 {
				continue
			}
			if len(rule) > 2 && !strings.EqualFold(rule[2], "allow") {
				continue
			}
			set[Permission(strings.TrimSpace(rule[1]))] = struct{}{}
		}
	}
	out := make([]Permission, 0, len(set))
	for perm := range set {
		out = append(out, perm)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (p *Policy) Replace(roles []Role) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e := buildCasbinEnforcer()
	if e == nil {
		p.enforcer = nil
		return
	}
	for _, r := range roles {
		roleName := strings.ToLower(strings.TrimSpace(r.Name))
		if roleName == "" {
			continue
		}
		for _, perm := range r.Permissions {
			permName := strings.ToLower(strings.TrimSpace(string(perm)))
			if permName == "" || !IsKnownPermission(Permission(permName)) {
				continue
			}
			_, _ = e.AddPolicy(roleName, permName, "allow")
		}
	}
	p.enforcer = e
}

func buildCasbinEnforcer() *casbin.Enforcer {
	m, err := model.NewModelFromString(`
[request_definition]
r = sub, obj

[policy_definition]
p = sub, obj, eft

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.obj == p.obj
`)
	if err != nil {
		return nil
	}
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil
	}
	return e
}
