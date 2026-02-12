package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"time"
)

// Double-submit compatible token generator using secret key.
func GenerateCSRF(secret, sessionID string) (string, error) {
	now := time.Now().Unix()
	msg := []byte(sessionID + ":" + itoa(now))
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write(msg); err != nil {
		return "", err
	}
	sig := mac.Sum(nil)
	payload := append(msg, sig...)
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func VerifyCSRF(secret, sessionID, token string, maxAge time.Duration) (bool, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return false, err
	}
	if len(raw) <= sha256.Size {
		return false, errors.New("token too short")
	}
	msg := raw[:len(raw)-sha256.Size]
	sig := raw[len(raw)-sha256.Size:]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(msg)
	expected := mac.Sum(nil)
	if !hmac.Equal(sig, expected) {
		return false, errors.New("bad signature")
	}
	parts := string(msg)
	// msg format: sessionID:timestamp
	var ts int64
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == ':' {
			ts = atoi(parts[i+1:])
			break
		}
	}
	if ts == 0 {
		return false, errors.New("timestamp missing")
	}
	if time.Now().Unix()-ts > int64(maxAge.Seconds()) {
		return false, errors.New("token expired")
	}
	return true, nil
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

func atoi(v string) int64 {
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}
