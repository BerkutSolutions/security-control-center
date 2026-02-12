package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
)

type Encryptor struct {
	aead cipher.AEAD
}

func NewEncryptorFromString(key string) (*Encryptor, error) {
	k, err := decodeKey(key)
	if err != nil {
		return nil, err
	}
	if len(k) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(k))
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Encryptor{aead: aead}, nil
}

func decodeKey(k string) ([]byte, error) {
	if len(k) == 32 {
		return []byte(k), nil
	}
	if len(k) == 64 {
		if b, err := hex.DecodeString(k); err == nil {
			return b, nil
		}
	}
	if b, err := base64.StdEncoding.DecodeString(k); err == nil {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(k); err == nil {
		return b, nil
	}
	return nil, errors.New("invalid encryption key format")
}

func (e *Encryptor) Encrypt(plaintext []byte) ([]byte, []byte, error) {
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	ciphertext := e.aead.Seal(nil, nonce, plaintext, nil)
	return nonce, ciphertext, nil
}

func (e *Encryptor) Decrypt(nonce, ciphertext []byte) ([]byte, error) {
	return e.aead.Open(nil, nonce, ciphertext, nil)
}

func (e *Encryptor) EncryptToBlob(plaintext []byte) ([]byte, error) {
	nonce, ct, err := e.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}
	return append(nonce, ct...), nil
}

func (e *Encryptor) DecryptBlob(data []byte) ([]byte, error) {
	ns := e.aead.NonceSize()
	if len(data) < ns {
		return nil, errors.New("ciphertext too short")
	}
	nonce := data[:ns]
	ct := data[ns:]
	return e.Decrypt(nonce, ct)
}
