package crypto

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

func DeriveKey(raw string) ([]byte, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, ErrInvalidKey
	}
	decoded := decodeKeyMaterial(value)
	if len(decoded) < 32 {
		return nil, ErrInvalidKey
	}
	if len(decoded) == 32 {
		return decoded, nil
	}
	sum := sha256.Sum256(decoded)
	return sum[:], nil
}

func decodeKeyMaterial(raw string) []byte {
	if b, err := hex.DecodeString(raw); err == nil {
		return b
	}
	if b, err := base64.StdEncoding.DecodeString(raw); err == nil {
		return b
	}
	if b, err := base64.RawStdEncoding.DecodeString(raw); err == nil {
		return b
	}
	return []byte(raw)
}
