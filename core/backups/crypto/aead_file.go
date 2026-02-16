package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"berkut-scc/core/backups/format"
)

const defaultChunkSize = 1024 * 1024

type FileCipher struct {
	key       []byte
	chunkSize uint32
}

func NewFileCipher(rawKey string, chunkSize uint32) (*FileCipher, error) {
	key, err := DeriveKey(rawKey)
	if err != nil {
		return nil, err
	}
	if chunkSize == 0 {
		chunkSize = defaultChunkSize
	}
	return &FileCipher{key: key, chunkSize: chunkSize}, nil
}

func (c *FileCipher) EncryptFile(inPath, outPath string) error {
	in, err := os.Open(inPath)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	baseNonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(baseNonce); err != nil {
		return err
	}
	header := format.Header{
		FormatVersion: format.FileFormatVersion,
		CryptoVersion: format.CryptoFormatVersion,
		Nonce:         baseNonce,
		ChunkSize:     c.chunkSize,
	}
	if err := format.EncodeHeader(out, header); err != nil {
		return err
	}
	aad := buildAAD(header)
	buf := make([]byte, c.chunkSize)
	var chunkID uint32
	for {
		n, readErr := in.Read(buf)
		if n > 0 {
			nonce := chunkNonce(baseNonce, chunkID)
			ciphertext := aead.Seal(nil, nonce, buf[:n], aad)
			if err := writeChunk(out, ciphertext); err != nil {
				return err
			}
			chunkID++
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	return nil
}

func (c *FileCipher) DecryptFile(inPath, outPath string) error {
	in, err := os.Open(inPath)
	if err != nil {
		return err
	}
	defer in.Close()

	header, err := format.DecodeHeader(in)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	if len(header.Nonce) != aead.NonceSize() {
		return fmt.Errorf("invalid nonce size")
	}

	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	aad := buildAAD(header)
	var chunkID uint32
	for {
		payload, err := readChunk(in)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		nonce := chunkNonce(header.Nonce, chunkID)
		plain, err := aead.Open(nil, nonce, payload, aad)
		if err != nil {
			return err
		}
		if _, err := out.Write(plain); err != nil {
			return err
		}
		chunkID++
	}
	return nil
}

func buildAAD(h format.Header) []byte {
	return []byte{
		h.FormatVersion,
		h.CryptoVersion,
		byte(len(h.Nonce)),
	}
}

func chunkNonce(base []byte, idx uint32) []byte {
	nonce := make([]byte, len(base))
	copy(nonce, base)
	if len(nonce) >= 4 {
		binary.BigEndian.PutUint32(nonce[len(nonce)-4:], idx)
	}
	return nonce
}

func writeChunk(w io.Writer, ciphertext []byte) error {
	var sz [4]byte
	binary.BigEndian.PutUint32(sz[:], uint32(len(ciphertext)))
	if _, err := w.Write(sz[:]); err != nil {
		return err
	}
	_, err := w.Write(ciphertext)
	return err
}

func readChunk(r io.Reader) ([]byte, error) {
	var sz [4]byte
	if _, err := io.ReadFull(r, sz[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(sz[:])
	if size == 0 {
		return nil, fmt.Errorf("invalid chunk size")
	}
	out := make([]byte, size)
	if _, err := io.ReadFull(r, out); err != nil {
		return nil, err
	}
	return out, nil
}
