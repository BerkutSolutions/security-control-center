package rbac

import "testing"

func TestNormalizePermissionNames(t *testing.T) {
	valid, invalid := NormalizePermissionNames([]string{
		" docs.view ",
		"DOCS.VIEW",
		"accounts.manage",
		"unknown.permission",
		"",
	})
	if len(valid) != 2 {
		t.Fatalf("expected 2 valid permissions, got %d", len(valid))
	}
	if len(invalid) != 1 || invalid[0] != "unknown.permission" {
		t.Fatalf("unexpected invalid permissions: %v", invalid)
	}
}

func TestIsKnownPermission(t *testing.T) {
	if !IsKnownPermission("docs.view") {
		t.Fatal("docs.view must be known")
	}
	if IsKnownPermission("custom.permission") {
		t.Fatal("custom.permission must be unknown")
	}
}
