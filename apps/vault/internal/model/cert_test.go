package model

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"
)

// makeTestCert generates a synthetic self-signed certificate for testing.
// Returns the PEM bytes. This is a throwaway test fixture — never a real cert.
func makeTestCert(t *testing.T, notBefore, notAfter time.Time, cn string) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject:      pkix.Name{CommonName: cn, Organization: []string{"Runtime Test"}},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		DNSNames:     []string{"example.test"},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestParseCertificate_Valid(t *testing.T) {
	pemData := makeTestCert(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), "valid.test")
	cert, err := ParseCertificate(pemData)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cert.Subject.CommonName != "valid.test" {
		t.Errorf("CN: got %q", cert.Subject.CommonName)
	}
}

func TestParseCertificate_NoBlock(t *testing.T) {
	if _, err := ParseCertificate([]byte("not a pem file")); err == nil {
		t.Error("expected error when no CERTIFICATE block present")
	}
}

func TestInspectCertificate_ValidWindow(t *testing.T) {
	pemData := makeTestCert(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), "ok.test")
	insp := inspectCertificate("cert.pem", pemData)
	if !insp.Valid {
		t.Errorf("expected valid cert, issues: %v", insp.Issues)
	}
	if insp.Kind != KindCertificate {
		t.Errorf("kind: got %v", insp.Kind)
	}
}

func TestInspectCertificate_Expired(t *testing.T) {
	pemData := makeTestCert(t, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour), "old.test")
	insp := inspectCertificate("cert.pem", pemData)
	if insp.Valid {
		t.Error("expected expired cert to be invalid")
	}
	found := false
	for _, iss := range insp.Issues {
		if strings.Contains(iss, "expired") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected expiry issue, got %v", insp.Issues)
	}
}
