package format

import (
	"bytes"
	"testing"
)

func TestHeaderEncodeDecodeRoundtrip(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	in := Header{
		FormatVersion: 1,
		CryptoVersion: 1,
		Nonce:         []byte("123456789012"),
		ChunkSize:     1024,
	}
	if err := EncodeHeader(buf, in); err != nil {
		t.Fatalf("encode header: %v", err)
	}
	out, err := DecodeHeader(buf)
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	if out.FormatVersion != in.FormatVersion || out.CryptoVersion != in.CryptoVersion {
		t.Fatalf("header versions mismatch")
	}
	if out.ChunkSize != in.ChunkSize {
		t.Fatalf("chunk size mismatch")
	}
	if string(out.Nonce) != string(in.Nonce) {
		t.Fatalf("nonce mismatch")
	}
}

func TestHeaderDecodeInvalidMagic(t *testing.T) {
	buf := bytes.NewBufferString("NOPE")
	if _, err := DecodeHeader(buf); err == nil {
		t.Fatalf("expected invalid magic error")
	}
}
