package model

import (
	"fmt"
	"regexp"
)

// apiKeyFormat describes a recognisable API-key shape by a name and a matching
// regular expression. The patterns match common prefixes/shapes only; they
// never capture or expose the key material itself.
type apiKeyFormat struct {
	name  string
	regex *regexp.Regexp
}

// knownAPIKeyFormats enumerates heuristics for common provider key formats.
// These are intentionally conservative prefix/shape checks, not secrets.
var knownAPIKeyFormats = []apiKeyFormat{
	{"AWS Access Key ID", regexp.MustCompile(`^(AKIA|ASIA)[0-9A-Z]{16}$`)},
	{"GitHub Token", regexp.MustCompile(`^gh[pousr]_[0-9A-Za-z]{36,}$`)},
	{"GitLab Personal Access Token", regexp.MustCompile(`^glpat-[0-9A-Za-z_-]{20,}$`)},
	{"Slack Token", regexp.MustCompile(`^xox[baprs]-[0-9A-Za-z-]{10,}$`)},
	{"Stripe Secret Key", regexp.MustCompile(`^sk_(live|test)_[0-9A-Za-z]{16,}$`)},
	{"Google API Key", regexp.MustCompile(`^AIza[0-9A-Za-z_-]{35}$`)},
	{"OpenAI API Key", regexp.MustCompile(`^sk-[0-9A-Za-z_-]{20,}$`)},
	{"UUID Token", regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)},
	{"Hex Token", regexp.MustCompile(`^[0-9a-fA-F]{32,}$`)},
}

// DetectAPIKeyFormat returns the name of the first matching known key format,
// or ("Generic", false) when no specific provider format matches. The key
// itself is never returned or logged.
func DetectAPIKeyFormat(key string) (string, bool) {
	for _, f := range knownAPIKeyFormats {
		if f.regex.MatchString(key) {
			return f.name, true
		}
	}
	return "Generic", false
}

// inspectAPIKey classifies a single-line token by shape and reports only
// non-sensitive metadata: detected format, length, and a masked placeholder.
// The key material is never echoed.
func inspectAPIKey(source, key string) Inspection {
	insp := Inspection{Kind: KindAPIKey, Source: source}

	format, recognised := DetectAPIKeyFormat(key)
	insp.Fields = []Field{
		{Key: "format", Value: format},
		{Key: "length", Value: fmt.Sprintf("%d chars", len([]rune(key)))},
		{Key: "value", Value: Mask(key), Sensitive: true},
	}

	if key == "" {
		insp.Issues = append(insp.Issues, "empty API key")
	} else if len(key) < 16 {
		insp.Issues = append(insp.Issues, "API key is unusually short (<16 chars); may be truncated or invalid")
	}
	if !recognised {
		insp.Issues = append(insp.Issues, "no known provider format matched; treated as a generic token")
	}

	// Recognising a known format with no length issue is the only fully-valid case.
	insp.Valid = recognised && key != ""
	return insp
}
