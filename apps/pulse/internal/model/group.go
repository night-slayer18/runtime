package model

import (
	"regexp"
	"sort"
	"strings"
)

// Normalization patterns used to collapse the variable parts of a log line into
// stable placeholders. They are applied in order; the order matters because the
// more specific patterns (UUIDs, hex) must run before the generic number
// pattern would otherwise partially rewrite them.
var (
	reUUID      = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	reTimestamp = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?\b`)
	reIP        = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	reHex       = regexp.MustCompile(`(?i)\b0x[0-9a-f]+\b|\b[0-9a-f]{8,}\b`)
	reNumber    = regexp.MustCompile(`\b\d+(\.\d+)?\b`)
	reSpaces    = regexp.MustCompile(`\s+`)
)

// Normalize collapses the variable parts of a log line — UUIDs, timestamps,
// IP addresses, hex strings, and numeric literals — into fixed placeholder tokens
// and squeezes runs of whitespace. Two log lines that differ only in those variable
// parts normalize to the same template, which is the basis for grouping similar errors.
func Normalize(line string) string {
	s := reUUID.ReplaceAllString(line, "<UUID>")
	s = reTimestamp.ReplaceAllString(s, "<TIMESTAMP>")
	s = reIP.ReplaceAllString(s, "<IP>")
	s = reHex.ReplaceAllString(s, "<HEX>")
	s = reNumber.ReplaceAllString(s, "<NUM>")
	s = reSpaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// LogGroup is a set of log entries that share a normalized template.
type LogGroup struct {
	// Template is the normalized line shared by every member of the group.
	Template string
	// Count is the number of entries in the group.
	Count int
	// Entries holds the group's members in the order they were first seen.
	Entries []LogEntry
}

// GroupSimilar partitions entries into groups keyed by their normalized
// template, so log lines that differ only in numbers, UUIDs, or hex values are
// collapsed together. Groups are returned sorted by descending Count (and by
// template for stable ordering among equal counts), surfacing the most frequent
// error shapes first.
func GroupSimilar(entries []LogEntry) []LogGroup {
	index := make(map[string]int)
	var groups []LogGroup
	for _, e := range entries {
		tmpl := Normalize(e.Text)
		if i, ok := index[tmpl]; ok {
			groups[i].Count++
			groups[i].Entries = append(groups[i].Entries, e)
			continue
		}
		index[tmpl] = len(groups)
		groups = append(groups, LogGroup{
			Template: tmpl,
			Count:    1,
			Entries:  []LogEntry{e},
		})
	}
	sort.SliceStable(groups, func(a, b int) bool {
		if groups[a].Count != groups[b].Count {
			return groups[a].Count > groups[b].Count
		}
		return groups[a].Template < groups[b].Template
	})
	return groups
}
