package tests

import (
	"testing"

	"berkut-scc/core/auth"
)

func TestRecoveryCodeHashVerifyNormalized(t *testing.T) {
	pepper := "pepper"
	code := "abcde-fghij"
	ph, err := auth.HashRecoveryCode(code, pepper)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	ok, err := auth.VerifyRecoveryCode("ABCDE F GHIJ", pepper, ph)
	if err != nil || !ok {
		t.Fatalf("verify failed: ok=%v err=%v", ok, err)
	}
	ok, _ = auth.VerifyRecoveryCode("wrong-wrong", pepper, ph)
	if ok {
		t.Fatalf("expected mismatch for wrong code")
	}
}
