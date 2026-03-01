package auth

import (
	"crypto/rand"
	"encoding/base32"
	"net/url"
	"strings"
)

func GenerateTOTPSecret() (string, error) {
	// 20 bytes is a common default for TOTP secrets.
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)
	secret = strings.ToUpper(strings.TrimSpace(secret))
	return secret, nil
}

func BuildTOTPProvisioningURI(issuer string, accountName string, secretBase32 string) string {
	issuer = strings.TrimSpace(issuer)
	if issuer == "" {
		issuer = "Berkut SCC"
	}
	accountName = strings.TrimSpace(accountName)
	secretBase32 = strings.TrimSpace(secretBase32)
	label := issuer
	if accountName != "" {
		label = issuer + ":" + accountName
	}
	q := url.Values{}
	q.Set("secret", secretBase32)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", "6")
	q.Set("period", "30")
	u := &url.URL{
		Scheme:   "otpauth",
		Host:     "totp",
		Path:     "/" + url.PathEscape(label),
		RawQuery: q.Encode(),
	}
	return u.String()
}

