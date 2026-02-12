package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    uint32 = 1
	argonMemory  uint32 = 64 * 1024
	argonThreads uint8  = 4
	argonKeyLen  uint32 = 32
	saltLen             = 16
)

type PasswordHash struct {
	Hash string
	Salt string
}

func HashPassword(password, pepper string) (*PasswordHash, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	input := append([]byte(password), []byte(pepper)...)
	key := argon2.IDKey(input, salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return &PasswordHash{
		Hash: base64.RawStdEncoding.EncodeToString(key),
		Salt: base64.RawStdEncoding.EncodeToString(salt),
	}, nil
}

func VerifyPassword(password, pepper string, stored *PasswordHash) (bool, error) {
	salt, err := base64.RawStdEncoding.DecodeString(stored.Salt)
	if err != nil {
		return false, err
	}
	expected, err := base64.RawStdEncoding.DecodeString(stored.Hash)
	if err != nil {
		return false, err
	}
	input := append([]byte(password), []byte(pepper)...)
	key := argon2.IDKey(input, salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	if subtle.ConstantTimeCompare(key, expected) == 1 {
		return true, nil
	}
	return false, nil
}

func MustHashPassword(password, pepper string) *PasswordHash {
	p, err := HashPassword(password, pepper)
	if err != nil {
		panic(err)
	}
	return p
}

func ParsePasswordHash(hash, salt string) (*PasswordHash, error) {
	if hash == "" || salt == "" {
		return nil, errors.New("empty hash or salt")
	}
	return &PasswordHash{Hash: hash, Salt: salt}, nil
}
