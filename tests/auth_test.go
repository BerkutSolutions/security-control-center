package tests

import (
	"testing"

	"berkut-scc/core/auth"
)

func TestPasswordHashVerify(t *testing.T) {
	pepper := "pepper"
	pass := "S3cure#Pass"
	ph, err := auth.HashPassword(pass, pepper)
	if err != nil {
		t.Fatalf("hash err: %v", err)
	}
	ok, err := auth.VerifyPassword(pass, pepper, ph)
	if err != nil || !ok {
		t.Fatalf("verify failed")
	}
	ok, _ = auth.VerifyPassword("wrong", pepper, ph)
	if ok {
		t.Fatalf("expected failure for wrong password")
	}
}
