package model

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// K8sSecret is the parsed, non-sensitive view of a Kubernetes Secret manifest.
// The decoded data values are retained in memory for comparison only and must
// be masked before display.
//
// Format support: Kubernetes Secret manifests are most commonly written in
// YAML, but JSON is an equally valid manifest form (kubectl get secret -o
// json). This inspector parses BOTH forms. YAML is a strict superset of JSON,
// so the YAML decoder (gopkg.in/yaml.v3) handles both; the implementation
// reads JSON directly when the content is a JSON object and falls back to the
// YAML decoder otherwise. No external conversion (e.g. `kubectl -o json`) is
// required.
type K8sSecret struct {
	Name string
	Type string
	// Data maps key -> decoded value. Values are sensitive.
	Data map[string]string
	// StringData maps key -> plaintext value (the stringData field). Sensitive.
	StringData map[string]string
}

// k8sManifest mirrors the subset of the Secret schema this inspector reads.
// Both json and yaml struct tags are declared so the same struct decodes JSON
// and YAML manifests identically.
type k8sManifest struct {
	Kind     string `json:"kind" yaml:"kind"`
	Metadata struct {
		Name string `json:"name" yaml:"name"`
	} `json:"metadata" yaml:"metadata"`
	Type       string            `json:"type" yaml:"type"`
	Data       map[string]string `json:"data" yaml:"data"`
	StringData map[string]string `json:"stringData" yaml:"stringData"`
}

// ParseK8sSecret parses a Kubernetes Secret manifest in either JSON or YAML
// form and base64url/std-base64-decodes the values under "data". A decode
// failure for a value is reported via issues without echoing the value. It
// returns an error only when the manifest itself is malformed or is not a
// Secret.
func ParseK8sSecret(data []byte) (*K8sSecret, []string, error) {
	var m k8sManifest
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "{") {
		// JSON object form: decode with the standard library.
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, nil, fmt.Errorf("invalid JSON manifest: %w", err)
		}
	} else {
		// YAML form (also accepts JSON, which is valid YAML).
		if err := yaml.Unmarshal(data, &m); err != nil {
			return nil, nil, fmt.Errorf("invalid YAML manifest: %w", err)
		}
	}
	if m.Kind != "" && !strings.EqualFold(m.Kind, "Secret") {
		return nil, nil, fmt.Errorf("manifest kind is %q, expected Secret", m.Kind)
	}

	var issues []string
	decoded := make(map[string]string, len(m.Data))
	for k, v := range m.Data {
		b, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			issues = append(issues, fmt.Sprintf("data[%q]: invalid base64 encoding", k))
			continue
		}
		decoded[k] = string(b)
	}

	sec := &K8sSecret{
		Name:       m.Metadata.Name,
		Type:       m.Type,
		Data:       decoded,
		StringData: m.StringData,
	}
	return sec, issues, nil
}

// inspectK8sSecret parses a JSON or YAML Secret manifest and reports key names
// and masked values. Decoded secret material is never echoed.
func inspectK8sSecret(source string, data []byte) Inspection {
	insp := Inspection{Kind: KindK8sSecret, Source: source}

	sec, issues, err := ParseK8sSecret(data)
	if err != nil {
		insp.Issues = []string{fmt.Sprintf("parse failed: %v", err)}
		return insp
	}
	insp.Issues = issues

	if sec.Name != "" {
		insp.Fields = append(insp.Fields, Field{Key: "name", Value: sec.Name})
	}
	if sec.Type != "" {
		insp.Fields = append(insp.Fields, Field{Key: "type", Value: sec.Type})
	}

	keys := make([]string, 0, len(sec.Data))
	for k := range sec.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		insp.Fields = append(insp.Fields, Field{Key: "data." + k, Value: Mask(sec.Data[k]), Sensitive: true})
	}

	skeys := make([]string, 0, len(sec.StringData))
	for k := range sec.StringData {
		skeys = append(skeys, k)
	}
	sort.Strings(skeys)
	for _, k := range skeys {
		insp.Fields = append(insp.Fields, Field{Key: "stringData." + k, Value: Mask(sec.StringData[k]), Sensitive: true})
	}

	if len(sec.Data) == 0 && len(sec.StringData) == 0 {
		insp.Issues = append(insp.Issues, "secret contains no data or stringData entries")
	}

	insp.Valid = len(insp.Issues) == 0
	return insp
}

// looksLikeK8sSecret reports whether content looks like a Kubernetes Secret
// manifest in either JSON or YAML form: it declares "kind: Secret" or carries
// a "data"/"stringData" object alongside an "apiVersion"/"metadata". The check
// is heuristic and reads no secret values.
func looksLikeK8sSecret(trimmed string) bool {
	var probe struct {
		Kind       string                 `json:"kind" yaml:"kind"`
		APIVersion string                 `json:"apiVersion" yaml:"apiVersion"`
		Data       map[string]interface{} `json:"data" yaml:"data"`
		StringData map[string]interface{} `json:"stringData" yaml:"stringData"`
		Metadata   map[string]interface{} `json:"metadata" yaml:"metadata"`
	}
	// YAML is a superset of JSON, so the YAML decoder handles both forms.
	if err := yaml.Unmarshal([]byte(trimmed), &probe); err != nil {
		return false
	}
	if strings.EqualFold(probe.Kind, "Secret") {
		return true
	}
	hasData := len(probe.Data) > 0 || len(probe.StringData) > 0
	return hasData && (probe.APIVersion != "" || len(probe.Metadata) > 0)
}
