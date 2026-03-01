package auth

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidTOTPSecret = errors.New("invalid totp secret")

type TOTPConfig struct {
	PeriodSec int64
	Digits    int
	Skew      int64
}

func DefaultTOTPConfig() TOTPConfig {
	return TOTPConfig{PeriodSec: 30, Digits: 6, Skew: 1}
}

func NormalizeTOTPCode(raw string) string {
	return strings.TrimSpace(strings.ReplaceAll(raw, " ", ""))
}

func VerifyTOTP(secretBase32 string, code string, now time.Time, cfg TOTPConfig) (bool, error) {
	secret, err := decodeBase32Secret(secretBase32)
	if err != nil {
		return false, err
	}
	code = NormalizeTOTPCode(code)
	if len(code) != cfg.Digits {
		return false, nil
	}
	if _, err := strconv.Atoi(code); err != nil {
		return false, nil
	}
	step := cfg.PeriodSec
	if step <= 0 {
		step = 30
	}
	counter := now.UTC().Unix() / step
	skew := cfg.Skew
	if skew < 0 {
		skew = 0
	}
	for i := -skew; i <= skew; i++ {
		exp := totpAt(secret, counter+i, cfg.Digits)
		if exp == code {
			return true, nil
		}
	}
	return false, nil
}

func ComputeTOTPCode(secretBase32 string, now time.Time, cfg TOTPConfig) (string, error) {
	secret, err := decodeBase32Secret(secretBase32)
	if err != nil {
		return "", err
	}
	step := cfg.PeriodSec
	if step <= 0 {
		step = 30
	}
	digits := cfg.Digits
	if digits <= 0 {
		digits = 6
	}
	counter := now.UTC().Unix() / step
	return totpAt(secret, counter, digits), nil
}

func decodeBase32Secret(secretBase32 string) ([]byte, error) {
	val := strings.ToUpper(strings.TrimSpace(secretBase32))
	val = strings.ReplaceAll(val, " ", "")
	val = strings.ReplaceAll(val, "-", "")
	if val == "" {
		return nil, ErrInvalidTOTPSecret
	}
	dec := base32.StdEncoding.WithPadding(base32.NoPadding)
	b, err := dec.DecodeString(val)
	if err != nil || len(b) < 10 {
		return nil, ErrInvalidTOTPSecret
	}
	return b, nil
}

func totpAt(secret []byte, counter int64, digits int) string {
	if digits <= 0 {
		digits = 6
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(counter))
	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	bin := (int(sum[offset])&0x7f)<<24 |
		(int(sum[offset+1])&0xff)<<16 |
		(int(sum[offset+2])&0xff)<<8 |
		(int(sum[offset+3]) & 0xff)
	mod := 1
	for i := 0; i < digits; i++ {
		mod *= 10
	}
	otp := bin % mod
	return fmt.Sprintf("%0*d", digits, otp)
}
