package utils

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
)

func RandBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	return b, err
}

func RandString(n int) (string, error) {
	b, err := RandBytes(n)
	if err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(b), nil
}

func ConstantTimeEquals(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare(a, b) == 1
}
