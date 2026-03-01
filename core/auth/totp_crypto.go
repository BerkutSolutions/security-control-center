package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"

	"berkut-scc/core/utils"
)

var ErrTOTPSecretDecryptFailed = errors.New("totp secret decrypt failed")

func EncryptTOTPSecret(secretBase32 string, pepper string) (string, error) {
	enc, err := totpEncryptor(pepper)
	if err != nil {
		return "", err
	}
	blob, err := enc.EncryptToBlob([]byte(strings.TrimSpace(secretBase32)))
	if err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(blob), nil
}

func DecryptTOTPSecret(secretEnc string, pepper string) (string, error) {
	secretEnc = strings.TrimSpace(secretEnc)
	if secretEnc == "" {
		return "", nil
	}
	blob, err := base64.RawStdEncoding.DecodeString(secretEnc)
	if err != nil {
		return "", ErrTOTPSecretDecryptFailed
	}
	enc, err := totpEncryptor(pepper)
	if err != nil {
		return "", err
	}
	plain, err := enc.DecryptBlob(blob)
	if err != nil {
		return "", ErrTOTPSecretDecryptFailed
	}
	return strings.TrimSpace(string(plain)), nil
}

func totpEncryptor(pepper string) (*utils.Encryptor, error) {
	pepper = strings.TrimSpace(pepper)
	if pepper == "" {
		return nil, errors.New("empty pepper")
	}
	sum := sha256.Sum256([]byte("berkut-scc:totp:" + pepper))
	keyHex := hex.EncodeToString(sum[:])
	return utils.NewEncryptorFromString(keyHex)
}

