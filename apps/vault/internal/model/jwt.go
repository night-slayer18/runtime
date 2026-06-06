package model

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// JWT holds the decoded, non-sensitive portions of a JSON Web Token. The raw
// signature bytes are intentionally not retained; only its presence and length
// are recorded. Claim values are not exposed by the inspector — only the set of
// claim keys and a few well-known, non-secret registered claims (exp, iat, nbf,
// iss, sub, aud, alg) are surfaced as metadata.
type JWT struct {
	Header       map[string]interface{}
	Claims       map[string]interface{}
	SignatureLen int
}

// DecodeJWT splits and base64url-decodes a token's three segments. It performs
// NO signature verification (by design — Vault inspects, it does not trust).
// It returns an error when the token is not three segments or a segment is not
// valid base64url JSON for the header/payload.
func DecodeJWT(token string) (*JWT, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("expected 3 segments, got %d", len(parts))
	}

	header := map[string]interface{}{}
	if err := decodeSegmentJSON(parts[0], &header); err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}
	claims := map[string]interface{}{}
	if err := decodeSegmentJSON(parts[1], &claims); err != nil {
		return nil, fmt.Errorf("payload: %w", err)
	}

	sig, err := decodeSegment(parts[2])
	if err != nil {
		return nil, fmt.Errorf("signature: %w", err)
	}

	return &JWT{Header: header, Claims: claims, SignatureLen: len(sig)}, nil
}

// decodeSegment base64url-decodes a single JWT segment, tolerating missing
// padding as permitted by RFC 7515.
func decodeSegment(seg string) ([]byte, error) {
	if pad := len(seg) % 4; pad != 0 {
		seg += strings.Repeat("=", 4-pad)
	}
	return base64.URLEncoding.DecodeString(seg)
}

func decodeSegmentJSON(seg string, dst *map[string]interface{}) error {
	b, err := decodeSegment(seg)
	if err != nil {
		return fmt.Errorf("invalid base64url: %w", err)
	}
	if err := json.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// inspectJWT decodes a token and reports non-sensitive metadata. Claim values
// are not echoed except for the standard time/identity claims that are not
// secret by convention; everything else is reported as a present claim key
// only. Expiry is checked when an "exp" claim is present.
func inspectJWT(source, token string) Inspection {
	insp := Inspection{Kind: KindJWT, Source: source}

	jwt, err := DecodeJWT(token)
	if err != nil {
		insp.Issues = []string{fmt.Sprintf("decode failed: %v", err)}
		return insp
	}

	if alg, ok := jwt.Header["alg"].(string); ok {
		insp.Fields = append(insp.Fields, Field{Key: "alg", Value: alg})
		if strings.EqualFold(alg, "none") {
			insp.Issues = append(insp.Issues, `insecure "alg": none accepts unsigned tokens`)
		}
	}
	if typ, ok := jwt.Header["typ"].(string); ok {
		insp.Fields = append(insp.Fields, Field{Key: "typ", Value: typ})
	}

	// Registered, non-secret claims that are safe to surface as metadata.
	for _, name := range []string{"iss", "sub", "aud"} {
		if v, ok := jwt.Claims[name]; ok {
			insp.Fields = append(insp.Fields, Field{Key: name, Value: fmt.Sprintf("%v", v)})
		}
	}
	for _, name := range []string{"iat", "nbf", "exp"} {
		if v, ok := jwt.Claims[name]; ok {
			insp.Fields = append(insp.Fields, Field{Key: name, Value: formatUnixClaim(v)})
		}
	}

	// All other claim keys are surfaced by name only (values may be secret).
	custom := make([]string, 0)
	for k := range jwt.Claims {
		switch k {
		case "iss", "sub", "aud", "iat", "nbf", "exp":
			continue
		default:
			custom = append(custom, k)
		}
	}
	sort.Strings(custom)
	for _, k := range custom {
		insp.Fields = append(insp.Fields, Field{Key: k, Value: "(claim present)", Sensitive: true})
	}

	insp.Fields = append(insp.Fields, Field{Key: "signature", Value: fmt.Sprintf("%d bytes", jwt.SignatureLen)})

	// Expiry validation.
	if exp, ok := unixClaim(jwt.Claims["exp"]); ok {
		if time.Now().After(time.Unix(exp, 0)) {
			insp.Issues = append(insp.Issues, "token is expired")
		}
	}
	if nbf, ok := unixClaim(jwt.Claims["nbf"]); ok {
		if time.Now().Before(time.Unix(nbf, 0)) {
			insp.Issues = append(insp.Issues, "token is not yet valid (nbf in the future)")
		}
	}

	insp.Valid = len(insp.Issues) == 0
	return insp
}

// unixClaim extracts a numeric Unix-timestamp claim. JSON numbers decode to
// float64, so that is the primary case.
func unixClaim(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

// formatUnixClaim renders a timestamp claim as an RFC3339 instant when it looks
// like a Unix time, falling back to the raw scalar otherwise.
func formatUnixClaim(v interface{}) string {
	if ts, ok := unixClaim(v); ok {
		return time.Unix(ts, 0).UTC().Format(time.RFC3339)
	}
	return fmt.Sprintf("%v", v)
}

// isJWTShaped reports whether s looks like a compact JWS: three non-empty,
// dot-separated base64url segments on a single line.
func isJWTShaped(s string) bool {
	if strings.ContainsAny(s, "\n\r ") {
		return false
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return false
	}
	for i, p := range parts {
		// The signature (3rd) may be empty for alg=none; header/payload may not.
		if p == "" && i < 2 {
			return false
		}
		if !isBase64URL(p) {
			return false
		}
	}
	return true
}

func isBase64URL(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9',
			r == '-', r == '_', r == '=':
		default:
			return false
		}
	}
	return true
}
