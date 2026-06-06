// Package model defines the core data model for runtime-vault.
//
// Security note: Vault treats every artifact it inspects as sensitive. The
// inspectors in this package parse, validate, decode, and compare secrets but
// never log or echo a secret VALUE. Results reference secrets by key name and
// expose only non-sensitive metadata (key names, value lengths, certificate
// subjects, JWT claim keys, detected formats). Use Mask to render any value
// that might be sensitive.
package model

import (
	"fmt"
	"os"
	"strings"
)

// Kind identifies the type of secret artifact an inspector recognises.
type Kind int

const (
	// KindUnknown indicates the artifact could not be classified.
	KindUnknown Kind = iota
	// KindEnv is a dotenv (KEY=VALUE) file.
	KindEnv
	// KindJWT is a JSON Web Token (header.payload.signature).
	KindJWT
	// KindCertificate is a PEM-encoded X.509 certificate.
	KindCertificate
	// KindAPIKey is a single-line API key/token.
	KindAPIKey
	// KindK8sSecret is a Kubernetes Secret manifest (JSON or YAML form).
	KindK8sSecret
)

// String returns a human-readable name for the kind.
func (k Kind) String() string {
	switch k {
	case KindEnv:
		return "env file"
	case KindJWT:
		return "JWT token"
	case KindCertificate:
		return "certificate"
	case KindAPIKey:
		return "API key"
	case KindK8sSecret:
		return "kubernetes secret"
	default:
		return "unknown"
	}
}

// Field is one displayable row of an inspection result. Sensitive values are
// never stored verbatim here; callers must mask them before constructing a
// Field with Sensitive set.
type Field struct {
	Key       string // name/label, safe to display
	Value     string // metadata or already-masked value, safe to display
	Sensitive bool   // true when Value represents a masked secret
}

// Inspection is the non-sensitive result of inspecting a secret artifact.
type Inspection struct {
	Kind   Kind
	Source string  // file path or "<input>"; never the secret content
	Fields []Field // displayable metadata; secret values are masked
	Issues []string
	Valid  bool
}

// State holds all application state for runtime-vault.
type State struct {
	Inspection *Inspection
	Err        error
}

// New returns an initialised State.
func New() *State { return &State{} }

// LoadFile reads the artifact at path, classifies it, and inspects it. The
// returned State never contains raw secret values. A read error or an
// inspection error is captured in State.Err so the UI can surface it without
// leaking content.
func LoadFile(path string) *State {
	s := New()
	data, err := os.ReadFile(path)
	if err != nil {
		s.Err = fmt.Errorf("read %s: %w", path, err)
		return s
	}
	insp := Inspect(path, data)
	s.Inspection = &insp
	return s
}

// Inspect classifies data and dispatches to the matching inspector. source is
// used only as a non-sensitive label (typically the file path).
func Inspect(source string, data []byte) Inspection {
	switch Detect(source, data) {
	case KindCertificate:
		return inspectCertificate(source, data)
	case KindK8sSecret:
		return inspectK8sSecret(source, data)
	case KindJWT:
		return inspectJWT(source, strings.TrimSpace(string(data)))
	case KindEnv:
		return inspectEnv(source, data)
	case KindAPIKey:
		return inspectAPIKey(source, strings.TrimSpace(string(data)))
	default:
		return Inspection{
			Kind:   KindUnknown,
			Source: source,
			Issues: []string{"unrecognised artifact: not an env file, JWT, certificate, API key, or kubernetes secret"},
		}
	}
}

// Detect classifies data using lightweight content and filename heuristics. It
// reads only enough of the content to classify and never returns secret data.
func Detect(source string, data []byte) Kind {
	trimmed := strings.TrimSpace(string(data))
	lowerName := strings.ToLower(source)

	// PEM certificate: unambiguous header.
	if strings.Contains(trimmed, "-----BEGIN CERTIFICATE-----") {
		return KindCertificate
	}

	// JSON / Kubernetes secret: starts with an object brace.
	if strings.HasPrefix(trimmed, "{") {
		if looksLikeK8sSecret(trimmed) {
			return KindK8sSecret
		}
		// JSON that is not a k8s secret is not something Vault inspects.
		return KindUnknown
	}

	// YAML Kubernetes Secret manifest (the common form). Detected by content
	// rather than filename so YAML manifests with any extension are routed to
	// the secret inspector. The check is strict (kind: Secret, or data/
	// stringData alongside apiVersion/metadata) so plain env/config YAML is
	// not misclassified.
	if looksLikeK8sSecret(trimmed) {
		return KindK8sSecret
	}

	// JWT: exactly three non-empty base64url segments on a single line.
	if isJWTShaped(trimmed) {
		return KindJWT
	}

	// Filename hints for env files.
	if strings.HasSuffix(lowerName, ".env") ||
		strings.Contains(lowerName, ".env") ||
		strings.HasPrefix(lowerName, ".env") {
		return KindEnv
	}

	// Multi-line KEY=VALUE content is an env file.
	if looksLikeEnv(trimmed) {
		return KindEnv
	}

	// A single token-like line: treat as an API key candidate.
	if trimmed != "" && !strings.ContainsAny(trimmed, "\n\r") && !strings.Contains(trimmed, " ") {
		return KindAPIKey
	}

	return KindUnknown
}

// Mask renders a value safely for display: it never reveals the content, only
// the length. Empty values are reported as such.
func Mask(value string) string {
	if value == "" {
		return "(empty)"
	}
	return fmt.Sprintf("•••• (%d chars)", len([]rune(value)))
}
