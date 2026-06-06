package model

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"
)

// ParseCertificate decodes the first CERTIFICATE PEM block in data and parses
// it as an X.509 certificate. Certificates are public artifacts, so their
// fields are not secret; nevertheless the inspector only surfaces identity and
// validity metadata. It returns an error when no certificate block is present
// or the DER cannot be parsed.
func ParseCertificate(data []byte) (*x509.Certificate, error) {
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil, fmt.Errorf("no CERTIFICATE PEM block found")
		}
		if block.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("parse certificate: %w", err)
			}
			return cert, nil
		}
	}
}

// inspectCertificate parses a PEM certificate and reports its subject, issuer,
// validity window, and serial number. Validity is checked against the current
// time.
func inspectCertificate(source string, data []byte) Inspection {
	insp := Inspection{Kind: KindCertificate, Source: source}

	cert, err := ParseCertificate(data)
	if err != nil {
		insp.Issues = []string{fmt.Sprintf("parse failed: %v", err)}
		return insp
	}

	insp.Fields = []Field{
		{Key: "subject", Value: cert.Subject.String()},
		{Key: "issuer", Value: cert.Issuer.String()},
		{Key: "serial", Value: cert.SerialNumber.String()},
		{Key: "not_before", Value: cert.NotBefore.UTC().Format(time.RFC3339)},
		{Key: "not_after", Value: cert.NotAfter.UTC().Format(time.RFC3339)},
		{Key: "signature_algorithm", Value: cert.SignatureAlgorithm.String()},
		{Key: "is_ca", Value: fmt.Sprintf("%t", cert.IsCA)},
	}
	for _, dns := range cert.DNSNames {
		insp.Fields = append(insp.Fields, Field{Key: "dns_name", Value: dns})
	}

	now := time.Now()
	if now.Before(cert.NotBefore) {
		insp.Issues = append(insp.Issues, fmt.Sprintf("certificate not valid until %s", cert.NotBefore.UTC().Format(time.RFC3339)))
	}
	if now.After(cert.NotAfter) {
		insp.Issues = append(insp.Issues, fmt.Sprintf("certificate expired on %s", cert.NotAfter.UTC().Format(time.RFC3339)))
	}

	insp.Valid = len(insp.Issues) == 0
	return insp
}
