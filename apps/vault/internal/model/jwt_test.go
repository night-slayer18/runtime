package model

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// makeJWT builds a synthetic, unsigned-shaped JWT for testing. The signature is
// an arbitrary non-secret string. This is a test fixture only — never a real token.
func makeJWT(t *testing.T, header, claims map[string]interface{}) string {
	t.Helper()
	enc := func(m map[string]interface{}) string {
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	sig := base64.RawURLEncoding.EncodeToString([]byte("test-signature"))
	return strings.Join([]string{enc(header), enc(claims), sig}, ".")
}

func TestDecodeJWT_Valid(t *testing.T) {
	token := makeJWT(t,
		map[string]interface{}{"alg": "HS256", "typ": "JWT"},
		map[string]interface{}{"sub": "user-123", "iss": "test"},
	)
	jwt, err := DecodeJWT(token)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if jwt.Header["alg"] != "HS256" {
		t.Errorf("alg: got %v", jwt.Header["alg"])
	}
	if jwt.Claims["sub"] != "user-123" {
		t.Errorf("sub: got %v", jwt.Claims["sub"])
	}
	if jwt.SignatureLen == 0 {
		t.Error("expected non-zero signature length")
	}
}

func TestDecodeJWT_WrongSegmentCount(t *testing.T) {
	if _, err := DecodeJWT("only.two"); err == nil {
		t.Error("expected error for 2-segment token")
	}
	if _, err := DecodeJWT("a.b.c.d"); err == nil {
		t.Error("expected error for 4-segment token")
	}
}

func TestDecodeJWT_InvalidBase64(t *testing.T) {
	if _, err := DecodeJWT("!!!.###.$$$"); err == nil {
		t.Error("expected error for invalid base64url segments")
	}
}

func TestInspectJWT_ExpiredFlagged(t *testing.T) {
	token := makeJWT(t,
		map[string]interface{}{"alg": "HS256"},
		map[string]interface{}{"exp": float64(time.Now().Add(-time.Hour).Unix())},
	)
	insp := inspectJWT("<input>", token)
	if insp.Valid {
		t.Error("expected expired token to be invalid")
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

func TestInspectJWT_AlgNoneFlagged(t *testing.T) {
	token := makeJWT(t,
		map[string]interface{}{"alg": "none"},
		map[string]interface{}{"sub": "x"},
	)
	insp := inspectJWT("<input>", token)
	found := false
	for _, iss := range insp.Issues {
		if strings.Contains(iss, "none") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected alg:none issue, got %v", insp.Issues)
	}
}

func TestInspectJWT_CustomClaimNotEchoed(t *testing.T) {
	token := makeJWT(t,
		map[string]interface{}{"alg": "HS256"},
		map[string]interface{}{"secret_claim": "do-not-leak"},
	)
	insp := inspectJWT("<input>", token)
	for _, f := range insp.Fields {
		if strings.Contains(f.Value, "do-not-leak") {
			t.Fatalf("custom claim value leaked in field %q", f.Key)
		}
	}
}
