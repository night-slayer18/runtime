package model

import (
	"regexp"
	"sort"
)

// envKeyPattern is the conventional dotenv key shape. Declared here so both the
// env parser and the detector share a single definition.
var envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// DiffStatus classifies how a single key differs between two secret sets.
type DiffStatus int

const (
	// DiffOnlyLeft means the key exists only in the left input.
	DiffOnlyLeft DiffStatus = iota
	// DiffOnlyRight means the key exists only in the right input.
	DiffOnlyRight
	// DiffChanged means the key exists in both but the values differ.
	DiffChanged
	// DiffSame means the key exists in both with identical values.
	DiffSame
)

// String returns a human-readable label for the diff status.
func (d DiffStatus) String() string {
	switch d {
	case DiffOnlyLeft:
		return "only-left"
	case DiffOnlyRight:
		return "only-right"
	case DiffChanged:
		return "changed"
	case DiffSame:
		return "same"
	default:
		return "unknown"
	}
}

// DiffEntry is the comparison outcome for a single key. It deliberately holds
// NO values — only the key name and the status — so a comparison result can be
// displayed or logged without leaking secret material.
type DiffEntry struct {
	Key    string
	Status DiffStatus
}

// CompareSecrets compares two key/value secret maps (for example, two parsed
// env files or two Kubernetes secrets' data) and returns a per-key diff sorted
// by key name. Values are compared in memory but never returned, so the result
// is safe to display. Keys present in both with equal values are reported as
// DiffSame; differing values are DiffChanged; keys unique to one side are
// reported as DiffOnlyLeft/DiffOnlyRight.
func CompareSecrets(left, right map[string]string) []DiffEntry {
	keys := make(map[string]struct{}, len(left)+len(right))
	for k := range left {
		keys[k] = struct{}{}
	}
	for k := range right {
		keys[k] = struct{}{}
	}

	ordered := make([]string, 0, len(keys))
	for k := range keys {
		ordered = append(ordered, k)
	}
	sort.Strings(ordered)

	diffs := make([]DiffEntry, 0, len(ordered))
	for _, k := range ordered {
		lv, lok := left[k]
		rv, rok := right[k]
		switch {
		case lok && !rok:
			diffs = append(diffs, DiffEntry{Key: k, Status: DiffOnlyLeft})
		case !lok && rok:
			diffs = append(diffs, DiffEntry{Key: k, Status: DiffOnlyRight})
		case lv == rv:
			diffs = append(diffs, DiffEntry{Key: k, Status: DiffSame})
		default:
			diffs = append(diffs, DiffEntry{Key: k, Status: DiffChanged})
		}
	}
	return diffs
}

// EnvMap parses env content and returns a key->value map suitable for
// CompareSecrets. Parsing issues are ignored here; use ParseEnv directly when
// issues matter.
func EnvMap(data []byte) map[string]string {
	entries, _ := ParseEnv(data)
	m := make(map[string]string, len(entries))
	for _, e := range entries {
		m[e.Key] = e.Value
	}
	return m
}
