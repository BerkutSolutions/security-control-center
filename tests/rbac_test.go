package tests

import (
	"testing"

	"berkut-scc/core/rbac"
)

func TestRBACPolicy(t *testing.T) {
	policy := rbac.NewPolicy(rbac.DefaultRoles())
	if !policy.Allowed([]string{"admin"}, "dashboard.view") {
		t.Fatalf("admin should view dashboard")
	}
	if policy.Allowed([]string{"analyst"}, "accounts.manage") {
		t.Fatalf("analyst must not manage accounts")
	}
	if policy.Allowed([]string{}, "dashboard.view") {
		t.Fatalf("deny by default expected")
	}
}
