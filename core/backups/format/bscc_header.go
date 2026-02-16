package format

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	Magic               = "BSCC"
	FileFormatVersion   = byte(1)
	CryptoFormatVersion = byte(1)
)

type Header struct {
	FormatVersion byte
	CryptoVersion byte
	Nonce         []byte
	ChunkSize     uint32
}

func EncodeHeader(w io.Writer, h Header) error {
	if len(h.Nonce) == 0 || len(h.Nonce) > 255 {
		return fmt.Errorf("invalid nonce length")
	}
	if h.ChunkSize == 0 {
		return fmt.Errorf("invalid chunk size")
	}
	if _, err := w.Write([]byte(Magic)); err != nil {
		return err
	}
	if _, err := w.Write([]byte{h.FormatVersion, h.CryptoVersion, byte(len(h.Nonce))}); err != nil {
		return err
	}
	if _, err := w.Write(h.Nonce); err != nil {
		return err
	}
	var chunkBuf [4]byte
	binary.BigEndian.PutUint32(chunkBuf[:], h.ChunkSize)
	_, err := w.Write(chunkBuf[:])
	return err
}

func DecodeHeader(r io.Reader) (Header, error) {
	magic := make([]byte, len(Magic))
	if _, err := io.ReadFull(r, magic); err != nil {
		return Header{}, err
	}
	if string(magic) != Magic {
		return Header{}, fmt.Errorf("invalid bscc magic")
	}
	meta := make([]byte, 3)
	if _, err := io.ReadFull(r, meta); err != nil {
		return Header{}, err
	}
	nonceLen := int(meta[2])
	if nonceLen <= 0 {
		return Header{}, fmt.Errorf("invalid nonce length")
	}
	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(r, nonce); err != nil {
		return Header{}, err
	}
	var chunkBuf [4]byte
	if _, err := io.ReadFull(r, chunkBuf[:]); err != nil {
		return Header{}, err
	}
	chunkSize := binary.BigEndian.Uint32(chunkBuf[:])
	if chunkSize == 0 {
		return Header{}, fmt.Errorf("invalid chunk size")
	}
	return Header{
		FormatVersion: meta[0],
		CryptoVersion: meta[1],
		Nonce:         nonce,
		ChunkSize:     chunkSize,
	}, nil
}
