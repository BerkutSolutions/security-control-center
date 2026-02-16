package crypto

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptDecryptFileRoundtrip(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.bin")
	enc := filepath.Join(dir, "out.bscc")
	dec := filepath.Join(dir, "out.bin")
	plain := []byte("berkut backup payload")
	if err := os.WriteFile(in, plain, 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}
	c, err := NewFileCipher("plain:key:for:backup:encryption:32chars", 1024)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	if err := c.EncryptFile(in, enc); err != nil {
		t.Fatalf("encrypt file: %v", err)
	}
	if err := c.DecryptFile(enc, dec); err != nil {
		t.Fatalf("decrypt file: %v", err)
	}
	got, err := os.ReadFile(dec)
	if err != nil {
		t.Fatalf("read decrypted: %v", err)
	}
	if string(got) != string(plain) {
		t.Fatalf("roundtrip mismatch")
	}
}
