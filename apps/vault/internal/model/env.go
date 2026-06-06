package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/runtime-sh/runtime/packages/schema"
	"github.com/runtime-sh/runtime/packages/validation"
)

// EnvEntry is a single parsed dotenv assignment. The value is retained in
// memory for comparison but must be masked before display.
type EnvEntry struct {
	Key   string
	Value string
	Line  int
}

// envKeyValidator enforces the conventional dotenv key shape: a leading letter
// or underscore followed by letters, digits, or underscores.
var envKeyValidator = validation.All(
	validation.Required(),
	validation.Pattern(envKeyPattern),
)

// ParseEnv parses dotenv content into entries. It supports KEY=VALUE lines,
// blank lines, # comments, optional "export " prefixes, and single/double
// quoted values. Parsing never logs values. Malformed lines are reported via
// the issues slice, keyed by line number, without echoing their content.
func ParseEnv(data []byte) (entries []EnvEntry, issues []string) {
	lines := strings.Split(string(data), "\n")
	for i, raw := range lines {
		lineNo := i + 1
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "export ")
		eq := strings.Index(trimmed, "=")
		if eq < 0 {
			issues = append(issues, fmt.Sprintf("line %d: missing '=' separator", lineNo))
			continue
		}
		key := strings.TrimSpace(trimmed[:eq])
		value := strings.TrimSpace(trimmed[eq+1:])
		value = unquote(value)
		if err := envKeyValidator(key); err != nil {
			issues = append(issues, fmt.Sprintf("line %d: invalid key: %v", lineNo, err))
			continue
		}
		entries = append(entries, EnvEntry{Key: key, Value: value, Line: lineNo})
	}
	return entries, issues
}

// unquote strips a single matching pair of surrounding single or double quotes.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// inspectEnv validates a dotenv file and produces a masked inspection. Values
// are never included verbatim; each field reports the key and a masked value.
// Duplicate keys are flagged as issues.
func inspectEnv(source string, data []byte) Inspection {
	entries, issues := ParseEnv(data)

	seen := make(map[string]int, len(entries))
	for _, e := range entries {
		seen[e.Key]++
	}
	dupes := make([]string, 0)
	for k, n := range seen {
		if n > 1 {
			dupes = append(dupes, fmt.Sprintf("duplicate key %q defined %d times", k, n))
		}
	}
	sort.Strings(dupes)
	issues = append(issues, dupes...)

	fields := make([]Field, 0, len(entries))
	for _, e := range entries {
		fields = append(fields, Field{Key: e.Key, Value: Mask(e.Value), Sensitive: true})
	}

	return Inspection{
		Kind:   KindEnv,
		Source: source,
		Fields: fields,
		Issues: issues,
		Valid:  len(issues) == 0,
	}
}

// ValidateEnvSchema validates parsed env entries against a required-key schema
// built from the supplied key names. It composes the schema and validation
// packages: each required key becomes a schema.Field whose Validator is a
// validation.Validator, so the two packages interoperate without an adapter.
// Returned errors reference keys by name only and never include values.
func ValidateEnvSchema(data []byte, requiredKeys ...string) error {
	entries, _ := ParseEnv(data)
	present := make(map[string]interface{}, len(entries))
	for _, e := range entries {
		// Store presence only; the value itself is not needed for schema checks
		// and is deliberately not surfaced through the schema layer.
		present[e.Key] = e.Value
	}

	s := schema.New()
	for _, k := range requiredKeys {
		s.Add(schema.Field{
			Name:      k,
			Type:      schema.TypeString,
			Required:  true,
			Validator: validation.Required(),
		})
	}
	return s.Validate(present)
}

// looksLikeEnv reports whether the content resembles a dotenv file: at least
// one non-comment line containing a KEY=VALUE assignment with a valid key.
func looksLikeEnv(trimmed string) bool {
	for _, raw := range strings.Split(trimmed, "\n") {
		line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		if envKeyValidator(key) == nil {
			return true
		}
	}
	return false
}
