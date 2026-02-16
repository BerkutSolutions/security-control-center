package rbac

import "testing"

func TestPolicyAllowed_DefaultRoles(t *testing.T) {
	p := NewPolicy(DefaultRoles())
	if p == nil {
		t.Fatal("policy is nil")
	}

	if !p.Allowed([]string{"admin"}, "accounts.manage") {
		t.Fatal("admin must have accounts.manage")
	}
	if p.Allowed([]string{"doc_viewer"}, "accounts.manage") {
		t.Fatal("doc_viewer must not have accounts.manage")
	}
	if !p.Allowed([]string{"doc_viewer"}, "docs.view") {
		t.Fatal("doc_viewer must have docs.view")
	}
}

func TestPolicyReplace_RebuildsEnforcer(t *testing.T) {
	p := NewPolicy(nil)
	p.Replace([]Role{
		{
			Name: "custom",
			Permissions: []Permission{
				"monitoring.view",
			},
		},
	})

	if !p.Allowed([]string{"custom"}, "monitoring.view") {
		t.Fatal("custom role must have monitoring.view")
	}
	if p.Allowed([]string{"custom"}, "accounts.manage") {
		t.Fatal("custom role must not have accounts.manage")
	}
}

func TestPermissionsForRoles_UniqueUnion(t *testing.T) {
	p := NewPolicy([]Role{
		{
			Name: "r1",
			Permissions: []Permission{
				"docs.view",
				"monitoring.view",
			},
		},
		{
			Name: "r2",
			Permissions: []Permission{
				"docs.view",
			},
		},
	})

	perms := p.PermissionsForRoles([]string{"r1", "r2"})
	if len(perms) != 2 {
		t.Fatalf("expected 2 unique permissions, got %d", len(perms))
	}
}
