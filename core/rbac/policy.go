package rbac

import "sync"

type Policy struct {
	mu        sync.RWMutex
	rolePerms map[string]map[Permission]struct{}
}

func NewPolicy(roles []Role) *Policy {
	p := &Policy{rolePerms: map[string]map[Permission]struct{}{}}
	p.Replace(roles)
	return p
}

func (p *Policy) Allowed(userRoles []string, perm Permission) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, r := range userRoles {
		if perms, ok := p.rolePerms[r]; ok {
			if _, ok := perms[perm]; ok {
				return true
			}
		}
	}
	return false
}

// For menu building.
func (p *Policy) Roles() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	keys := make([]string, 0, len(p.rolePerms))
	for k := range p.rolePerms {
		keys = append(keys, k)
	}
	return keys
}

// PermissionsForRoles returns the union of permissions for the provided roles.
func (p *Policy) PermissionsForRoles(roles []string) []Permission {
	p.mu.RLock()
	defer p.mu.RUnlock()
	set := map[Permission]struct{}{}
	for _, r := range roles {
		if perms, ok := p.rolePerms[r]; ok {
			for perm := range perms {
				set[perm] = struct{}{}
			}
		}
	}
	out := make([]Permission, 0, len(set))
	for perm := range set {
		out = append(out, perm)
	}
	return out
}

func (p *Policy) Replace(roles []Role) {
	p.mu.Lock()
	defer p.mu.Unlock()
	rp := make(map[string]map[Permission]struct{})
	for _, r := range roles {
		m := make(map[Permission]struct{})
		for _, perm := range r.Permissions {
			m[perm] = struct{}{}
		}
		rp[r.Name] = m
	}
	p.rolePerms = rp
}
