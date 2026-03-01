package auth

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"strings"
)

var ErrInvalidRecoveryCode = errors.New("invalid recovery code")

func NormalizeRecoveryCode(raw string) string {
	val := strings.ToUpper(strings.TrimSpace(raw))
	val = strings.ReplaceAll(val, " ", "")
	val = strings.ReplaceAll(val, "-", "")
	return val
}

func GenerateRecoveryCodes(count int) ([]string, error) {
	if count <= 0 {
		count = 10
	}
	out := make([]string, 0, count)
	for len(out) < count {
		code, err := generateRecoveryCode()
		if err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, nil
}

func generateRecoveryCode() (string, error) {
	// 10 base32 chars -> 50 bits, good enough for one-time codes.
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	raw := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, "=", "")
	raw = strings.ToUpper(raw)
	if len(raw) < 10 {
		return "", errors.New("recovery code generation failed")
	}
	code := raw[:10]
	return code[:5] + "-" + code[5:], nil
}

