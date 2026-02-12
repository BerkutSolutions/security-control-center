package monitoring

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"
)

func TestTLSInfoFromState(t *testing.T) {
	now := time.Now().UTC()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "example.local"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(30 * 24 * time.Hour),
		DNSNames:     []string{"example.local"},
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	state := &tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}
	info := tlsFromState(state)
	if info == nil {
		t.Fatalf("expected tls info")
	}
	if info.CommonName != "example.local" {
		t.Fatalf("unexpected common name: %s", info.CommonName)
	}
	if info.NotAfter.IsZero() {
		t.Fatalf("missing not_after")
	}
	daysLeft := int(time.Until(info.NotAfter).Hours() / 24)
	if daysLeft < 25 {
		t.Fatalf("unexpected days left: %d", daysLeft)
	}
}
